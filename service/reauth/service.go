// Package reauth issues and validates short-lived authorization grants.
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

	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/conf"
	"sso-server/dal/db"
	"sso-server/dal/kv"
	serviceauth "sso-server/service/auth"
	"sso-server/util/mask"
)

const (
	MethodPasskey = "passkey"
	MethodEmail   = "email"
)

// Deps contains the dependencies required by the re-authentication service.
type Deps struct {
	Config *conf.Config
	DB     *gorm.DB
	Store  kv.Store
	Auth   *serviceauth.AuthService
}

// Grant is the server-side authorization state bound to one login session.
type Grant struct {
	UserID    string    `json:"user_id"`
	SessionID string    `json:"session_id"`
	Method    string    `json:"method"`
	ProofID   string    `json:"proof_id,omitempty"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Result contains the opaque token returned only once to the browser.
type Result struct {
	Token     string `json:"token"`
	TokenType string `json:"token_type"`
	ExpiresIn int    `json:"expires_in"`
}

// Descriptor tells the browser which verification methods are available.
type Descriptor struct {
	Methods   []string `json:"methods"`
	MaxAge    int      `json:"max_age"`
	EmailHint string   `json:"email_hint,omitempty"`
	Username  string   `json:"username,omitempty"`
	AvatarURL string   `json:"avatar_url,omitempty"`
}

type emailChallenge struct {
	UserID    string    `json:"user_id"`
	SessionID string    `json:"session_id"`
	DeviceID  string    `json:"device_id"`
	Email     string    `json:"email"`
	ExpiresAt time.Time `json:"expires_at"`
}

// ReauthAuthorizer describes middleware-facing re-authentication operations.
type ReauthAuthorizer interface {
	Authorize(ctx context.Context, token string, userID string, sessionID string) (*Grant, error)
	AuthorizeSession(ctx context.Context, userID string, sessionID string) (*Grant, error)
	Describe(ctx context.Context, userID string) (*Descriptor, error)
}

// Service manages reusable, short-lived authorization grants.
type Service struct {
	cfg      *conf.Config
	database *gorm.DB
	store    kv.Store
	auth     *serviceauth.AuthService
	ttl      time.Duration
}

// NewService creates a generic re-authentication service.
func NewService(deps Deps) *Service {
	ttl := 5 * time.Minute
	if deps.Config != nil && deps.Config.Auth.ReauthTokenTTL > 0 {
		ttl = deps.Config.Auth.ReauthTokenTTL
	}
	return &Service{cfg: deps.Config, database: deps.DB, store: deps.Store, auth: deps.Auth, ttl: ttl}
}

// Issue creates an opaque token whose digest is the only value used as a key.
func (s *Service) Issue(ctx context.Context, userID string, sessionID string, method string, proofID string) (*Result, error) {
	if method != MethodPasskey && method != MethodEmail {
		return nil, common.ErrReauthMethodUnavailable
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(random)
	now := time.Now()
	grant := Grant{UserID: userID, SessionID: sessionID, Method: method, ProofID: proofID, IssuedAt: now, ExpiresAt: now.Add(s.ttl)}
	data, err := json.Marshal(grant)
	if err != nil {
		return nil, err
	}
	if err := s.store.Set(ctx, kv.KeyReauthGrant(tokenDigest(token)), string(data), s.ttl); err != nil {
		return nil, err
	}
	if err := s.store.Set(ctx, kv.KeyReauthSession(sessionID), string(data), s.ttl); err != nil {
		return nil, err
	}
	return &Result{Token: token, TokenType: "Reauth", ExpiresIn: int(s.ttl / time.Second)}, nil
}

// Authorize validates that a token belongs to the current user and login session.
func (s *Service) Authorize(ctx context.Context, token string, userID string, sessionID string) (*Grant, error) {
	if strings.TrimSpace(token) == "" {
		return nil, common.ErrReauthRequired
	}
	return s.authorizeGrant(ctx, kv.KeyReauthGrant(tokenDigest(token)), userID, sessionID, common.ErrReauthTokenInvalid)
}

// AuthorizeSession validates the recent verification state of the current login session.
func (s *Service) AuthorizeSession(ctx context.Context, userID string, sessionID string) (*Grant, error) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(sessionID) == "" {
		return nil, common.ErrReauthRequired
	}
	return s.authorizeGrant(ctx, kv.KeyReauthSession(sessionID), userID, sessionID, common.ErrReauthRequired)
}

func (s *Service) authorizeGrant(ctx context.Context, key string, userID string, sessionID string, invalidError error) (*Grant, error) {
	data, err := s.store.Get(ctx, key)
	if err != nil {
		if errors.Is(err, kv.ErrNotFound) {
			return nil, invalidError
		}
		return nil, err
	}
	var grant Grant
	if err := json.Unmarshal([]byte(data), &grant); err != nil {
		return nil, invalidError
	}
	if grant.UserID != userID || grant.SessionID != sessionID || !time.Now().Before(grant.ExpiresAt) {
		return nil, invalidError
	}
	return &grant, nil
}

// Describe returns available methods in preferred display order.
func (s *Service) Describe(ctx context.Context, userID string) (*Descriptor, error) {
	descriptor := &Descriptor{Methods: make([]string, 0, 2), MaxAge: int(s.ttl / time.Second)}
	if s.database == nil {
		return descriptor, nil
	}
	user, err := db.NewUserRepository(s.database).FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if s.cfg != nil && strings.TrimSpace(s.cfg.Passkey.RPID) != "" {
		hasPasskey, err := db.NewWebAuthnRepository(s.database).HasCredentials(ctx, s.cfg.Passkey.RPID, userID)
		if err != nil {
			return nil, err
		}
		if hasPasskey {
			descriptor.Methods = append(descriptor.Methods, MethodPasskey)
		}
	}
	if user.Email != nil && strings.TrimSpace(*user.Email) != "" {
		descriptor.Methods = append(descriptor.Methods, MethodEmail)
		descriptor.EmailHint = mask.Email(*user.Email)
	}
	if user.Username != nil {
		descriptor.Username = strings.TrimSpace(*user.Username)
	}
	if user.AvatarURL != nil {
		descriptor.AvatarURL = strings.TrimSpace(*user.AvatarURL)
	}
	return descriptor, nil
}

// BeginEmail sends a purpose-bound email challenge to the current account.
func (s *Service) BeginEmail(ctx context.Context, userID string, sessionID string, deviceID string, captchaID string, captcha string, requestContext serviceauth.OTPRequestContext) (*serviceauth.ChallengeResult, error) {
	if s.database == nil || s.auth == nil {
		return nil, common.ErrReauthMethodUnavailable
	}
	user, err := db.NewUserRepository(s.database).FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.Email == nil || strings.TrimSpace(*user.Email) == "" {
		return nil, common.ErrReauthMethodUnavailable
	}
	result, err := s.auth.SendEmailOTP(ctx, *user.Email, captchaID, captcha, requestContext, serviceauth.ChallengePurposeReauth)
	if err != nil {
		return nil, err
	}
	state := emailChallenge{
		UserID:    userID,
		SessionID: sessionID,
		DeviceID:  deviceID,
		Email:     strings.ToLower(strings.TrimSpace(*user.Email)),
		ExpiresAt: time.Now().Add(time.Duration(result.ExpiresIn) * time.Second),
	}
	data, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	if err := s.store.Set(ctx, kv.KeyReauthEmailChallenge(result.ChallengeID), string(data), time.Duration(result.ExpiresIn)*time.Second); err != nil {
		return nil, err
	}
	return result, nil
}

// FinishEmail verifies an email challenge and issues a session-bound grant.
func (s *Service) FinishEmail(ctx context.Context, userID string, sessionID string, deviceID string, ip string, challengeID string, code string) (*Result, error) {
	data, err := s.store.Get(ctx, kv.KeyReauthEmailChallenge(challengeID))
	if err != nil {
		return nil, common.ErrChallengeInvalid
	}
	var state emailChallenge
	if err := json.Unmarshal([]byte(data), &state); err != nil {
		return nil, common.ErrChallengeInvalid
	}
	if state.UserID != userID || state.SessionID != sessionID || state.DeviceID != deviceID || !time.Now().Before(state.ExpiresAt) {
		return nil, common.ErrChallengeInvalid
	}
	email, err := s.auth.VerifyEmailOTPForPurpose(ctx, challengeID, code, serviceauth.OTPVerifyContext{DeviceID: deviceID, IP: ip}, serviceauth.ChallengePurposeReauth)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(strings.TrimSpace(email), state.Email) {
		return nil, common.ErrChallengeInvalid
	}
	if err := s.store.Del(ctx, kv.KeyReauthEmailChallenge(challengeID)); err != nil {
		return nil, err
	}
	return s.Issue(ctx, userID, sessionID, MethodEmail, "")
}

func tokenDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
