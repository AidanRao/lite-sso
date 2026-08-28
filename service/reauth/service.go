// Package reauth issues and validates short-lived Passkey authorization grants.
package reauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"sso-server/common"
	"sso-server/conf"
	"sso-server/dal/kv"
)

// Grant is the server-side authorization state bound to one login session.
type Grant struct {
	UserID       string    `json:"user_id"`
	SessionID    string    `json:"session_id"`
	CredentialID string    `json:"credential_id"`
	IssuedAt     time.Time `json:"issued_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// Result contains the opaque token returned only once to the browser.
type Result struct {
	Token     string `json:"token"`
	TokenType string `json:"token_type"`
	ExpiresIn int    `json:"expires_in"`
}

// ReauthAuthorizer validates a generic recent Passkey verification for an endpoint.
type ReauthAuthorizer interface {
	Authorize(ctx context.Context, token string, userID string, sessionID string) (*Grant, error)
}

// Service manages reusable, short-lived Passkey grants.
type Service struct {
	store kv.Store
	ttl   time.Duration
}

// NewService creates a generic Passkey re-authentication service.
func NewService(cfg *conf.Config, store kv.Store) *Service {
	ttl := 5 * time.Minute
	if cfg != nil && cfg.Passkey.ReauthTokenTTL > 0 {
		ttl = cfg.Passkey.ReauthTokenTTL
	}
	return &Service{store: store, ttl: ttl}
}

// Issue creates an opaque token whose digest is the only value used as a key.
func (s *Service) Issue(ctx context.Context, userID string, sessionID string, credentialID string) (*Result, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(random)
	now := time.Now()
	grant := Grant{UserID: userID, SessionID: sessionID, CredentialID: credentialID, IssuedAt: now, ExpiresAt: now.Add(s.ttl)}
	data, err := json.Marshal(grant)
	if err != nil {
		return nil, err
	}
	if err := s.store.Set(ctx, kv.KeyPasskeyReauthGrant(tokenDigest(token)), string(data), s.ttl); err != nil {
		return nil, err
	}
	return &Result{Token: token, TokenType: "Reauth", ExpiresIn: int(s.ttl / time.Second)}, nil
}

// Authorize validates that a token belongs to the current user and login session.
func (s *Service) Authorize(ctx context.Context, token string, userID string, sessionID string) (*Grant, error) {
	if strings.TrimSpace(token) == "" {
		return nil, common.ErrReauthRequired
	}
	data, err := s.store.Get(ctx, kv.KeyPasskeyReauthGrant(tokenDigest(token)))
	if err != nil {
		if errors.Is(err, kv.ErrNotFound) {
			return nil, common.ErrReauthTokenInvalid
		}
		return nil, err
	}
	var grant Grant
	if err := json.Unmarshal([]byte(data), &grant); err != nil {
		return nil, common.ErrReauthTokenInvalid
	}
	if grant.UserID != userID || grant.SessionID != sessionID || !time.Now().Before(grant.ExpiresAt) {
		return nil, common.ErrReauthTokenInvalid
	}
	return &grant, nil
}

func tokenDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
