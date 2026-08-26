package user

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/conf"
	"sso-server/model"
)

func Test_UnbindThirdParty_SupportedProvider(t *testing.T) {
	for _, provider := range []string{"github", "feishu"} {
		t.Run(provider, func(t *testing.T) {
			database := newUnbindTestDB(t)
			email := provider + "@example.com"
			createUnbindTestUser(t, database, &model.User{
				ID:       "u1",
				Email:    &email,
				IsActive: true,
			})
			createUnbindTestBinding(t, database, provider)

			service := NewUserService(&conf.Config{}, database, nil, nil)
			if err := service.UnbindThirdParty(context.Background(), "u1", provider); err != nil {
				t.Fatalf("unbind %s: %v", provider, err)
			}

			var count int64
			if err := database.Model(&model.UserThirdParty{}).
				Where("user_id = ? AND provider = ?", "u1", provider).
				Count(&count).Error; err != nil {
				t.Fatalf("count binding: %v", err)
			}
			if count != 0 {
				t.Fatalf("expected binding deleted, got %d", count)
			}
		})
	}
}

func Test_UnbindThirdParty_WithoutEmail(t *testing.T) {
	testCases := []struct {
		name  string
		email *string
	}{
		{name: "nil", email: nil},
		{name: "blank", email: stringPointer(" ")},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			database := newUnbindTestDB(t)
			createUnbindTestUser(t, database, &model.User{
				ID:       "u1",
				Email:    testCase.email,
				IsActive: true,
			})
			createUnbindTestBinding(t, database, "github")

			service := NewUserService(&conf.Config{}, database, nil, nil)
			err := service.UnbindThirdParty(context.Background(), "u1", "github")
			if !errors.Is(err, common.ErrEmailRequiredForUnbind) {
				t.Fatalf("expected ErrEmailRequiredForUnbind, got %v", err)
			}

			var count int64
			if err := database.Model(&model.UserThirdParty{}).
				Where("user_id = ? AND provider = ?", "u1", "github").
				Count(&count).Error; err != nil {
				t.Fatalf("count binding: %v", err)
			}
			if count != 1 {
				t.Fatalf("expected binding preserved, got %d", count)
			}
		})
	}
}

func Test_UnbindThirdParty_InvalidProvider(t *testing.T) {
	database := newUnbindTestDB(t)
	email := "u1@example.com"
	createUnbindTestUser(t, database, &model.User{
		ID:       "u1",
		Email:    &email,
		IsActive: true,
	})

	service := NewUserService(&conf.Config{}, database, nil, nil)
	err := service.UnbindThirdParty(context.Background(), "u1", "google")
	if !errors.Is(err, common.ErrInvalidProvider) {
		t.Fatalf("expected ErrInvalidProvider, got %v", err)
	}
}

func Test_UnbindThirdParty_NotBound(t *testing.T) {
	database := newUnbindTestDB(t)
	email := "u1@example.com"
	createUnbindTestUser(t, database, &model.User{
		ID:       "u1",
		Email:    &email,
		IsActive: true,
	})

	service := NewUserService(&conf.Config{}, database, nil, nil)
	err := service.UnbindThirdParty(context.Background(), "u1", "github")
	if !errors.Is(err, common.ErrThirdPartyNotBound) {
		t.Fatalf("expected ErrThirdPartyNotBound, got %v", err)
	}
}

func newUnbindTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	databaseName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	database, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", databaseName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.AutoMigrate(&model.User{}, &model.UserThirdParty{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return database
}

func createUnbindTestUser(t *testing.T, database *gorm.DB, user *model.User) {
	t.Helper()
	if err := database.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
}

func createUnbindTestBinding(t *testing.T, database *gorm.DB, provider string) {
	t.Helper()
	if err := database.Create(&model.UserThirdParty{
		UserID:      "u1",
		Provider:    provider,
		ProviderUID: provider + "-uid",
	}).Error; err != nil {
		t.Fatalf("create binding: %v", err)
	}
}

func stringPointer(value string) *string {
	return &value
}
