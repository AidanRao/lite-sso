package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"sso-server/common"
	"sso-server/dal/db"
	"sso-server/dal/kv"
	"sso-server/model"
)

const (
	passwordLoginFailureLimit  = 5
	passwordLoginFailureWindow = 15 * time.Minute
	passwordLoginLockDuration  = 15 * time.Minute
	passwordLoginLockValue     = "locked"
)

// LoginWithPassword authenticates a user with email and password
func (s *AuthService) LoginWithPassword(ctx context.Context, email, password string) (*model.User, error) {
	if err := s.passwordLoginLockError(ctx, email); err != nil {
		return nil, err
	}

	userRepo := db.NewUserRepository(s.db)

	user, err := userRepo.FindByEmail(ctx, email)
	if err != nil {
		if err := s.recordPasswordLoginFailure(ctx, email); err != nil {
			return nil, err
		}
		return nil, common.ErrInvalidCredentials
	}

	if user.PasswordHash == nil {
		if err := s.recordPasswordLoginFailure(ctx, email); err != nil {
			return nil, err
		}
		return nil, common.ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(password)); err != nil {
		if err := s.recordPasswordLoginFailure(ctx, email); err != nil {
			return nil, err
		}
		return nil, common.ErrInvalidCredentials
	}

	if err := s.clearPasswordLoginFailures(ctx, email); err != nil {
		return nil, err
	}

	if !user.IsActive {
		return nil, common.ErrUserInactive
	}

	return user, nil
}

func passwordLoginKeyEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (s *AuthService) passwordLoginLockError(ctx context.Context, email string) error {
	lockKey := kv.KeyPasswordLoginLock(passwordLoginKeyEmail(email))
	ttl, err := s.kv.TTL(ctx, lockKey)
	if errors.Is(err, kv.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if ttl <= 0 {
		return nil
	}

	return common.AccountLockedError{RetryAfterSeconds: retryAfterSecondsFromDuration(ttl)}
}

func (s *AuthService) recordPasswordLoginFailure(ctx context.Context, email string) error {
	keyEmail := passwordLoginKeyEmail(email)
	failuresKey := kv.KeyPasswordLoginFailures(keyEmail)

	failures, err := s.kv.Increment(ctx, failuresKey, passwordLoginFailureWindow)
	if err != nil {
		return err
	}

	if failures >= passwordLoginFailureLimit {
		if err := s.kv.Set(ctx, kv.KeyPasswordLoginLock(keyEmail), passwordLoginLockValue, passwordLoginLockDuration); err != nil {
			return err
		}
		if err := s.kv.Del(ctx, failuresKey); err != nil {
			return err
		}
		return common.AccountLockedError{RetryAfterSeconds: retryAfterSecondsFromDuration(passwordLoginLockDuration)}
	}

	return nil
}

func (s *AuthService) clearPasswordLoginFailures(ctx context.Context, email string) error {
	return s.kv.Del(ctx, kv.KeyPasswordLoginFailures(passwordLoginKeyEmail(email)))
}

func retryAfterSecondsFromDuration(remaining time.Duration) int {
	if remaining <= 0 {
		return 1
	}
	return int((remaining + time.Second - time.Nanosecond) / time.Second)
}
