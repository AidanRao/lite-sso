package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/dal/db"
	"sso-server/dto"
	"sso-server/model"
)

const (
	RefreshTokenCookieName = "sso_refresh_token"
	SessionCookieName      = "sso_session"
	SessionTTL             = 12 * time.Hour
)

type AuthMethod string

const (
	AuthMethodPassword AuthMethod = "PASSWORD"
	AuthMethodEmailOTP AuthMethod = "EMAIL_OTP"
	AuthMethodQR       AuthMethod = "QR"
	AuthMethodGitHub   AuthMethod = "GITHUB"
	AuthMethodFeishu   AuthMethod = "FEISHU"
)

type LoginMetadata struct {
	DeviceID  string
	IP        string
	UserAgent string
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
}

func generateSessionID() (string, error) {
	return "ses_" + uuid.NewString(), nil
}

type LoginResult struct {
	User        *dto.UserResponse `json:"user"`
	RedirectURL string            `json:"redirect_url"`
	AccessToken string            `json:"access_token"`
	TokenType   string            `json:"token_type"`
	ExpiresIn   int               `json:"expires_in"`
}

type accessClaims struct {
	SessionID string `json:"sid"`
	jwt.RegisteredClaims
}

func (s *AuthService) accessTokenTTL() time.Duration {
	if s.cfg != nil && s.cfg.Auth.AccessTokenTTL > 0 {
		return s.cfg.Auth.AccessTokenTTL
	}
	return 15 * time.Minute
}

func (s *AuthService) refreshTokenTTL() time.Duration {
	if s.cfg != nil && s.cfg.Auth.RefreshTokenTTL > 0 {
		return s.cfg.Auth.RefreshTokenTTL
	}
	return 30 * 24 * time.Hour
}

// RefreshTokenTTL returns the configured lifetime of a refresh token.
func (s *AuthService) RefreshTokenTTL() time.Duration {
	return s.refreshTokenTTL()
}

func (s *AuthService) jwtSecret() []byte {
	if s.cfg != nil && strings.TrimSpace(s.cfg.Auth.JWTSecret) != "" {
		return []byte(s.cfg.Auth.JWTSecret)
	}
	return []byte("test-only-jwt-secret")
}

func refreshHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", sum[:])
}

func generateRefreshToken(sessionID string) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return sessionID + "." + base64.RawURLEncoding.EncodeToString(buf), nil
}

func (s *AuthService) issueAccessToken(userID string, sessionID string) (string, int, error) {
	ttl := s.accessTokenTTL()
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims{
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	})
	signed, err := token.SignedString(s.jwtSecret())
	return signed, int(ttl / time.Second), err
}

// ParseAccessToken validates signature, algorithm, subject, session and expiry.
func (s *AuthService) ParseAccessToken(tokenString string) (*accessClaims, error) {
	var claims accessClaims
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected jwt signing method")
		}
		return s.jwtSecret(), nil
	})
	if err != nil || !token.Valid || claims.Subject == "" || claims.SessionID == "" {
		return nil, common.ErrInvalidCredentials
	}
	return &claims, nil
}

// CompleteLoginWithContext creates the persistent session and both tokens.
func (s *AuthService) CompleteLoginWithContext(ctx context.Context, userID string, redirect string, metadata LoginMetadata, method AuthMethod) (*LoginResult, *TokenPair, error) {
	if s.db == nil {
		return nil, nil, errors.New("database is nil")
	}
	user, err := db.NewUserRepository(s.db).FindByID(ctx, userID)
	if err != nil {
		return nil, nil, common.ErrInvalidCredentials
	}
	if !user.IsActive {
		return nil, nil, common.ErrUserInactive
	}
	redirectURL, err := NormalizeLoginRedirect(redirect)
	if err != nil {
		return nil, nil, err
	}

	sessionID := "ses_" + uuid.NewString()
	refreshToken, err := generateRefreshToken(sessionID)
	if err != nil {
		return nil, nil, err
	}
	now := time.Now()
	session := &model.UserSession{
		ID:               sessionID,
		UserID:           user.ID,
		DeviceID:         metadata.DeviceID,
		AuthMethod:       string(method),
		RefreshTokenHash: refreshHash(refreshToken),
		IP:               metadata.IP,
		UserAgent:        metadata.UserAgent,
		CreatedAt:        now,
		LastSeenAt:       now,
		ExpiresAt:        now.Add(s.refreshTokenTTL()),
	}
	if err := db.NewUserSessionRepository(s.db).Create(ctx, session); err != nil {
		return nil, nil, err
	}
	accessToken, expiresIn, err := s.issueAccessToken(user.ID, sessionID)
	if err != nil {
		return nil, nil, err
	}
	return &LoginResult{
		User:        dto.ToUserResponse(user),
		RedirectURL: redirectURL,
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   expiresIn,
	}, &TokenPair{AccessToken: accessToken, RefreshToken: refreshToken, ExpiresIn: expiresIn}, nil
}

