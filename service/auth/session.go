package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"sso-server/dal/kv"
)

const (
	SessionCookieName = "sso_session"
	SessionTTL        = 12 * time.Hour
)

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
