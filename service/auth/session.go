package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/dal/db"
	"sso-server/dto"
	"sso-server/model"
	"sso-server/util/requestmeta"
	"sso-server/util/useragent"
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
)

type LoginMetadata struct {
	DeviceID  string
	IP        string
	UserAgent string
}

type TokenPair struct {
	Status       string
	SessionID    string `json:"-"`
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
	return s.cfg.Tokens.Session.AccessTokenTTL
}

func (s *AuthService) refreshTokenTTL() time.Duration {
	return s.cfg.Tokens.Session.RefreshTokenTTL
}

// RefreshTokenTTL returns the configured lifetime of a refresh token.
func (s *AuthService) RefreshTokenTTL() time.Duration {
	return s.refreshTokenTTL()
}

func (s *AuthService) jwtSecret() []byte {
	return []byte(s.cfg.Auth.JWTSecret)
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
	var result *LoginResult
	var pair *TokenPair
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user, err := db.LockUser(ctx, tx, userID)
		if err != nil {
			return common.ErrInvalidCredentials
		}
		now, err := db.DatabaseTime(ctx, tx)
		if err != nil {
			return err
		}
		if user.Status != model.UserStatusActive && user.Status != model.UserStatusPendingDeletion {
			return common.ErrUserInactive
		}
		var deletionID *string
		if user.Status == model.UserStatusPendingDeletion {
			request, err := db.PendingUserDeletion(ctx, tx, user.ID)
			if err != nil {
				return err
			}
			if request.Due(now) {
				return common.ErrAccountDeletionDue
			}
			deletionID = &request.ID
		}
		// Hydrate the verified primary email only after locking the account.
		user, err = db.NewUserRepository(tx).FindByID(ctx, userID)
		if err != nil {
			return err
		}
		redirectURL := "/account/deletion"
		purpose := model.SessionPurposeAccountDeletion
		if user.Status != model.UserStatusPendingDeletion {
			purpose = model.SessionPurposeNormal
			redirectURL, err = NormalizeLoginRedirect(redirect)
			if err != nil {
				return err
			}
		}
		sessionID := "ses_" + uuid.NewString()
		refreshToken, err := generateRefreshToken(sessionID)
		if err != nil {
			return err
		}
		session := &model.UserSession{ID: sessionID, UserID: user.ID, DeviceID: metadata.DeviceID, AuthMethod: string(method), Purpose: purpose, DeletionID: deletionID,
			RefreshTokenHash: refreshHash(refreshToken), IP: requestmeta.NormalizeIP(metadata.IP), Location: s.loginLocation(metadata.IP), UserAgent: useragent.Normalize(metadata.UserAgent), CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(s.refreshTokenTTL())}
		if err := db.NewUserSessionRepository(tx).Create(ctx, session); err != nil {
			return err
		}
		accessToken, expiresIn, err := s.issueAccessToken(user.ID, sessionID)
		if err != nil {
			return err
		}
		result = &LoginResult{User: dto.ToUserResponse(user), RedirectURL: redirectURL, AccessToken: accessToken, TokenType: "Bearer", ExpiresIn: expiresIn}
		pair = &TokenPair{Status: user.Status, SessionID: sessionID, AccessToken: accessToken, RefreshToken: refreshToken, ExpiresIn: expiresIn}
		return nil
	})
	return result, pair, err
}

// CompleteLogin is retained as a small internal adapter for non-HTTP callers.
func (s *AuthService) CompleteLogin(ctx context.Context, userID string, redirect string) (*LoginResult, string, error) {
	result, pair, err := s.CompleteLoginWithContext(ctx, userID, redirect, LoginMetadata{}, AuthMethodPassword)
	if err != nil {
		return nil, "", err
	}
	return result, pair.SessionID, nil
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
	original, err := db.NewUserSessionRepository(s.db).FindByID(ctx, parts[0])
	if err != nil {
		return nil, common.ErrRefreshTokenInvalid
	}
	var pair *TokenPair
	reuse := false
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user, err := db.LockUser(ctx, tx, original.UserID)
		if err != nil {
			return common.ErrRefreshTokenInvalid
		}
		now, err := db.DatabaseTime(ctx, tx)
		if err != nil {
			return err
		}
		repository := db.NewUserSessionRepository(tx)
		session, err := repository.FindByID(ctx, parts[0])
		if err != nil {
			return common.ErrRefreshTokenInvalid
		}
		if err := db.ValidateUserSession(ctx, tx, user, session, now); err != nil {
			return common.ErrRefreshTokenInvalid
		}
		if subtle.ConstantTimeCompare([]byte(session.RefreshTokenHash), []byte(refreshHash(refreshToken))) != 1 {
			reuse = true
			return repository.Revoke(ctx, session.ID, "refresh_token_reuse", now)
		}
		newRefresh, err := generateRefreshToken(session.ID)
		if err != nil {
			return err
		}
		if err := repository.Rotate(ctx, session.ID, session.RefreshTokenHash, refreshHash(newRefresh), now); err != nil {
			return err
		}
		access, expiresIn, err := s.issueAccessToken(session.UserID, session.ID)
		if err != nil {
			return err
		}
		pair = &TokenPair{Status: user.Status, SessionID: session.ID, AccessToken: access, RefreshToken: newRefresh, ExpiresIn: expiresIn}
		return nil
	})
	if reuse {
		return nil, common.ErrRefreshTokenInvalid
	}
	return pair, err
}

// ResolveSession validates the current account and the session's lifecycle purpose.
func (s *AuthService) ResolveSession(ctx context.Context, sessionID string) (*model.User, *model.UserSession, error) {
	now, err := db.DatabaseTime(ctx, s.db)
	if err != nil {
		return nil, nil, err
	}
	session, err := db.NewUserSessionRepository(s.db).FindActive(ctx, sessionID, now)
	if err != nil {
		return nil, nil, common.ErrSessionRevoked
	}
	user, err := db.NewUserRepository(s.db).FindByID(ctx, session.UserID)
	if err != nil {
		return nil, nil, common.ErrSessionRevoked
	}
	if err := db.ValidateUserSession(ctx, s.db, user, session, now); err != nil {
		return nil, nil, err
	}
	return user, session, nil
}

func (s *AuthService) ResolveSessionUserID(ctx context.Context, sessionID string) (string, error) {
	user, _, err := s.ResolveSession(ctx, sessionID)
	if err != nil {
		return "", err
	}
	return user.ID, nil
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
	_, session, err := s.ResolveSession(ctx, claims.SessionID)
	if err != nil || session.UserID != claims.Subject {
		return nil, common.ErrSessionRevoked
	}
	return session, nil
}
