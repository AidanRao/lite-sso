package oauth2_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	gooauth2store "github.com/go-oauth2/oauth2/v4/store"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"sso-server/conf"
	"sso-server/dal/kv"
	apiauth "sso-server/handler/api/auth"
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

	cfg := testOAuthConfig(t)

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
		Name:          "app",
		ClientID:      clientID,
		ClientSecret:  clientSecret,
		Audiences:     []string{"classhopper-api", "profile-api"},
		AllowedScopes: []string{"courses:read", "profile:read"},
		RedirectURI:   redirectURI,
	}).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}

	tokenStore, err := gooauth2store.NewMemoryTokenStore()
	if err != nil {
		t.Fatalf("token store: %v", err)
	}

	cfg := testOAuthConfig(t)
	cfg.Server.Port = "0"

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
	r.GET("/.well-known/jwks.json", o.HandleJWKS)
	r.POST("/api/auth/logout", apiauth.NewAuthHandler(apiauth.AuthDeps{Config: cfg, DB: db, KV: kvStore, OAuth2: o}).Logout)

	w := httptest.NewRecorder()
	confidentialAuthorizeURL := "/oauth/authorize?response_type=code&client_id=" + url.QueryEscape(clientID) + "&redirect_uri=" + url.QueryEscape(redirectURI) + "&state=xyz&scope=courses%3Aread&code_challenge=" + url.QueryEscape(codeChallenge) + "&code_challenge_method=S256"
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
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		Scope        string `json:"scope"`
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
	if tokenResp.RefreshToken == "" || tokenResp.ExpiresIn != int64(30*time.Minute/time.Second) || tokenResp.Scope != "courses:read" {
		t.Fatalf("unexpected OAuth token response: %s", w.Body.String())
	}
	var claims jwt.MapClaims
	parsed, _, err := jwt.NewParser().ParseUnverified(tokenResp.AccessToken, &claims)
	if err != nil || parsed.Header["alg"] != "RS256" || parsed.Header["kid"] == "" || claims["iss"] != "http://localhost:8080" || claims["sub"] != userID || claims["client_id"] != clientID || claims["scope"] != "courses:read" {
		t.Fatalf("unexpected JWT claims or header: %v %v %v", parsed, claims, err)
	}
	audiences, ok := claims["aud"].([]interface{})
	if !ok || len(audiences) != 2 || audiences[0] != "classhopper-api" || audiences[1] != "profile-api" {
		t.Fatalf("unexpected JWT audiences: %#v", claims["aud"])
	}
	if int64(claims["exp"].(float64)-claims["iat"].(float64)) != tokenResp.ExpiresIn {
		t.Fatalf("JWT expiry differs from response")
	}
	jwksRecorder := httptest.NewRecorder()
	r.ServeHTTP(jwksRecorder, httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil))
	if jwksRecorder.Code != http.StatusOK || !strings.Contains(jwksRecorder.Body.String(), `"kid"`) || jwksRecorder.Header().Get("Cache-Control") != "public, max-age=300" {
		t.Fatalf("JWKS endpoint invalid: status=%d body=%s", jwksRecorder.Code, jwksRecorder.Body.String())
	}

	refreshForm := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tokenResp.RefreshToken}}
	refreshRequest := func(client, secret string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(refreshForm.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.SetBasicAuth(client, secret)
		r.ServeHTTP(recorder, request)
		return recorder
	}
	if got := refreshRequest("wrong-client", "wrong-secret"); got.Code == http.StatusOK {
		t.Fatalf("other client refreshed token: %s", got.Body.String())
	}
	refreshed := refreshRequest(clientID, clientSecret)
	if refreshed.Code != http.StatusOK {
		t.Fatalf("refresh failed: %s", refreshed.Body.String())
	}
	var refreshedPair struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(refreshed.Body.Bytes(), &refreshedPair); err != nil || refreshedPair.AccessToken == "" || refreshedPair.RefreshToken == "" || refreshedPair.RefreshToken == tokenResp.RefreshToken {
		t.Fatalf("invalid refresh response: %s", refreshed.Body.String())
	}
	if got := refreshRequest(clientID, clientSecret); got.Code == http.StatusOK {
		t.Fatalf("reused refresh token: %s", got.Body.String())
	}
	refreshForm.Set("refresh_token", refreshedPair.RefreshToken)
	refreshForm.Set("scope", "profile:read")
	if got := refreshRequest(clientID, clientSecret); got.Code == http.StatusOK {
		t.Fatalf("refresh expanded original scope: %s", got.Body.String())
	}
	refreshForm.Del("scope")

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

	unauthorizedScopeURL := strings.Replace(confidentialAuthorizeURL, "scope=courses%3Aread", "scope=admin%3Awrite", 1)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, unauthorizedScopeURL, nil)
	req.AddCookie(&http.Cookie{Name: serviceauth.SessionCookieName, Value: pair.SessionID})
	r.ServeHTTP(w, req)
	if w.Code != http.StatusFound || !strings.Contains(w.Header().Get("Location"), "error=invalid_scope") {
		t.Fatalf("unregistered scope accepted: %s", w.Header().Get("Location"))
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
	var narrowedClient model.OAuthClient
	if err := db.First(&narrowedClient, "client_id = ?", clientID).Error; err != nil {
		t.Fatal(err)
	}
	narrowedClient.Audiences = []string{"profile-api"}
	narrowedClient.AllowedScopes = []string{}
	if err := db.Save(&narrowedClient).Error; err != nil {
		t.Fatal(err)
	}
	refreshForm.Set("refresh_token", refreshedPair.RefreshToken)
	narrowed := refreshRequest(clientID, clientSecret)
	if narrowed.Code != http.StatusOK {
		t.Fatalf("refresh after permission reduction failed: %s", narrowed.Body.String())
	}
	if err := json.Unmarshal(narrowed.Body.Bytes(), &refreshedPair); err != nil {
		t.Fatal(err)
	}
	narrowedClaims := jwt.MapClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(refreshedPair.AccessToken, &narrowedClaims); err != nil {
		t.Fatal(err)
	}
	narrowedAudiences, ok := narrowedClaims["aud"].([]interface{})
	if _, hasScope := narrowedClaims["scope"]; hasScope || !ok || len(narrowedAudiences) != 1 || narrowedAudiences[0] != "profile-api" {
		t.Fatalf("refresh expanded or retained removed claims: %#v", narrowedClaims)
	}

	publicRedirectURI := "lite-sso-demo://oauth/callback"
	publicClientID := "android-app"
	if err := db.Create(&model.OAuthClient{
		Name:          "Android App",
		ClientID:      publicClientID,
		ClientSecret:  "",
		ClientType:    model.OAuthClientTypePublic,
		Audiences:     []string{"classhopper-api"},
		AllowedScopes: []string{"courses:read"},
		HomepageURL:   "https://android.example.com",
		RedirectURI:   publicRedirectURI,
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
	var publicPair struct {
		RefreshToken string `json:"refresh_token"`
		AccessToken  string `json:"access_token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &publicPair); err != nil || publicPair.RefreshToken == "" {
		t.Fatalf("public client missing refresh token: %s", w.Body.String())
	}
	publicClaims := jwt.MapClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(publicPair.AccessToken, &publicClaims); err != nil {
		t.Fatal(err)
	}
	if _, hasScope := publicClaims["scope"]; hasScope {
		t.Fatalf("unsolicited scope appeared in JWT: %#v", publicClaims)
	}
	publicRefreshForm := url.Values{"grant_type": {"refresh_token"}, "client_id": {publicClientID}, "refresh_token": {publicPair.RefreshToken}}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(publicRefreshForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("public refresh failed: %s", w.Body.String())
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
	pendingRecorder := httptest.NewRecorder()
	pendingRequest := httptest.NewRequest(http.MethodGet, strings.Replace(publicAuthorizeURL, "android-state", "logout-pending", 1), nil)
	pendingRequest.AddCookie(&http.Cookie{Name: serviceauth.SessionCookieName, Value: pair.SessionID})
	r.ServeHTTP(pendingRecorder, pendingRequest)
	pendingCallback, err := url.Parse(pendingRecorder.Header().Get("Location"))
	if err != nil || pendingCallback.Query().Get("code") == "" {
		t.Fatalf("pending authorization failed: %s", pendingRecorder.Header().Get("Location"))
	}
	logoutRecorder := httptest.NewRecorder()
	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutRequest.AddCookie(&http.Cookie{Name: serviceauth.SessionCookieName, Value: pair.SessionID})
	r.ServeHTTP(logoutRecorder, logoutRequest)
	if logoutRecorder.Code != http.StatusOK {
		t.Fatalf("logout failed: %s", logoutRecorder.Body.String())
	}
	refreshForm.Set("refresh_token", refreshedPair.RefreshToken)
	if got := refreshRequest(clientID, clientSecret); got.Code == http.StatusOK {
		t.Fatalf("refresh after logout: %s", got.Body.String())
	}
	accessAfterLogout := httptest.NewRecorder()
	accessRequest := httptest.NewRequest(http.MethodGet, "/oauth/userinfo", nil)
	accessRequest.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	r.ServeHTTP(accessAfterLogout, accessRequest)
	if accessAfterLogout.Code != http.StatusOK {
		t.Fatalf("issued JWT stopped working before expiry: %s", accessAfterLogout.Body.String())
	}
	publicForm.Set("code", pendingCallback.Query().Get("code"))
	publicForm.Set("code_verifier", codeVerifier)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(publicForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Fatalf("authorization code redeemed after logout: %s", w.Body.String())
	}
}

func testOAuthConfig(t *testing.T) *conf.Config {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return &conf.Config{
		Tokens: conf.TokenConfig{
			OAuth: conf.OAuthTokenConfig{
				AccessTokenTTL:       30 * time.Minute,
				AuthorizationCodeTTL: 5 * time.Minute,
				RefreshTokenTTL:      30 * 24 * time.Hour,
				Issuer:               "http://localhost:8080",
				SigningKID:           "oauth-test-key",
				SigningPrivateKeyPEM: string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})),
			},
			Session: conf.SessionTokenConfig{AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 30 * 24 * time.Hour},
		},
		Auth: conf.AuthConfig{JWTSecret: "oauth-flow-test-jwt-secret"},
	}
}
