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
	gooauth2store "github.com/go-oauth2/oauth2/v4/store"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"sso-server/conf"
	"sso-server/dal/kv"
	"sso-server/handler/api/user"
	"sso-server/handler/oauth2"
	"sso-server/model"
)

func TestAuthRegister_CreatesUserAndReturnsToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file:auth_register_session?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.OAuthClient{}, &model.UserThirdParty{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	tokenStore, err := gooauth2store.NewMemoryTokenStore()
	if err != nil {
		t.Fatalf("token store: %v", err)
	}

	cfg := &conf.Config{}
	cfg.Security.AccessTokenExpire = time.Hour

	o, err := oauth2.NewWithStores(cfg, db, tokenStore)
	if err != nil {
		t.Fatalf("new oauth2: %v", err)
	}

	kvStore := kv.NewMemoryStore()
	_ = kvStore.Set(context.Background(), kv.KeyOTP("u1@example.com"), "123456", time.Minute)

	h := user.NewUserHandler(user.UserDeps{
		Config: cfg,
		DB:     db,
		KV:     kvStore,
		OAuth2: o,
	})

	r := gin.New()
	r.POST("/api/user/register", h.Register)

	body := `{"email":"u1@example.com","password":"password123","username":"u1","otp":"123456"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			AccessToken string `json:"access_token"`
			TokenType   string `json:"token_type"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Code != 200 {
		t.Fatalf("expected code 200, got %d", resp.Code)
	}
	if resp.Data.AccessToken == "" || resp.Data.TokenType == "" {
		t.Fatalf("expected token fields, got %s", w.Body.String())
	}

	found := false
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "sso_session" && cookie.Value != "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected sso_session cookie, got %#v", w.Result().Cookies())
	}

	var count int64
	if err := db.Model(&model.User{}).Where("email = ?", "u1@example.com").Count(&count).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 user, got %d", count)
	}
}