// CompleteLogin is retained as a small internal adapter for non-HTTP callers.
func (s *AuthService) CompleteLogin(ctx context.Context, userID string, redirect string) (*LoginResult, string, error) {
	result, _, err := s.CompleteLoginWithContext(ctx, userID, redirect, LoginMetadata{}, AuthMethodPassword)
	if err != nil {
		return nil, "", err
	}
	claims, err := s.ParseAccessToken(result.AccessToken)
	if err != nil {
		return nil, "", err
	}
	return result, claims.SessionID, nil
}

// CreateSession creates a new persistent session for legacy internal callers;
// HTTP login flows must use CompleteLoginWithContext to obtain tokens.
func (s *AuthService) CreateSession(ctx context.Context, userID string) (string, error) {
	_, sessionID, err := s.CompleteLogin(ctx, userID, "")
	return sessionID, err
}

func (s *AuthService) RefreshTokens(ctx context.Context, refreshToken string) (*TokenPair, error) {
	parts := strings.SplitN(refreshToken, ".", 2)
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "ses_") {
		return nil, common.ErrRefreshTokenInvalid
	}
	repository := db.NewUserSessionRepository(s.db)
	session, err := repository.FindByID(ctx, parts[0])
	if err != nil || session.RevokedAt != nil || time.Now().After(session.ExpiresAt) {
		return nil, common.ErrRefreshTokenInvalid
	}
	if subtle.ConstantTimeCompare([]byte(session.RefreshTokenHash), []byte(refreshHash(refreshToken))) != 1 {
		_ = repository.Revoke(ctx, session.ID, "refresh_token_reuse", time.Now())
		return nil, common.ErrRefreshTokenInvalid
	}
	newRefresh, err := generateRefreshToken(session.ID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if err := repository.Rotate(ctx, session.ID, session.RefreshTokenHash, refreshHash(newRefresh), now); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_ = repository.Revoke(ctx, session.ID, "refresh_token_reuse", now)
		}
		return nil, common.ErrRefreshTokenInvalid
	}
	access, expiresIn, err := s.issueAccessToken(session.UserID, session.ID)
	if err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: access, RefreshToken: newRefresh, ExpiresIn: expiresIn}, nil
}

func (s *AuthService) ResolveSessionUserID(ctx context.Context, sessionID string) (string, error) {
	session, err := db.NewUserSessionRepository(s.db).FindActive(ctx, sessionID, time.Now())
	if err != nil {
		return "", common.ErrSessionRevoked
	}
	return session.UserID, nil
}

func (s *AuthService) InvalidateSession(ctx context.Context, sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return errors.New("session id is empty")
	}
	return db.NewUserSessionRepository(s.db).Revoke(ctx, sessionID, "logout", time.Now())
}

// InvalidateRefreshToken revokes the session only when the supplied refresh
// token matches the hash persisted for that session.
func (s *AuthService) InvalidateRefreshToken(ctx context.Context, refreshToken string) (bool, error) {
	parts := strings.SplitN(refreshToken, ".", 2)
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "ses_") {
		return false, nil
	}
	repository := db.NewUserSessionRepository(s.db)
	session, err := repository.FindByID(ctx, parts[0])
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	if subtle.ConstantTimeCompare([]byte(session.RefreshTokenHash), []byte(refreshHash(refreshToken))) != 1 {
		return false, nil
	}
	return true, repository.Revoke(ctx, session.ID, "logout", time.Now())
}

func (s *AuthService) InvalidateAllSessions(ctx context.Context, userID string, reason string) error {
	return db.NewUserSessionRepository(s.db).RevokeAll(ctx, userID, reason, time.Now())
}

func (s *AuthService) sessionForAccessToken(ctx context.Context, token string) (*model.UserSession, error) {
	claims, err := s.ParseAccessToken(token)
	if err != nil {
		return nil, err
	}
	session, err := db.NewUserSessionRepository(s.db).FindActive(ctx, claims.SessionID, time.Now())
	if err != nil || session.UserID != claims.Subject {
		return nil, common.ErrSessionRevoked
	}
	return session, nil
}

func requestMetadata(request *http.Request, deviceID string) LoginMetadata {
	return LoginMetadata{DeviceID: deviceID, IP: requestIP(request), UserAgent: request.UserAgent()}
}
