package oauth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"sso-server/conf"
	"sso-server/dal/kv"
	"sso-server/handler/api/oauth"
	"sso-server/model"
)

func TestOAuthClientInfo_ReturnsClientName(t *testing.T) {
	gin.SetMode(gin.TestMode)

	gormDB, err := gorm.Open(sqlite.Open("file:oauth_client_info?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := gormDB.AutoMigrate(&model.User{}, &model.OAuthClient{}, &model.UserThirdParty{}, &model.UserOAuthClient{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := gormDB.Create(&model.OAuthClient{
		Name:         "订单系统",
		ClientID:     "order-app",
		ClientSecret: "secret",
		RedirectURI:  "http://localhost/callback",
	}).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}

	handler := oauth.NewOAuthHandler(oauth.OAuthDeps{
		Config: &conf.Config{},
		DB:     gormDB,
		KV:     kv.NewMemoryStore(),
	})

	router := gin.New()
	router.GET("/api/oauth/client", handler.ClientInfo)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/oauth/client?client_id=order-app", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			ClientID string `json:"client_id"`
			Name     string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Data.ClientID != "order-app" || resp.Data.Name != "订单系统" {
		t.Fatalf("unexpected response: %s", w.Body.String())
	}
}
