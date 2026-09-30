package systemadmin

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/conf"
	"sso-server/dto"
	"sso-server/model"
)

type fakeImageStore struct {
	objectKey string
	imageURL  string
	uploadErr error
	deleteErr error
	deleted   []string
}

func (s *fakeImageStore) UploadImage(_ context.Context, _ string, _ string, body io.Reader, _ int64) (string, string, error) {
	if _, err := io.ReadAll(body); err != nil {
		return "", "", err
	}
	if s.uploadErr != nil {
		return "", "", s.uploadErr
	}
	return s.objectKey, s.imageURL, nil
}

func (s *fakeImageStore) DeleteImage(_ context.Context, objectKey string) error {
	s.deleted = append(s.deleted, objectKey)
	return s.deleteErr
}

func Test_UploadOAuthClientLogo_ReplacesExistingLogo(t *testing.T) {
	database := newOAuthClientLogoTestDB(t)
	previousURL := "https://cdn.example.com/logos/old.png"
	previousObjectKey := "logos/old.png"
	client := createOAuthClientForLogoTest(t, database, &previousURL, &previousObjectKey)
	store := &fakeImageStore{objectKey: "logos/new.png", imageURL: "https://cdn.example.com/logos/new.png"}
	service := NewAdminService(&conf.Config{}, database, store)

	response, err := service.UploadOAuthClientLogo(context.Background(), client.ID, "image/png", ".png", bytes.NewBufferString("image"), 5)
	if err != nil {
		t.Fatalf("upload logo: %v", err)
	}
	if response.LogoURL == nil || *response.LogoURL != store.imageURL {
		t.Fatalf("unexpected logo response: %#v", response.LogoURL)
	}
	if len(store.deleted) != 1 || store.deleted[0] != previousObjectKey {
		t.Fatalf("expected old logo cleanup, got %#v", store.deleted)
	}

	var saved model.OAuthClient
	if err := database.First(&saved, client.ID).Error; err != nil {
		t.Fatalf("load client: %v", err)
	}
	if saved.LogoObjectKey == nil || *saved.LogoObjectKey != store.objectKey {
		t.Fatalf("unexpected object key: %#v", saved.LogoObjectKey)
	}
}

func Test_UploadOAuthClientLogo_CleansNewObjectWhenDatabaseUpdateFails(t *testing.T) {
	database := newOAuthClientLogoTestDB(t)
	client := createOAuthClientForLogoTest(t, database, nil, nil)
	if err := database.Exec("CREATE TRIGGER reject_logo_update BEFORE UPDATE ON oauth_clients BEGIN SELECT RAISE(FAIL, 'update rejected'); END;").Error; err != nil {
		t.Fatalf("create trigger: %v", err)
	}
	store := &fakeImageStore{objectKey: "logos/new.png", imageURL: "https://cdn.example.com/logos/new.png"}
	service := NewAdminService(&conf.Config{}, database, store)

	_, err := service.UploadOAuthClientLogo(context.Background(), client.ID, "image/png", ".png", bytes.NewBufferString("image"), 5)
	if err == nil {
		t.Fatal("expected update error")
	}
	if len(store.deleted) != 1 || store.deleted[0] != store.objectKey {
		t.Fatalf("expected new logo cleanup, got %#v", store.deleted)
	}
}

func Test_ClearOAuthClientLogo_RemovesAssignmentAndObject(t *testing.T) {
	database := newOAuthClientLogoTestDB(t)
	logoURL := "https://cdn.example.com/logos/current.png"
	logoObjectKey := "logos/current.png"
	client := createOAuthClientForLogoTest(t, database, &logoURL, &logoObjectKey)
	store := &fakeImageStore{}
	service := NewAdminService(&conf.Config{}, database, store)

	response, err := service.ClearOAuthClientLogo(context.Background(), client.ID)
	if err != nil {
		t.Fatalf("clear logo: %v", err)
	}
	if response.LogoURL != nil {
		t.Fatalf("expected cleared logo url, got %#v", response.LogoURL)
	}
	if len(store.deleted) != 1 || store.deleted[0] != logoObjectKey {
		t.Fatalf("expected old logo cleanup, got %#v", store.deleted)
	}
}

func Test_UploadOAuthClientLogo_RequiresStorageAndClient(t *testing.T) {
	database := newOAuthClientLogoTestDB(t)
	service := NewAdminService(&conf.Config{}, database, nil)

	_, err := service.UploadOAuthClientLogo(context.Background(), 1, "image/png", ".png", bytes.NewBufferString("image"), 5)
	if !errors.Is(err, common.ErrLogoStorageUnavailable) {
		t.Fatalf("expected storage error, got %v", err)
	}

	service = NewAdminService(&conf.Config{}, database, &fakeImageStore{})
	_, err = service.UploadOAuthClientLogo(context.Background(), 1, "image/png", ".png", bytes.NewBufferString("image"), 5)
	if !errors.Is(err, common.ErrOAuthClientNotFound) {
		t.Fatalf("expected missing client error, got %v", err)
	}
}

func Test_OAuthClientClaims_CreateAndUpdate(t *testing.T) {
	database := newOAuthClientLogoTestDB(t)
	service := NewAdminService(&conf.Config{}, database, nil)
	request := dto.CreateOAuthClientRequest{Name: "Android", ClientID: "android-app", ClientType: "public", HomepageURL: "https://app.example.com", RedirectURI: "app://callback", Audiences: []string{"classhopper-api", "profile-api"}, AllowedScopes: []string{"courses:read", "profile:read"}}
	created, err := service.CreateOAuthClient(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(created.Audiences) != 2 || len(created.AllowedScopes) != 2 {
		t.Fatalf("claim configuration was not returned: %#v", created)
	}
	updated, err := service.UpdateOAuthClient(context.Background(), created.ID, dto.UpdateOAuthClientRequest{Name: request.Name, ClientType: &request.ClientType, HomepageURL: request.HomepageURL, RedirectURI: request.RedirectURI, Audiences: []string{"classhopper-api"}, AllowedScopes: []string{"courses:read"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Audiences) != 1 || updated.Audiences[0] != "classhopper-api" {
		t.Fatalf("claim update failed: %#v", updated)
	}
	var saved model.OAuthClient
	if err := database.First(&saved, created.ID).Error; err != nil {
		t.Fatal(err)
	}
	if len(saved.AllowedScopes) != 1 || saved.AllowedScopes[0] != "courses:read" {
		t.Fatalf("claim configuration not persisted: %#v", saved)
	}
}

func newOAuthClientLogoTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.AutoMigrate(&model.OAuthClient{}); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	return database
}

func createOAuthClientForLogoTest(t *testing.T, database *gorm.DB, logoURL *string, logoObjectKey *string) *model.OAuthClient {
	t.Helper()
	client := &model.OAuthClient{
		Name:          "demo",
		ClientID:      "demo-client",
		ClientSecret:  "secret",
		HomepageURL:   "https://demo.example.com",
		RedirectURI:   "https://demo.example.com/callback",
		LogoURL:       logoURL,
		LogoObjectKey: logoObjectKey,
	}
	if err := database.Create(client).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	return client
}
