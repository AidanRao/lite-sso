package user_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"sso-server/conf"
	"sso-server/dal/kv"
	apiuser "sso-server/handler/api/user"
	serverhandler "sso-server/handler/server"
	"sso-server/model"
)

func TestUserProfile_RequiresSessionCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.OAuthClient{}, &model.UserThirdParty{}, &model.UserOAuthClient{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	h := apiuser.NewUserHandler(apiuser.UserDeps{
		Config: &conf.Config{},
		DB:     db,
		KV:     kv.NewMemoryStore(),
	})

	r := gin.New()
	r.GET("/api/user/profile", serverhandler.RequireSessionAuth(kv.NewMemoryStore()), h.GetProfile)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/user/profile", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestUserProfile_WithSessionCookieReturnsUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.OAuthClient{}, &model.UserThirdParty{}, &model.UserOAuthClient{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	email := "u1@example.com"
	if err := db.Create(&model.User{
		ID:       "u1",
		Email:    &email,
		IsActive: true,
	}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Create(&model.OAuthClient{
		Name:         "demo app",
		ClientID:     "c1",
		ClientSecret: "s1",
		RedirectURIs: `["http://localhost/cb"]`,
	}).Error; err != nil {
		t.Fatalf("create oauth client: %v", err)
	}
	if err := db.Create(&model.UserOAuthClient{
		UserID:      "u1",
		ClientID:    "c1",
		LastLoginAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("create user oauth client: %v", err)
	}
	if err := db.Create(&model.UserThirdParty{
		UserID:      "u1",
		Provider:    "github",
		ProviderUID: "gh_1",
	}).Error; err != nil {
		t.Fatalf("create third party binding: %v", err)
	}

	kvStore := kv.NewMemoryStore()
	if err := kvStore.Set(context.Background(), kv.KeySession("sid-1"), "u1", 12*time.Hour); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	h := apiuser.NewUserHandler(apiuser.UserDeps{
		Config: &conf.Config{},
		DB:     db,
		KV:     kvStore,
	})

	r := gin.New()
	r.GET("/api/user/profile", serverhandler.RequireSessionAuth(kvStore), h.GetProfile)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/user/profile", nil)
	req.AddCookie(&http.Cookie{Name: "sso_session", Value: "sid-1"})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			User struct {
				ID string `json:"id"`
			} `json:"user"`
			Applications []struct {
				ClientID string `json:"client_id"`
				Name     string `json:"name"`
			} `json:"applications"`
			ThirdPartyProviders []struct {
				Provider string `json:"provider"`
				Bound    bool   `json:"bound"`
			} `json:"third_party_providers"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Code != 200 || resp.Data.User.ID != "u1" {
		t.Fatalf("expected user u1, got %s", w.Body.String())
	}
	if len(resp.Data.Applications) != 1 || resp.Data.Applications[0].Name != "demo app" {
		t.Fatalf("expected demo app, got %s", w.Body.String())
	}
	if len(resp.Data.ThirdPartyProviders) != 2 {
		t.Fatalf("expected two providers, got %s", w.Body.String())
	}
	if resp.Data.ThirdPartyProviders[0].Provider != "github" || !resp.Data.ThirdPartyProviders[0].Bound {
		t.Fatalf("expected github bound, got %s", w.Body.String())
	}
}
