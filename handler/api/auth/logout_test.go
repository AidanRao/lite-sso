package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"sso-server/conf"
	"sso-server/dal/kv"
	apiauth "sso-server/handler/api/auth"
	apiuser "sso-server/handler/api/user"
	serverhandler "sso-server/handler/server"
	"sso-server/model"
)

func TestAuthLogout_InvalidatesSessionAndClearsCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file:auth_logout_session?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.OAuthClient{}, &model.UserThirdParty{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	kvStore := kv.NewMemoryStore()
	if err := kvStore.Set(context.Background(), kv.KeySession("sid-logout"), "u1", 12*time.Hour); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	h := apiauth.NewAuthHandler(apiauth.AuthDeps{
		Config: &conf.Config{},
		DB:     db,
		KV:     kvStore,
	})

	r := gin.New()
	r.POST("/api/auth/logout", serverhandler.RequireSessionAuth(kvStore), h.Logout)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "sso_session", Value: "sid-logout"})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			LoggedOut bool `json:"logged_out"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Code != 200 || !resp.Data.LoggedOut {
		t.Fatalf("expected logged_out=true, got %s", w.Body.String())
	}

	if _, err := kvStore.Get(context.Background(), kv.KeySession("sid-logout")); err == nil {
		t.Fatalf("expected session to be deleted")
	}

	foundCleared := false
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "sso_session" && cookie.Value == "" && cookie.MaxAge < 0 {
			foundCleared = true
		}
	}
	if !foundCleared {
		t.Fatalf("expected cleared session cookie, got %#v", w.Result().Cookies())
	}
}

func TestAuthLogout_LoggedOutSessionCannotAccessProfile(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file:auth_logout_profile?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.OAuthClient{}, &model.UserThirdParty{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Create(&model.User{
		ID:       "u1",
		Email:    "u1@example.com",
		IsActive: true,
	}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	kvStore := kv.NewMemoryStore()
	if err := kvStore.Set(context.Background(), kv.KeySession("sid-logout"), "u1", 12*time.Hour); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	authHandler := apiauth.NewAuthHandler(apiauth.AuthDeps{
		Config: &conf.Config{},
		DB:     db,
		KV:     kvStore,
	})
	userHandler := apiuser.NewUserHandler(apiuser.UserDeps{
		Config: &conf.Config{},
		DB:     db,
		KV:     kvStore,
	})

	r := gin.New()
	r.POST("/api/auth/logout", serverhandler.RequireSessionAuth(kvStore), authHandler.Logout)
	r.GET("/api/user/profile", serverhandler.RequireSessionAuth(kvStore), userHandler.GetProfile)

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutReq.AddCookie(&http.Cookie{Name: "sso_session", Value: "sid-logout"})
	logoutResp := httptest.NewRecorder()
	r.ServeHTTP(logoutResp, logoutReq)
	if logoutResp.Code != http.StatusOK {
		t.Fatalf("expected logout 200, got %d, body=%s", logoutResp.Code, logoutResp.Body.String())
	}

	profileReq := httptest.NewRequest(http.MethodGet, "/api/user/profile", nil)
	profileReq.AddCookie(&http.Cookie{Name: "sso_session", Value: "sid-logout"})
	profileResp := httptest.NewRecorder()
	r.ServeHTTP(profileResp, profileReq)

	if profileResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got %d, body=%s", profileResp.Code, profileResp.Body.String())
	}
}

func TestAuthLogout_WithLogoutURIs_ReturnsHTMLPage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file:auth_logout_uris?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.OAuthClient{}, &model.UserThirdParty{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if err := db.Create(&model.OAuthClient{
		Name:         "Test App",
		ClientID:     "test-client",
		ClientSecret: "secret",
		RedirectURIs: `["https://app.example.com/callback"]`,
		LogoutURIs:   `["https://app.example.com/logout", "https://app2.example.com/logout"]`,
	}).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}

	kvStore := kv.NewMemoryStore()
	if err := kvStore.Set(context.Background(), kv.KeySession("sid-logout-uris"), "u1", 12*time.Hour); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	h := apiauth.NewAuthHandler(apiauth.AuthDeps{
		Config: &conf.Config{},
		DB:     db,
		KV:     kvStore,
	})

	r := gin.New()
	r.POST("/api/auth/logout", serverhandler.RequireSessionAuth(kvStore), h.Logout)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout?redirect=https://app.example.com/home", nil)
	req.AddCookie(&http.Cookie{Name: "sso_session", Value: "sid-logout-uris"})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Fatalf("expected html content type, got %s", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "https://app.example.com/logout") {
		t.Fatalf("expected logout uri in body, got %s", body)
	}
	if !strings.Contains(body, "https://app2.example.com/logout") {
		t.Fatalf("expected second logout uri in body, got %s", body)
	}
	if !strings.Contains(body, "app.example.com\\/home") {
		t.Fatalf("expected redirect uri in body, got %s", body)
	}
}
