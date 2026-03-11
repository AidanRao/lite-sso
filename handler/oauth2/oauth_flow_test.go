package oauth2_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	gooauth2store "github.com/go-oauth2/oauth2/v4/store"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"sso-server/conf"
	"sso-server/dal/kv"
	"sso-server/handler/api/oauth"
	"sso-server/handler/oauth2"
	"sso-server/model"
)

func TestOAuth2_AuthorizeTokenUserinfo_Flow(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.OAuthClient{}, &model.UserThirdParty{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	userID := "u1"
	if err := db.Create(&model.User{ID: userID, Email: "u1@example.com", IsActive: true}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	redirectURI := "http://localhost/cb"
	clientID := "c1"
	clientSecret := "s1"
	if err := db.Create(&model.OAuthClient{
		Name:         "app",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURIs: `["` + redirectURI + `"]`,
	}).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}

	tokenStore, err := gooauth2store.NewMemoryTokenStore()
	if err != nil {
		t.Fatalf("token store: %v", err)
	}

	cfg := &conf.Config{}
	cfg.Server.Port = "0"
	cfg.Security.AccessTokenExpire = time.Hour
	cfg.Dev.UserID = userID

	o, err := oauth2.NewWithStores(cfg, db, tokenStore)
	if err != nil {
		t.Fatalf("new oauth2: %v", err)
	}

	// Create OAuth handler
	kvStore := kv.NewMemoryStore()
	oauthHandler := oauth.NewOAuthHandler(oauth.OAuthDeps{
		Config: cfg,
		DB:     db,
		KV:     kvStore,
		OAuth2: o,
	})

	r := gin.New()
	r.GET("/oauth/authorize", o.HandleAuthorize)
	r.POST("/oauth/token", o.HandleToken)
	r.GET("/oauth/userinfo", oauthHandler.HandleUserinfo)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?response_type=code&client_id="+url.QueryEscape(clientID)+"&redirect_uri="+url.QueryEscape(redirectURI)+"&state=xyz", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d, body=%s", w.Code, w.Body.String())
	}
	loc := w.Header().Get("Location")
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("parse location: %v", err)
	}
	code := u.Query().Get("code")
	if code == "" {
		t.Fatalf("expected code in redirect, got %q", loc)
	}
	if u.Query().Get("state") != "xyz" {
		t.Fatalf("expected state=xyz, got %q", u.Query().Get("state"))
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &tokenResp); err != nil {
		t.Fatalf("unmarshal token: %v", err)
	}
	if tokenResp.AccessToken == "" {
		t.Fatalf("expected access_token, got %s", w.Body.String())
	}
	if tokenResp.TokenType == "" {
		t.Fatalf("expected token_type, got %s", w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/oauth/userinfo", nil)
	req.Header.Set("Authorization", tokenResp.TokenType+" "+tokenResp.AccessToken)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), userID) {
		t.Fatalf("expected body contains user id, got %s", w.Body.String())
	}
}
