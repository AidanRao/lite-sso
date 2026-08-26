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

func (s *AuthService) passwordMinLength() int {
	if s.cfg != nil && s.cfg.Auth.PasswordMinLength > 0 {
		return s.cfg.Auth.PasswordMinLength
	}
	return 12
}

func (s *AuthService) validatePassword(password string) error {
	if len([]rune(password)) < s.passwordMinLength() || len([]rune(password)) > 256 {
		return errors.New("invalid password length")
	}
	return nil
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
