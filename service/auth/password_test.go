package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/dal/kv"
	"sso-server/model"
)

func TestPasswordService_UsesArgon2id(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("expected argon2id hash, got %q", hash)
	}
	matched, err := VerifyPassword("correct horse battery staple", hash)
	if err != nil || !matched {
		t.Fatalf("expected password match, matched=%t err=%v", matched, err)
	}
}

func TestLoginWithPassword_RequiresCaptchaAfterAccountRiskThreshold(t *testing.T) {
	service, email := newPasswordLoginTestService(t, "password123456")
	loginContext := PasswordLoginContext{DeviceID: "dev_test", IP: "127.0.0.1"}

	for i := 0; i < passwordLoginFailureLimit; i++ {
		_, err := service.LoginWithPasswordContext(context.Background(), email, "wrong-password", loginContext)
		if !errors.Is(err, common.ErrInvalidCredentials) {
			t.Fatalf("attempt %d expected invalid credentials, got %v", i+1, err)
		}
	}

	_, err := service.LoginWithPasswordContext(context.Background(), email, "password123456", loginContext)
	if !errors.Is(err, common.ErrCaptchaRequired) {
		t.Fatalf("expected captcha requirement, got %v", err)
	}

	loginContext.CaptchaValid = true
	user, err := service.LoginWithPasswordContext(context.Background(), email, "password123456", loginContext)
	if err != nil || user == nil {
		t.Fatalf("expected successful login after captcha, user=%v err=%v", user, err)
	}
}

func TestLoginWithPassword_AcceptsOnlyVerifiedSecondaryEmail(t *testing.T) {
	service, database, _ := newPasswordLoginTestServiceWithDB(t, "password123456")
	verifiedAt := time.Now()
	secondary := model.UserEmail{ID: "ue-secondary", UserID: "u1", Email: "secondary@example.com", VerifiedAt: &verifiedAt}
	unverified := model.UserEmail{ID: "ue-unverified", UserID: "u1", Email: "unverified@example.com"}
	if err := database.Create(&[]model.UserEmail{secondary, unverified}).Error; err != nil {
		t.Fatalf("create secondary emails: %v", err)
	}

	user, err := service.LoginWithPassword(context.Background(), " Secondary@Example.COM ", "password123456")
	if err != nil || user == nil || user.ID != "u1" {
		t.Fatalf("expected verified secondary email login, user=%#v err=%v", user, err)
	}
	if _, err := service.LoginWithPassword(context.Background(), "unverified@example.com", "password123456"); !errors.Is(err, common.ErrInvalidCredentials) {
		t.Fatalf("expected unverified email rejection, got %v", err)
	}
}

func TestValidatePassword_RequiresLengthLetterAndDigit(t *testing.T) {
	service := NewAuthService(nil, nil, nil, nil, nil)
	testCases := []struct {
		name     string
		password string
		valid    bool
		expected error
	}{
		{name: "valid", password: "password123", valid: true},
		{name: "too short", password: "pass1234", expected: common.ErrPasswordLengthInvalid},
		{name: "missing digit", password: "passwordonly", expected: common.ErrPasswordDigitRequired},
		{name: "missing letter", password: "1234567890", expected: common.ErrPasswordLetterRequired},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := service.validatePassword(testCase.password)
			if testCase.valid && err != nil {
				t.Fatalf("expected valid password, got %v", err)
			}
			if !testCase.valid && !errors.Is(err, testCase.expected) {
				t.Fatalf("expected policy error, got %v", err)
			}
		})
	}
}

func TestChangePassword_UpdatesPasswordAndRevokesOtherSessions(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.AutoMigrate(&model.User{}, &model.UserEmail{}, &model.UserSession{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	oldHash, err := HashPassword("old-password-123")
	if err != nil {
		t.Fatalf("hash old password: %v", err)
	}
	email := "u1@example.com"
	if err := database.Create(&model.User{ID: "u1", Email: &email, PasswordHash: &oldHash, IsActive: true}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	now := time.Now()
	sessions := []model.UserSession{
		{ID: "ses-current", UserID: "u1", DeviceID: "dev-current", AuthMethod: string(AuthMethodPassword), RefreshTokenHash: strings.Repeat("a", 64), IP: "192.0.2.1", UserAgent: "current", CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(time.Hour)},
		{ID: "ses-other", UserID: "u1", DeviceID: "dev-other", AuthMethod: string(AuthMethodPassword), RefreshTokenHash: strings.Repeat("b", 64), IP: "192.0.2.2", UserAgent: "other", CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(time.Hour)},
	}
	if err := database.Create(&sessions).Error; err != nil {
		t.Fatalf("create sessions: %v", err)
	}

	service := NewAuthService(nil, database, nil, nil, nil)
	if err := service.ChangePassword(context.Background(), "u1", "ses-current", "wrong-password", "new-password-456"); !errors.Is(err, common.ErrCurrentPasswordInvalid) {
		t.Fatalf("expected current password error, got %v", err)
	}
	if err := service.ChangePassword(context.Background(), "u1", "ses-current", "old-password-123", "new-password-456"); err != nil {
		t.Fatalf("change password: %v", err)
	}

	var updated model.User
	if err := database.First(&updated, "id = ?", "u1").Error; err != nil || updated.PasswordHash == nil {
		t.Fatalf("load updated user: %v", err)
	}
	matched, err := VerifyPassword("new-password-456", *updated.PasswordHash)
	if err != nil || !matched {
		t.Fatalf("expected updated argon2id hash, matched=%t err=%v", matched, err)
	}

	var storedSessions []model.UserSession
	if err := database.Order("id").Find(&storedSessions).Error; err != nil {
		t.Fatalf("load sessions: %v", err)
	}
	if storedSessions[0].ID != "ses-current" || storedSessions[0].RevokedAt != nil {
		t.Fatalf("expected current session preserved, got %#v", storedSessions[0])
	}
	if storedSessions[1].ID != "ses-other" || storedSessions[1].RevokedAt == nil || storedSessions[1].RevokeReason == nil || *storedSessions[1].RevokeReason != "password_changed" {
		t.Fatalf("expected other session revoked after password change, got %#v", storedSessions[1])
	}
}

func newPasswordLoginTestService(t *testing.T, password string) (*AuthService, string) {
	service, _, email := newPasswordLoginTestServiceWithDB(t, password)
	return service, email
}

func newPasswordLoginTestServiceWithDB(t *testing.T, password string) (*AuthService, *gorm.DB, string) {
	t.Helper()
	gormDB, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := gormDB.AutoMigrate(&model.User{}, &model.UserEmail{}, &model.UserSession{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	email := "u1@example.com"
	if err := gormDB.Create(&model.User{ID: "u1", Email: &email, PasswordHash: &hash, IsActive: true}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return NewAuthService(nil, gormDB, kv.NewMemoryStore(), nil, nil), gormDB, email
}
