package auth

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/dal/kv"
	"sso-server/model"
)

func Test_LoginWithPassword_LocksAfterFailedAttempts(t *testing.T) {
	service, email := newPasswordLoginTestService(t, "password123")

	for i := 1; i < passwordLoginFailureLimit; i++ {
		_, err := service.LoginWithPassword(context.Background(), email, "wrong-password")
		if !errors.Is(err, common.ErrInvalidCredentials) {
			t.Fatalf("attempt %d expected invalid credentials, got %v", i, err)
		}
	}

	_, err := service.LoginWithPassword(context.Background(), email, "wrong-password")
	if !errors.Is(err, common.ErrAccountLocked) {
		t.Fatalf("expected account locked, got %v", err)
	}
	var lockedError common.AccountLockedError
	if !errors.As(err, &lockedError) {
		t.Fatalf("expected account locked error")
	}
	if lockedError.RetryAfterSeconds <= 0 || lockedError.RetryAfterSeconds > int(passwordLoginLockDuration/time.Second) {
		t.Fatalf("expected retry seconds within lock duration, got %d", lockedError.RetryAfterSeconds)
	}

	_, err = service.LoginWithPassword(context.Background(), email, "password123")
	if !errors.Is(err, common.ErrAccountLocked) {
		t.Fatalf("expected account locked with correct password, got %v", err)
	}
	if !errors.As(err, &lockedError) {
		t.Fatalf("expected account locked error with correct password")
	}
	if lockedError.RetryAfterSeconds <= 0 || lockedError.RetryAfterSeconds > int(passwordLoginLockDuration/time.Second) {
		t.Fatalf("expected retry seconds within lock duration, got %d", lockedError.RetryAfterSeconds)
	}
}

func Test_LoginWithPassword_ClearsFailuresAfterSuccess(t *testing.T) {
	service, email := newPasswordLoginTestService(t, "password123")

	for i := 1; i < passwordLoginFailureLimit; i++ {
		_, err := service.LoginWithPassword(context.Background(), email, "wrong-password")
		if !errors.Is(err, common.ErrInvalidCredentials) {
			t.Fatalf("attempt %d expected invalid credentials, got %v", i, err)
		}
	}

	user, err := service.LoginWithPassword(context.Background(), email, "password123")
	if err != nil {
		t.Fatalf("expected successful login, got %v", err)
	}
	if user == nil || user.ID == "" {
		t.Fatalf("expected user")
	}

	for i := 1; i < passwordLoginFailureLimit; i++ {
		_, err := service.LoginWithPassword(context.Background(), email, "wrong-password")
		if !errors.Is(err, common.ErrInvalidCredentials) {
			t.Fatalf("attempt %d after success expected invalid credentials, got %v", i, err)
		}
	}
}

func Test_RetryAfterSecondsFromDuration_ReturnsSeconds(t *testing.T) {
	got := retryAfterSecondsFromDuration(8 * time.Minute)
	if got != 480 {
		t.Fatalf("expected retry seconds, got %d", got)
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

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	hashStr := string(hash)
	email := "u1@example.com"

	if err := gormDB.Create(&model.User{
		ID:           "u1",
		Email:        &email,
		PasswordHash: &hashStr,
		IsActive:     true,
	}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	return NewAuthService(nil, gormDB, kv.NewMemoryStore(), nil, nil), email
}
