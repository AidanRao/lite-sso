package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

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

func newPasswordLoginTestService(t *testing.T, password string) (*AuthService, string) {
	t.Helper()
	gormDB, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := gormDB.AutoMigrate(&model.User{}); err != nil {
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
	return NewAuthService(nil, gormDB, kv.NewMemoryStore(), nil, nil), email
}
