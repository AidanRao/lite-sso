package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"sso-server/conf"
	"sso-server/dal/kv"
)

var ErrInvalidRefresh = errors.New("invalid OAuth refresh token")

// RefreshGrant binds a refresh token to the original OAuth authorization.
type RefreshGrant struct {
	UserID       string    `json:"user_id"`
	ClientID     string    `json:"client_id"`
	Audiences    []string  `json:"audiences"`
	Scopes       []string  `json:"scopes"`
	AuthorizedAt int64     `json:"authorized_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// RefreshStore persists and rotates OAuth refresh grants.
type RefreshStore interface {
	Issue(context.Context, string, RefreshGrant) error
	Load(context.Context, string) (RefreshGrant, error)
	Rotate(context.Context, string, string, RefreshGrant) error
	RevokeUser(context.Context, string) error
}

// NewRefreshToken creates a random opaque refresh token.
func NewRefreshToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func refreshDigest(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

// RedisRefreshStore persists refresh grants with atomic Redis operations.
type RedisRefreshStore struct {
	client *redis.Client
	prefix string
}

// NewRedisRefreshStore creates the production Redis-backed refresh store.
func NewRedisRefreshStore(client *redis.Client) *RedisRefreshStore {
	return &RedisRefreshStore{client: client, prefix: kv.NamespacePrefix(conf.GetEnvironmentName())}
}

func (s *RedisRefreshStore) key(token string) string {
	return s.prefix + kv.KeyOAuthRefreshToken(refreshDigest(token))
}
func (s *RedisRefreshStore) userKey(userID string) string {
	return s.prefix + kv.KeyOAuthRefreshUser(userID)
}
func (s *RedisRefreshStore) revokedKey(userID string) string {
	return s.prefix + kv.KeyOAuthRefreshRevoked(userID)
}

const issueRefreshScript = `
if tonumber(ARGV[3]) <= tonumber(redis.call('GET', KEYS[3]) or '0') then return 0 end
redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[2])
redis.call('SADD', KEYS[2], KEYS[1])
if redis.call('PTTL', KEYS[2]) < tonumber(ARGV[2]) then redis.call('PEXPIRE', KEYS[2], ARGV[2]) end
return 1`

const rotateRefreshScript = `
if redis.call('GET', KEYS[1]) ~= ARGV[1] then return 0 end
redis.call('DEL', KEYS[1])
redis.call('SET', KEYS[2], ARGV[2], 'PX', ARGV[3])
redis.call('SREM', KEYS[3], KEYS[1])
redis.call('SADD', KEYS[3], KEYS[2])
if redis.call('PTTL', KEYS[3]) < tonumber(ARGV[3]) then redis.call('PEXPIRE', KEYS[3], ARGV[3]) end
return 1`

const revokeUserRefreshScript = `
local keys = redis.call('SMEMBERS', KEYS[1])
for _, key in ipairs(keys) do redis.call('DEL', key) end
redis.call('DEL', KEYS[1])
redis.call('SET', KEYS[2], ARGV[1], 'PX', ARGV[2])
return #keys`

func (s *RedisRefreshStore) Issue(ctx context.Context, token string, grant RefreshGrant) error {
	payload, err := json.Marshal(grant)
	if err != nil {
		return err
	}
	remaining := time.Until(grant.ExpiresAt)
	if remaining <= 0 {
		return ErrInvalidRefresh
	}
	result, err := s.client.Eval(ctx, issueRefreshScript, []string{s.key(token), s.userKey(grant.UserID), s.revokedKey(grant.UserID)}, string(payload), remaining.Milliseconds(), grant.AuthorizedAt).Int()
	if err != nil {
		return err
	}
	if result != 1 {
		return ErrInvalidRefresh
	}
	return nil
}

func (s *RedisRefreshStore) Load(ctx context.Context, token string) (RefreshGrant, error) {
	var grant RefreshGrant
	payload, err := s.client.Get(ctx, s.key(token)).Bytes()
	if errors.Is(err, redis.Nil) {
		return grant, ErrInvalidRefresh
	}
	if err != nil {
		return grant, err
	}
	if err := json.Unmarshal(payload, &grant); err != nil {
		return grant, err
	}
	if !time.Now().Before(grant.ExpiresAt) {
		return grant, ErrInvalidRefresh
	}
	return grant, nil
}

func (s *RedisRefreshStore) Rotate(ctx context.Context, oldToken, newToken string, grant RefreshGrant) error {
	oldPayload, err := s.client.Get(ctx, s.key(oldToken)).Bytes()
	if errors.Is(err, redis.Nil) {
		return ErrInvalidRefresh
	}
	if err != nil {
		return err
	}
	newPayload, err := json.Marshal(grant)
	if err != nil {
		return err
	}
	remaining := time.Until(grant.ExpiresAt)
	if remaining <= 0 {
		return ErrInvalidRefresh
	}
	result, err := s.client.Eval(ctx, rotateRefreshScript, []string{s.key(oldToken), s.key(newToken), s.userKey(grant.UserID)}, string(oldPayload), string(newPayload), remaining.Milliseconds()).Int()
	if err != nil {
		return err
	}
	if result != 1 {
		return ErrInvalidRefresh
	}
	return nil
}

func (s *RedisRefreshStore) RevokeUser(ctx context.Context, userID string) error {
	return s.client.Eval(ctx, revokeUserRefreshScript, []string{s.userKey(userID), s.revokedKey(userID)}, time.Now().UnixMicro(), int64((30*24*time.Hour)/time.Millisecond)).Err()
}

// MemoryRefreshStore is an isolated refresh store for tests.
type MemoryRefreshStore struct {
	mu      sync.Mutex
	grants  map[string]RefreshGrant
	revoked map[string]int64
}

// NewMemoryRefreshStore creates an isolated store for tests.
func NewMemoryRefreshStore() *MemoryRefreshStore {
	return &MemoryRefreshStore{grants: make(map[string]RefreshGrant), revoked: make(map[string]int64)}
}

func (s *MemoryRefreshStore) Issue(ctx context.Context, token string, grant RefreshGrant) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !time.Now().Before(grant.ExpiresAt) || grant.AuthorizedAt <= s.revoked[grant.UserID] {
		return ErrInvalidRefresh
	}
	s.grants[refreshDigest(token)] = grant
	return nil
}

func (s *MemoryRefreshStore) Load(ctx context.Context, token string) (RefreshGrant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	grant, ok := s.grants[refreshDigest(token)]
	if !ok || !time.Now().Before(grant.ExpiresAt) {
		return grant, ErrInvalidRefresh
	}
	return grant, nil
}

func (s *MemoryRefreshStore) Rotate(ctx context.Context, oldToken, newToken string, grant RefreshGrant) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.grants[refreshDigest(oldToken)]
	if !ok || !time.Now().Before(old.ExpiresAt) || !time.Now().Before(grant.ExpiresAt) {
		return ErrInvalidRefresh
	}
	delete(s.grants, refreshDigest(oldToken))
	s.grants[refreshDigest(newToken)] = grant
	return nil
}

func (s *MemoryRefreshStore) RevokeUser(ctx context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for token, grant := range s.grants {
		if grant.UserID == userID {
			delete(s.grants, token)
		}
	}
	s.revoked[userID] = time.Now().UnixMicro()
	return nil
}
