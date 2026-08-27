package user

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/conf"
	"sso-server/model"
)

type fakeAvatarStore struct {
	objectKey string
	avatarURL string
	uploadErr error
	deleteErr error
	deleted   []string
}

func (s *fakeAvatarStore) UploadImage(_ context.Context, _ string, _ string, body io.Reader, _ int64) (string, string, error) {
	if _, err := io.ReadAll(body); err != nil {
		return "", "", err
	}
	if s.uploadErr != nil {
		return "", "", s.uploadErr
	}
	return s.objectKey, s.avatarURL, nil
}

func (s *fakeAvatarStore) DeleteImage(_ context.Context, objectKey string) error {
	s.deleted = append(s.deleted, objectKey)
	return s.deleteErr
}

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

			service := NewUserService(&conf.Config{}, database, nil, nil, nil)
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

			service := NewUserService(&conf.Config{}, database, nil, nil, nil)
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

	service := NewUserService(&conf.Config{}, database, nil, nil, nil)
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

	service := NewUserService(&conf.Config{}, database, nil, nil, nil)
	err := service.UnbindThirdParty(context.Background(), "u1", "github")
	if !errors.Is(err, common.ErrThirdPartyNotBound) {
		t.Fatalf("expected ErrThirdPartyNotBound, got %v", err)
	}
}

func Test_UploadAvatar_ReplacesSystemAvatarAndDeletesOldObject(t *testing.T) {
	database := newUnbindTestDB(t)
	previousURL := "https://cdn.example.com/avatars/u1/old.png"
	previousObjectKey := "avatars/u1/old.png"
	createUnbindTestUser(t, database, &model.User{
		ID:              "u1",
		AvatarURL:       &previousURL,
		AvatarObjectKey: &previousObjectKey,
		IsActive:        true,
	})
	store := &fakeAvatarStore{
		objectKey: "avatars/u1/new.png",
		avatarURL: "https://cdn.example.com/avatars/u1/new.png",
	}
	service := NewUserService(&conf.Config{}, database, nil, nil, store)

	user, err := service.UploadAvatar(context.Background(), "u1", "image/png", ".png", bytes.NewBufferString("image-data"), 10)
	if err != nil {
		t.Fatalf("upload avatar: %v", err)
	}
	if user.AvatarURL == nil || *user.AvatarURL != store.avatarURL {
		t.Fatalf("unexpected avatar URL: %#v", user.AvatarURL)
	}
	if len(store.deleted) != 1 || store.deleted[0] != previousObjectKey {
		t.Fatalf("expected old system object to be deleted, got %#v", store.deleted)
	}

	var saved model.User
	if err := database.First(&saved, "id = ?", "u1").Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	if saved.AvatarObjectKey == nil || *saved.AvatarObjectKey != store.objectKey {
		t.Fatalf("unexpected saved object key: %#v", saved.AvatarObjectKey)
	}
}

func Test_UploadAvatar_PreservesExternalAvatarObject(t *testing.T) {
	database := newUnbindTestDB(t)
	externalURL := "https://avatars.example.com/u1.png"
	createUnbindTestUser(t, database, &model.User{ID: "u1", AvatarURL: &externalURL, IsActive: true})
	store := &fakeAvatarStore{
		objectKey: "avatars/u1/new.png",
		avatarURL: "https://cdn.example.com/avatars/u1/new.png",
	}
	service := NewUserService(&conf.Config{}, database, nil, nil, store)

	if _, err := service.UploadAvatar(context.Background(), "u1", "image/png", ".png", bytes.NewBufferString("image-data"), 10); err != nil {
		t.Fatalf("upload avatar: %v", err)
	}
	if len(store.deleted) != 0 {
		t.Fatalf("external avatar must not be deleted: %#v", store.deleted)
	}
}

func Test_UploadAvatar_CleansNewObjectWhenDatabaseUpdateFails(t *testing.T) {
	database := newUnbindTestDB(t)
	createUnbindTestUser(t, database, &model.User{ID: "u1", IsActive: true})
	if err := database.Exec("CREATE TRIGGER reject_avatar_update BEFORE UPDATE ON users BEGIN SELECT RAISE(FAIL, 'update rejected'); END;").Error; err != nil {
		t.Fatalf("create update rejection trigger: %v", err)
	}
	store := &fakeAvatarStore{
		objectKey: "avatars/u1/new.png",
		avatarURL: "https://cdn.example.com/avatars/u1/new.png",
	}
	service := NewUserService(&conf.Config{}, database, nil, nil, store)

	_, err := service.UploadAvatar(context.Background(), "u1", "image/png", ".png", bytes.NewBufferString("image-data"), 10)
	if err == nil {
		t.Fatal("expected database update error")
	}
	if len(store.deleted) != 1 || store.deleted[0] != store.objectKey {
		t.Fatalf("expected newly uploaded object cleanup, got %#v", store.deleted)
	}
}

func Test_UploadAvatar_KeepsNewAvatarWhenOldObjectDeletionFails(t *testing.T) {
	database := newUnbindTestDB(t)
	previousObjectKey := "avatars/u1/old.png"
	createUnbindTestUser(t, database, &model.User{ID: "u1", AvatarObjectKey: &previousObjectKey, IsActive: true})
	store := &fakeAvatarStore{
		objectKey: "avatars/u1/new.png",
		avatarURL: "https://cdn.example.com/avatars/u1/new.png",
		deleteErr: errors.New("delete failed"),
	}
	service := NewUserService(&conf.Config{}, database, nil, nil, store)

	user, err := service.UploadAvatar(context.Background(), "u1", "image/png", ".png", bytes.NewBufferString("image-data"), 10)
	if err != nil {
		t.Fatalf("upload avatar: %v", err)
	}
	if user.AvatarURL == nil || *user.AvatarURL != store.avatarURL {
		t.Fatalf("expected new avatar to be saved, got %#v", user.AvatarURL)
	}
	if len(store.deleted) != 1 || store.deleted[0] != previousObjectKey {
		t.Fatalf("expected old object deletion attempt, got %#v", store.deleted)
	}
}

func Test_UploadAvatar_RequiresConfiguredStore(t *testing.T) {
	database := newUnbindTestDB(t)
	service := NewUserService(&conf.Config{}, database, nil, nil, nil)

	_, err := service.UploadAvatar(context.Background(), "u1", "image/png", ".png", bytes.NewBufferString("image-data"), 10)
	if !errors.Is(err, common.ErrAvatarStorageUnavailable) {
		t.Fatalf("expected ErrAvatarStorageUnavailable, got %v", err)
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
