package auth

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/alexedwards/argon2id"
	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/dal/db"
	"sso-server/model"
)

var dummyPasswordHash string
var dummyPasswordHashOnce sync.Once

const (
	passwordLoginFailureLimit = 5
	passwordLoginLockDuration = 15 * time.Minute
	passwordMinLength         = 10
	passwordMaxLength         = 256
)

func retryAfterSecondsFromDuration(remaining time.Duration) int {
	if remaining <= 0 {
		return 1
	}
	return int((remaining + time.Second - time.Nanosecond) / time.Second)
}

// HashPassword creates an Argon2id password hash.
func HashPassword(password string) (string, error) {
	return argon2id.CreateHash(password, argon2id.DefaultParams)
}

// VerifyPassword verifies only Argon2id hashes.
func VerifyPassword(password string, hash string) (bool, error) {
	return argon2id.ComparePasswordAndHash(password, hash)
}

func (s *AuthService) validatePassword(password string) error {
	if len([]rune(password)) < passwordMinLength || len([]rune(password)) > passwordMaxLength {
		return common.ErrPasswordLengthInvalid
	}

	hasLetter := false
	hasDigit := false
	for _, character := range password {
		if (character >= 'A' && character <= 'Z') || (character >= 'a' && character <= 'z') {
			hasLetter = true
		}
		if character >= '0' && character <= '9' {
			hasDigit = true
		}
	}
	if !hasLetter {
		return common.ErrPasswordLetterRequired
	}
	if !hasDigit {
		return common.ErrPasswordDigitRequired
	}
	return nil
}

// ChangePassword verifies the current password, updates it, and revokes all other active sessions.
func (s *AuthService) ChangePassword(ctx context.Context, userID string, sessionID string, currentPassword string, newPassword string) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(sessionID) == "" {
		return common.ErrSessionRevoked
	}

	user, err := db.NewUserRepository(s.db).FindByID(ctx, userID)
	if err != nil {
		return common.ErrUserNotFound
	}
	if user.PasswordHash == nil || strings.TrimSpace(*user.PasswordHash) == "" {
		return common.ErrPasswordNotSet
	}

	matched, err := VerifyPassword(currentPassword, *user.PasswordHash)
	if err != nil || !matched {
		return common.ErrCurrentPasswordInvalid
	}
	if err := s.validatePassword(newPassword); err != nil {
		return err
	}

	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		session, err := db.NewUserSessionRepository(tx).FindActive(ctx, sessionID, time.Now())
		if err != nil || session.UserID != userID {
			return common.ErrSessionRevoked
		}
		user.PasswordHash = &hash
		if err := db.NewUserRepository(tx).Update(ctx, user); err != nil {
			return err
		}
		return db.NewUserSessionRepository(tx).RevokeOthers(ctx, userID, sessionID, "password_changed", time.Now())
	})
}

// PasswordLoginContext carries request context used by the login guard and
// session metadata.
type PasswordLoginContext struct {
	DeviceID     string
	IP           string
	CaptchaValid bool
}

// LoginWithPassword authenticates with conservative defaults for service-level callers.
func (s *AuthService) LoginWithPassword(ctx context.Context, email string, password string) (*model.User, error) {
	return s.LoginWithPasswordContext(ctx, email, password, PasswordLoginContext{})
}

// LoginWithPasswordContext authenticates a password while applying the
// account, device and IP risk controls.
func (s *AuthService) LoginWithPasswordContext(ctx context.Context, email string, password string, loginContext PasswordLoginContext) (*model.User, error) {
	email = normalizeEmail(email)
	if err := s.guard.allowPasswordLogin(ctx, email, loginContext.DeviceID, loginContext.IP, loginContext.CaptchaValid); err != nil {
		return nil, err
	}

	userRepo := db.NewUserRepository(s.db)
	user, err := userRepo.FindByEmail(ctx, email)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	hash := ""
	if user != nil && user.PasswordHash != nil {
		hash = strings.TrimSpace(*user.PasswordHash)
	}
	if hash == "" {
		hash = s.dummyPasswordHash()
	}
	matched, verifyErr := VerifyPassword(password, hash)
	if verifyErr != nil || !matched || user == nil {
		if err := s.guard.recordPasswordFailure(ctx, email, loginContext.DeviceID, loginContext.IP); err != nil {
			return nil, err
		}
		return nil, common.ErrInvalidCredentials
	}
	if err := s.guard.clearPasswordAccountFailure(ctx, email); err != nil {
		return nil, err
	}
	if !user.IsActive {
		return nil, common.ErrUserInactive
	}
	return user, nil
}

func (s *AuthService) dummyPasswordHash() string {
	dummyPasswordHashOnce.Do(func() {
		dummyPasswordHash, _ = HashPassword("dummy-password-for-timing-only")
	})
	return dummyPasswordHash
}
