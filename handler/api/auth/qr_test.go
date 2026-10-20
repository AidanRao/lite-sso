package auth_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"sso-server/conf"
	"sso-server/dal/kv"
	"sso-server/handler/api/auth"
	"sso-server/handler/oauth2"
	"sso-server/handler/server"
	"sso-server/model"
)

func Test_QRHandlers_OAuthPermissionsBrowserIsolationAndContinuation(t *testing.T) {
	database, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	connection, err := database.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = connection.Close() })
	require.NoError(t, database.AutoMigrate(&model.User{}, &model.UserDeletion{}, &model.UserEmail{}, &model.OAuthClient{}, &model.UserSession{}, &model.UserOAuthClient{}))
	require.NoError(t, database.Create(&model.User{ID: "u1", Status: model.UserStatusActive}).Error)
	require.NoError(t, database.Create(&model.OAuthClient{Name: "Scanner", ClientID: "scanner", Audiences: []string{"sso-api"}, AllowedScopes: []string{"qr:login"}}).Error)
	require.NoError(t, database.Create(&model.UserOAuthClient{UserID: "u1", ClientID: "scanner", LastLoginAt: time.Now()}).Error)
	require.NoError(t, database.Create(&model.OAuthClient{Name: "Target", ClientID: "target", ClientSecret: "secret", ClientType: model.OAuthClientTypeConfidential, RedirectURIs: []string{"https://target.test/callback"}, Audiences: []string{"target-api"}}).Error)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	cfg := &conf.Config{Auth: conf.AuthConfig{JWTSecret: "session-test-secret"}, Server: conf.ServerConfig{CookieSecure: true}, Tokens: conf.TokenConfig{
		OAuth:   conf.OAuthTokenConfig{Issuer: "https://sso.test", SigningKID: "test", SigningPrivateKeyPEM: string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})), AccessTokenTTL: time.Hour, RefreshTokenTTL: time.Hour, AuthorizationCodeTTL: time.Minute},
		Session: conf.SessionTokenConfig{AccessTokenTTL: time.Hour, RefreshTokenTTL: time.Hour},
	}}
	oauthServer, err := oauth2.NewWithStores(cfg, database, nil)
	require.NoError(t, err)
	handler := auth.NewAuthHandler(auth.AuthDeps{Config: cfg, DB: database, KV: kv.NewMemoryStore(), OAuth2: oauthServer})
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/auth/qr/generate", handler.GenerateQRCode)
	router.GET("/api/auth/qr/poll", handler.PollQRCode)
	router.POST("/api/auth/qr/scan", handler.ScanQRCode)
	router.POST("/api/auth/qr/confirm", handler.ConfirmQRCode)
	router.POST("/api/auth/qr/reject", handler.RejectQRCode)
	router.POST("/api/auth/qr/cancel", handler.CancelQRCode)
	router.POST("/api/auth/qr/complete", handler.CompleteQRCode)
	router.GET("/oauth/authorize", server.RequireSessionAuthOrRedirect(handler.Service()), oauthServer.HandleAuthorize)
	call := func(method, path, body, token, browser string, cookies []*http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		req.Header.Set("X-QR-Browser-Token", browser)
		if method == "GET" && strings.HasPrefix(path, "/api/auth/qr/generate") {
			req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")
			req.RemoteAddr = "203.0.113.10:8080"
		} else {
			req.Header.Set("User-Agent", "Scanner/1.0")
		}
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		return recorder
	}
	decode := func(recorder *httptest.ResponseRecorder) map[string]interface{} {
		require.Equal(t, 200, recorder.Code, recorder.Body.String())
		require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
		var payload struct {
			Data map[string]interface{} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
		return payload.Data
	}
	sign := func(scopes string, audiences []string) string {
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{"iss": cfg.Tokens.OAuth.Issuer, "sub": "u1", "client_id": "scanner", "scope": scopes, "aud": audiences, "exp": time.Now().Add(time.Hour).Unix()})
		token.Header["kid"] = "test"
		raw, err := token.SignedString(key)
		require.NoError(t, err)
		return raw
	}
	redirect := "/oauth/authorize?response_type=code&client_id=target&redirect_uri=" + url.QueryEscape("https://target.test/callback") + "&state=keep-original&code_challenge=" + strings.Repeat("a", 43) + "&code_challenge_method=S256"
	generated := call("GET", "/api/auth/qr/generate?redirect="+url.QueryEscape(redirect), "", "", "", nil)
	challenge := decode(generated)
	code, browser := challenge["code"].(string), challenge["browser_token"].(string)
	cookies := generated.Result().Cookies()
	require.Len(t, cookies, 1)
	body := `{"code":"` + code + `"}`
	require.Equal(t, 403, call("GET", "/api/auth/qr/poll?code="+code, "", "", "", cookies).Code)
	require.Equal(t, 403, call("GET", "/api/auth/qr/poll?code="+code, "", "", browser, nil).Code)
	require.Equal(t, 401, call("POST", "/api/auth/qr/scan", body, "", "", cookies).Code)
	for _, claims := range []jwt.MapClaims{
		{"iss": "https://other.test", "sub": "u1", "client_id": "scanner", "scope": "qr:login", "aud": []string{"sso-api"}, "exp": time.Now().Add(time.Hour).Unix()},
		{"iss": cfg.Tokens.OAuth.Issuer, "sub": "u1", "client_id": "scanner", "scope": "qr:login", "aud": []string{"sso-api"}, "exp": time.Now().Add(-time.Minute).Unix()},
	} {
		invalid := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		invalid.Header["kid"] = "test"
		raw, err := invalid.SignedString(key)
		require.NoError(t, err)
		require.Equal(t, 401, call("POST", "/api/auth/qr/scan", body, raw, "", nil).Code)
	}
	validToken := sign("qr:login", []string{"sso-api"})
	parts := strings.Split(validToken, ".")
	parts[2] = strings.Repeat("A", len(parts[2]))
	require.Equal(t, 401, call("POST", "/api/auth/qr/scan", body, strings.Join(parts, "."), "", nil).Code)
	for _, token := range []string{sign("", []string{"sso-api"}), sign("qr:login", []string{"target-api"})} {
		denied := call("POST", "/api/auth/qr/scan", body, token, "", nil)
		require.Equal(t, 403, denied.Code)
		require.Contains(t, denied.Body.String(), "QR_TOKEN_PERMISSION_MISSING")
	}
	token := sign("qr:login", []string{"sso-api"})
	preview := decode(call("POST", "/api/auth/qr/scan", body, token, "", nil))
	require.Equal(t, "Target", preview["target_name"])
	require.NotContains(t, preview, "login_ticket")
	require.NotContains(t, preview, "user_agent")
	require.NotContains(t, preview, "ip")
	environment := preview["environment"].(map[string]interface{})
	require.Equal(t, "Chrome · macOS", environment["display_name"])
	require.Equal(t, "203.0.113.10", environment["ip"])
	require.Equal(t, "desktop", environment["device_type"])
	decode(call("POST", "/api/auth/qr/confirm", body, token, "", nil))
	ticket := decode(call("GET", "/api/auth/qr/poll?code="+code, "", "", browser, cookies))["login_ticket"].(string)
	completeBody := `{"code":"` + code + `","login_ticket":"` + ticket + `"}`
	require.Equal(t, 403, call("POST", "/api/auth/qr/complete", completeBody, "", "wrong", cookies).Code)
	complete := call("POST", "/api/auth/qr/complete", completeBody, "", browser, cookies)
	result := decode(complete)
	setupURL, err := url.Parse(result["redirect_url"].(string))
	require.NoError(t, err)
	require.Equal(t, "/setup-email", setupURL.Path)
	require.Equal(t, redirect, setupURL.Query().Get("redirect"))
	sessionCookies := complete.Result().Cookies()
	require.Len(t, sessionCookies, 2)
	for _, cookie := range sessionCookies {
		require.True(t, cookie.HttpOnly)
		require.True(t, cookie.Secure)
		require.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
	}
	require.Equal(t, 401, call("POST", "/api/auth/qr/scan", body, sessionCookies[0].Value, "", nil).Code)
	authorized := call("GET", redirect, "", "", "", sessionCookies)
	require.Equal(t, 302, authorized.Code)
	callback, err := url.Parse(authorized.Header().Get("Location"))
	require.NoError(t, err)
	require.Equal(t, "keep-original", callback.Query().Get("state"))
	require.NotEmpty(t, callback.Query().Get("code"))
	require.Equal(t, 409, call("POST", "/api/auth/qr/complete", completeBody, "", browser, cookies).Code)
	require.NoError(t, database.Model(&model.OAuthClient{}).Where("client_id = ?", "scanner").Update("allowed_scopes", "[]").Error)
	denied := call("POST", "/api/auth/qr/scan", body, token, "", nil)
	require.Equal(t, 403, denied.Code)
	require.Contains(t, denied.Body.String(), "QR_CLIENT_FORBIDDEN")
	require.NoError(t, database.Model(&model.OAuthClient{}).Where("client_id = ?", "scanner").Update("allowed_scopes", `["qr:login"]`).Error)
	require.NoError(t, database.Model(&model.User{}).Where("id = ?", "u1").Update("status", model.UserStatusDisabled).Error)
	inactive := call("POST", "/api/auth/qr/scan", body, token, "", nil)
	require.Equal(t, 403, inactive.Code)
	require.Contains(t, inactive.Body.String(), "USER_INACTIVE")
	require.NoError(t, database.Where("client_id = ?", "scanner").Delete(&model.OAuthClient{}).Error)
	require.Equal(t, 401, call("POST", "/api/auth/qr/scan", body, token, "", nil).Code)
}
