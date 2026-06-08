package admin_test

import (
	"bytes"
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
	"sso-server/handler/api/admin"
	serverhandler "sso-server/handler/server"
	"sso-server/model"
	serviceauth "sso-server/service/auth"
)

func TestAdminUsers_RequiresConfiguredAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newAdminTestDB(t)
	kvStore := kv.NewMemoryStore()
	cfg := &conf.Config{
		Admin: conf.AdminConfig{UserIDs: []string{"admin-user"}},
	}

	createAdminTestUser(t, database, "admin-user", "admin@example.com")
	createAdminTestUser(t, database, "normal-user", "normal@example.com")
	createAdminTestSession(t, kvStore, "sid-admin", "admin-user")
	createAdminTestSession(t, kvStore, "sid-normal", "normal-user")

	router := newAdminTestRouter(cfg, database, kvStore)

	for _, tc := range []struct {
		name       string
		sessionID  string
		wantStatus int
	}{
		{name: "normal user", sessionID: "sid-normal", wantStatus: http.StatusForbidden},
		{name: "admin user", sessionID: "sid-admin", wantStatus: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
			req.AddCookie(&http.Cookie{Name: serviceauth.SessionCookieName, Value: tc.sessionID})
			router.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d, body=%s", tc.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestAdminOAuthClients_CreateAndUpdate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newAdminTestDB(t)
	kvStore := kv.NewMemoryStore()
	cfg := &conf.Config{
		Admin: conf.AdminConfig{UserIDs: []string{"admin-user"}},
	}

	createAdminTestUser(t, database, "admin-user", "admin@example.com")
	createAdminTestSession(t, kvStore, "sid-admin", "admin-user")
	router := newAdminTestRouter(cfg, database, kvStore)

	createBody := `{"name":"订单系统","client_id":"order-app","client_secret":"secret-1","homepage_url":"https://order.example.com","redirect_uri":"https://order.example.com/callback","logout_uri":"https://order.example.com/logout"}`
	createResp := doAdminJSONRequest(t, router, http.MethodPost, "/api/admin/oauth-clients", "sid-admin", createBody)
	if createResp.Code != http.StatusOK {
		t.Fatalf("expected create 200, got %d, body=%s", createResp.Code, createResp.Body.String())
	}

	var created struct {
		Data struct {
			Client struct {
				ID          uint   `json:"id"`
				HomepageURL string `json:"homepage_url"`
			} `json:"client"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createResp.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}
	if created.Data.Client.ID == 0 {
		t.Fatalf("expected created client id, body=%s", createResp.Body.String())
	}
	if created.Data.Client.HomepageURL != "https://order.example.com" {
		t.Fatalf("expected homepage url in response, body=%s", createResp.Body.String())
	}

	updateBody := `{"name":"订单中心","client_id":"order-center","homepage_url":"https://order.example.com/home","redirect_uri":"https://order.example.com/oauth/callback","logout_uri":"https://order.example.com/logout"}`
	updateResp := doAdminJSONRequest(t, router, http.MethodPut, "/api/admin/oauth-clients/1", "sid-admin", updateBody)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("expected update 200, got %d, body=%s", updateResp.Code, updateResp.Body.String())
	}

	var client model.OAuthClient
	if err := database.First(&client, "id = ?", created.Data.Client.ID).Error; err != nil {
		t.Fatalf("find client: %v", err)
	}
	if client.Name != "订单中心" || client.ClientID != "order-center" {
		t.Fatalf("client not updated: %+v", client)
	}
	if client.HomepageURL != "https://order.example.com/home" {
		t.Fatalf("expected homepage url updated, got %q", client.HomepageURL)
	}
	if client.ClientSecret != "secret-1" {
		t.Fatalf("expected secret preserved, got %q", client.ClientSecret)
	}
	if client.RedirectURI != "https://order.example.com/oauth/callback" {
		t.Fatalf("unexpected redirect uri: %s", client.RedirectURI)
	}
	if client.LogoutURI != "https://order.example.com/logout" {
		t.Fatalf("unexpected logout uri: %s", client.LogoutURI)
	}
}

func TestAdminOAuthClientSecret_ReturnsSecretSeparately(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newAdminTestDB(t)
	kvStore := kv.NewMemoryStore()
	cfg := &conf.Config{
		Admin: conf.AdminConfig{UserIDs: []string{"admin-user"}},
	}

	createAdminTestUser(t, database, "admin-user", "admin@example.com")
	createAdminTestSession(t, kvStore, "sid-admin", "admin-user")
	if err := database.Create(&model.OAuthClient{
		Name:         "订单系统",
		ClientID:     "order-app",
		ClientSecret: "secret-1",
		HomepageURL:  "https://order.example.com",
		RedirectURI:  "https://order.example.com/callback",
		LogoutURI:    "https://order.example.com/logout",
	}).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}

	router := newAdminTestRouter(cfg, database, kvStore)
	listResp := doAdminJSONRequest(t, router, http.MethodGet, "/api/admin/oauth-clients", "sid-admin", "")
	if bytes.Contains(listResp.Body.Bytes(), []byte("secret-1")) {
		t.Fatalf("list response should not include secret, body=%s", listResp.Body.String())
	}

	secretResp := doAdminJSONRequest(t, router, http.MethodGet, "/api/admin/oauth-clients/1/secret", "sid-admin", "")
	if secretResp.Code != http.StatusOK {
		t.Fatalf("expected secret 200, got %d, body=%s", secretResp.Code, secretResp.Body.String())
	}

	var resp struct {
		Data struct {
			Secret struct {
				ID           uint   `json:"id"`
				ClientID     string `json:"client_id"`
				ClientSecret string `json:"client_secret"`
			} `json:"secret"`
		} `json:"data"`
	}
	if err := json.Unmarshal(secretResp.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal secret response: %v", err)
	}
	if resp.Data.Secret.ID != 1 || resp.Data.Secret.ClientID != "order-app" || resp.Data.Secret.ClientSecret != "secret-1" {
		t.Fatalf("unexpected secret response: %s", secretResp.Body.String())
	}
}

func TestAdminUserDetail_ReturnsProfileOverview(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newAdminTestDB(t)
	kvStore := kv.NewMemoryStore()
	cfg := &conf.Config{
		Admin: conf.AdminConfig{UserIDs: []string{"admin-user"}},
	}

	createAdminTestUser(t, database, "admin-user", "admin@example.com")
	createAdminTestUser(t, database, "target-user", "target@example.com")
	createAdminTestSession(t, kvStore, "sid-admin", "admin-user")
	if err := database.Create(&model.OAuthClient{
		Name:         "demo app",
		ClientID:     "demo",
		ClientSecret: "secret",
		HomepageURL:  "https://demo.example.com",
		RedirectURI:  "https://demo.example.com/callback",
	}).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := database.Create(&model.UserOAuthClient{
		UserID:      "target-user",
		ClientID:    "demo",
		LastLoginAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("create user oauth client: %v", err)
	}
	if err := database.Create(&model.UserThirdParty{
		UserID:      "target-user",
		Provider:    "github",
		ProviderUID: "gh_target",
	}).Error; err != nil {
		t.Fatalf("create third party: %v", err)
	}

	router := newAdminTestRouter(cfg, database, kvStore)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/users/target-user", nil)
	req.AddCookie(&http.Cookie{Name: serviceauth.SessionCookieName, Value: "sid-admin"})
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			Profile struct {
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
			} `json:"profile"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Data.Profile.User.ID != "target-user" {
		t.Fatalf("expected target-user, got %s", w.Body.String())
	}
	if len(resp.Data.Profile.Applications) != 1 || resp.Data.Profile.Applications[0].Name != "demo app" {
		t.Fatalf("expected demo app, got %s", w.Body.String())
	}
	if len(resp.Data.Profile.ThirdPartyProviders) != 2 || !resp.Data.Profile.ThirdPartyProviders[0].Bound {
		t.Fatalf("expected github bound provider, got %s", w.Body.String())
	}
}

func newAdminTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.AutoMigrate(&model.User{}, &model.OAuthClient{}, &model.UserThirdParty{}, &model.UserOAuthClient{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return database
}

func newAdminTestRouter(cfg *conf.Config, database *gorm.DB, kvStore kv.Store) *gin.Engine {
	handler := admin.NewAdminHandler(admin.AdminDeps{
		Config: cfg,
		DB:     database,
	})

	router := gin.New()
	group := router.Group("/api/admin")
	group.Use(serverhandler.RequireSessionAuth(kvStore), serverhandler.RequireAdmin(cfg))
	group.GET("/users", handler.ListUsers)
	group.GET("/users/:id", handler.GetUserDetail)
	group.GET("/oauth-clients/:id/secret", handler.GetOAuthClientSecret)
	group.POST("/oauth-clients", handler.CreateOAuthClient)
	group.PUT("/oauth-clients/:id", handler.UpdateOAuthClient)
	return router
}

func createAdminTestUser(t *testing.T, database *gorm.DB, id string, email string) {
	t.Helper()

	user := &model.User{
		ID:        id,
		Email:     &email,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := database.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
}

func createAdminTestSession(t *testing.T, kvStore kv.Store, sessionID string, userID string) {
	t.Helper()

	if err := kvStore.Set(context.Background(), kv.KeySession(sessionID), userID, time.Hour); err != nil {
		t.Fatalf("create session: %v", err)
	}
}

func doAdminJSONRequest(t *testing.T, router *gin.Engine, method string, path string, sessionID string, body string) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: serviceauth.SessionCookieName, Value: sessionID})
	router.ServeHTTP(w, req)
	return w
}
