package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"sso-server/common"
	"sso-server/dal/db"
	"sso-server/dal/kv"
	"sso-server/dto"
)

const (
	SessionCookieName = "sso_session"
	SessionTTL        = 12 * time.Hour
)

type LoginResult struct {
	User        *dto.UserResponse `json:"user"`
	RedirectURL string            `json:"redirect_url"`
}

func generateSessionID() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func (s *AuthService) CreateSession(ctx context.Context, userID string) (string, error) {
	if s.kv == nil {
		return "", errors.New("kv store is nil")
	}

	sessionID, err := generateSessionID()
	if err != nil {
		return "", err
	}

	if err := s.kv.Set(ctx, kv.KeySession(sessionID), userID, SessionTTL); err != nil {
		return "", err
	}

	return sessionID, nil
}

func (s *AuthService) CompleteLogin(ctx context.Context, userID string, redirect string) (*LoginResult, string, error) {
	userRepo := db.NewUserRepository(s.db)
	user, err := userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, "", common.ErrInvalidCredentials
	}
	if !user.IsActive {
		return nil, "", common.ErrUserInactive
	}

	redirectURL, err := NormalizeLoginRedirect(redirect)
	if err != nil {
		return nil, "", err
	}

	sessionID, err := s.CreateSession(ctx, user.ID)
	if err != nil {
		return nil, "", err
	}

	return &LoginResult{
		User:        dto.ToUserResponse(user),
		RedirectURL: redirectURL,
	}, sessionID, nil
}

func (s *AuthService) ResolveSessionUserID(ctx context.Context, sessionID string) (string, error) {
	if s.kv == nil {
		return "", errors.New("kv store is nil")
	}
	return s.kv.Get(ctx, kv.KeySession(sessionID))
}

func (s *AuthService) InvalidateSession(ctx context.Context, sessionID string) error {
	if s.kv == nil {
		return errors.New("kv store is nil")
	}
	if sessionID == "" {
		return errors.New("session id is empty")
	}
	return s.kv.Del(ctx, kv.KeySession(sessionID))
}
