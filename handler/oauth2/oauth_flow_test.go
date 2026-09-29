package oauth2_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
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
	serverhandler "sso-server/handler/server"
	"sso-server/model"
	serviceauth "sso-server/service/auth"
)

func TestOAuth2_Authorize_RequiresSession(t *testing.T) {
	dsn := "file:oauthtest_requires_session?mode=memory&cache=shared"
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.UserEmail{}, &model.OAuthClient{}, &model.UserThirdParty{}, &model.UserOAuthClient{}, &model.UserSession{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if err := db.Create(&model.OAuthClient{
		Name:         "app",
		ClientID:     "c1",
		ClientSecret: "s1",
		RedirectURI:  "http://localhost/cb",
	}).Error; err != nil {
		t.Fatalf("create client: %v", err)
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

	r := gin.New()
	r.GET("/oauth/authorize", serverhandler.RequireSessionAuth(kv.NewMemoryStore()), o.HandleAuthorize)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?response_type=code&client_id=c1&redirect_uri="+url.QueryEscape("http://localhost/cb"), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestOAuth2_AuthorizeTokenUserinfo_Flow(t *testing.T) {
	dsn := "file:oauthtest_authorize_flow?mode=memory&cache=shared"
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.UserEmail{}, &model.OAuthClient{}, &model.UserThirdParty{}, &model.UserOAuthClient{}, &model.UserSession{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	userID := "u1"
	email := "u1@example.com"
	if err := db.Create(&model.User{ID: userID, Email: &email, IsActive: true}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	verifiedAt := time.Now()
	if err := db.Create(&model.UserEmail{ID: "ue-secondary", UserID: userID, Email: "secondary@example.com", VerifiedAt: &verifiedAt}).Error; err != nil {
		t.Fatalf("create secondary email: %v", err)
	}

	redirectURI := "http://localhost:8000/auth/sso/callback"
	clientID := "c1"
	clientSecret := "s1"
	codeVerifier := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_"
	codeChallengeDigest := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(codeChallengeDigest[:])
	if err := db.Create(&model.OAuthClient{
		Name:         "app",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
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

	o, err := oauth2.NewWithStores(cfg, db, tokenStore)
	if err != nil {
		t.Fatalf("new oauth2: %v", err)
	}

	kvStore := kv.NewMemoryStore()
	oauthHandler := oauth.NewOAuthHandler(oauth.OAuthDeps{
		Config: cfg,
		DB:     db,
		KV:     kvStore,
		OAuth2: o,
	})
	authService := serviceauth.NewAuthService(cfg, db, kvStore, nil, o)
	_, pair, err := authService.CompleteLoginWithContext(context.Background(), userID, "", serviceauth.LoginMetadata{
		DeviceID:  "dev-1",
		IP:        "192.0.2.1",
		UserAgent: "oauth-flow-test",
	}, serviceauth.AuthMethodPassword)
	if err != nil {
		t.Fatalf("create persistent session: %v", err)
	}

	r := gin.New()
	r.GET("/oauth/authorize", serverhandler.RequireSessionAuthOrRedirect(authService), o.HandleAuthorize)
	r.POST("/oauth/token", o.HandleToken)
	r.GET("/oauth/userinfo", oauthHandler.HandleUserinfo)

	w := httptest.NewRecorder()
	confidentialAuthorizeURL := "/oauth/authorize?response_type=code&client_id=" + url.QueryEscape(clientID) + "&redirect_uri=" + url.QueryEscape(redirectURI) + "&state=xyz&code_challenge=" + url.QueryEscape(codeChallenge) + "&code_challenge_method=S256"
	req := httptest.NewRequest(http.MethodGet, confidentialAuthorizeURL, nil)
	req.AddCookie(&http.Cookie{Name: serviceauth.SessionCookieName, Value: pair.SessionID})
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
	var appLogin model.UserOAuthClient
	if err := db.First(&appLogin, "user_id = ? AND client_id = ?", userID, clientID).Error; err != nil {
		t.Fatalf("expected authorized application record: %v", err)
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("code_verifier", codeVerifier)

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
	if !strings.Contains(w.Body.String(), email) || strings.Contains(w.Body.String(), "secondary@example.com") {
		t.Fatalf("expected userinfo to expose only primary email, got %s", w.Body.String())
	}

	for _, credential := range []struct {
		name   string
		secret string
	}{
		{name: "missing secret"},
		{name: "incorrect secret", secret: "incorrect-secret"},
	} {
		t.Run(credential.name, func(t *testing.T) {
			authorizeURL := strings.Replace(confidentialAuthorizeURL, "state=xyz", "state="+url.QueryEscape(credential.name), 1)
			authorizeRecorder := httptest.NewRecorder()
			authorizeRequest := httptest.NewRequest(http.MethodGet, authorizeURL, nil)
			authorizeRequest.AddCookie(&http.Cookie{Name: serviceauth.SessionCookieName, Value: pair.SessionID})
			r.ServeHTTP(authorizeRecorder, authorizeRequest)
			if authorizeRecorder.Code != http.StatusFound {
				t.Fatalf("expected authorization 302, got %d, body=%s", authorizeRecorder.Code, authorizeRecorder.Body.String())
			}
			callback, parseErr := url.Parse(authorizeRecorder.Header().Get("Location"))
			if parseErr != nil {
				t.Fatalf("parse callback: %v", parseErr)
			}
			credentialForm := url.Values{}
			credentialForm.Set("grant_type", "authorization_code")
			credentialForm.Set("client_id", clientID)
			credentialForm.Set("code", callback.Query().Get("code"))
			credentialForm.Set("redirect_uri", redirectURI)
			credentialForm.Set("code_verifier", codeVerifier)
			tokenRecorder := httptest.NewRecorder()
			tokenRequest := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(credentialForm.Encode()))
			tokenRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if credential.secret != "" {
				tokenRequest.SetBasicAuth(clientID, credential.secret)
			}
			r.ServeHTTP(tokenRecorder, tokenRequest)
			if tokenRecorder.Code == http.StatusOK {
				t.Fatalf("expected %s to fail, got %s", credential.name, tokenRecorder.Body.String())
			}
		})
	}

	publicRedirectURI := "lite-sso-demo://oauth/callback"
	publicClientID := "android-app"
	if err := db.Create(&model.OAuthClient{
		Name:         "Android App",
		ClientID:     publicClientID,
		ClientSecret: "",
		ClientType:   model.OAuthClientTypePublic,
		HomepageURL:  "https://android.example.com",
		RedirectURI:  publicRedirectURI,
	}).Error; err != nil {
		t.Fatalf("create public client: %v", err)
	}

	publicAuthorizeURL := "/oauth/authorize?response_type=code&client_id=" + url.QueryEscape(publicClientID) +
		"&redirect_uri=" + url.QueryEscape(publicRedirectURI) + "&state=android-state&code_challenge=" + url.QueryEscape(codeChallenge) +
		"&code_challenge_method=S256"
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, publicAuthorizeURL, nil)
	req.AddCookie(&http.Cookie{Name: serviceauth.SessionCookieName, Value: pair.SessionID})
	r.ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("expected public authorization 302, got %d, body=%s", w.Code, w.Body.String())
	}
	publicCallback, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse public callback: %v", err)
	}
	publicCode := publicCallback.Query().Get("code")
	if publicCode == "" || publicCallback.Query().Get("state") != "android-state" {
		t.Fatalf("expected public authorization code and state, got %q", publicCallback.String())
	}

	publicForm := url.Values{}
	publicForm.Set("grant_type", "authorization_code")
	publicForm.Set("client_id", publicClientID)
	publicForm.Set("code", publicCode)
	publicForm.Set("redirect_uri", publicRedirectURI)
	publicForm.Set("code_verifier", codeVerifier)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(publicForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected public token exchange 200, got %d, body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	badVerifierAuthorizeURL := strings.Replace(publicAuthorizeURL, "android-state", "bad-verifier-state", 1)
	req = httptest.NewRequest(http.MethodGet, badVerifierAuthorizeURL, nil)
	req.AddCookie(&http.Cookie{Name: serviceauth.SessionCookieName, Value: pair.SessionID})
	r.ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("expected second public authorization 302, got %d, body=%s", w.Code, w.Body.String())
	}
	badVerifierCallback, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse second public callback: %v", err)
	}
	publicForm.Set("code", badVerifierCallback.Query().Get("code"))
	publicForm.Set("code_verifier", "wrong-verifier")
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(publicForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Fatalf("expected wrong code verifier to fail, got %s", w.Body.String())
	}

	missingVerifierAuthorizeURL := strings.Replace(publicAuthorizeURL, "android-state", "missing-verifier-state", 1)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, missingVerifierAuthorizeURL, nil)
	req.AddCookie(&http.Cookie{Name: serviceauth.SessionCookieName, Value: pair.SessionID})
	r.ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("expected third public authorization 302, got %d, body=%s", w.Code, w.Body.String())
	}
	missingVerifierCallback, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse third public callback: %v", err)
	}
	publicForm.Set("code", missingVerifierCallback.Query().Get("code"))
	publicForm.Set("code_verifier", "")
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(publicForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Fatalf("expected missing code verifier to fail, got %s", w.Body.String())
	}

	missingChallengeURL := "/oauth/authorize?response_type=code&client_id=" + url.QueryEscape(publicClientID) + "&redirect_uri=" + url.QueryEscape(publicRedirectURI)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, missingChallengeURL, nil)
	req.AddCookie(&http.Cookie{Name: serviceauth.SessionCookieName, Value: pair.SessionID})
	r.ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("expected missing public challenge to redirect with error, got %d, body=%s", w.Code, w.Body.String())
	}
	missingChallengeCallback, err := url.Parse(w.Header().Get("Location"))
	if err != nil || missingChallengeCallback.Query().Get("error") == "" {
		t.Fatalf("expected callback error for missing public challenge, got %q", w.Header().Get("Location"))
	}
}
