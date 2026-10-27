package oauth2

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"sso-server/conf"
	"sso-server/dal/kv"
	"sso-server/model"
	"sso-server/service/auth"
)

func Test_OAuthScopes_DefaultSubsetAndLiveRefresh(t *testing.T) {
	database, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	connection, err := database.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = connection.Close() })
	require.NoError(t, database.AutoMigrate(&model.User{}, &model.UserDeletion{}, &model.UserEmail{}, &model.OAuthClient{}, &model.UserSession{}, &model.UserOAuthClient{}))
	require.NoError(t, database.Create(&model.User{ID: "u1", Status: model.UserStatusActive}).Error)
	client := model.OAuthClient{ClientID: "app", Name: "App", ClientType: model.OAuthClientTypeConfidential, ClientSecret: "secret", RedirectURIs: []string{"https://app.test/callback"}, Audiences: []string{"app-api"}, AllowedScopes: []string{"read", "write"}}
	require.NoError(t, database.Create(&client).Error)
	cfg := &conf.Config{Auth: conf.AuthConfig{JWTSecret: "test-secret"}, Tokens: conf.TokenConfig{
		OAuth:   conf.OAuthTokenConfig{Issuer: "https://sso.test", SigningKID: "test", SigningPrivateKeyPEM: testPrivateKeyPEM(t), AccessTokenTTL: time.Hour, RefreshTokenTTL: time.Hour, AuthorizationCodeTTL: time.Minute},
		Session: conf.SessionTokenConfig{AccessTokenTTL: time.Hour, RefreshTokenTTL: time.Hour},
	}}
	o, err := NewWithStores(cfg, database, nil)
	require.NoError(t, err)
	_, pair, err := auth.NewAuthService(cfg, database, kv.NewMemoryStore(), nil, nil, nil).CompleteLoginWithContext(context.Background(), "u1", "", auth.LoginMetadata{DeviceID: "dev-test"}, auth.AuthMethodPassword, auth.LoginOptions{})
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/oauth/authorize", func(c *gin.Context) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), "user_id", "u1"))
		c.Set("session_id", pair.SessionID)
		o.HandleAuthorize(c)
	})
	router.POST("/oauth/token", o.HandleToken)
	authorize := func(scope *string) string {
		query := url.Values{"client_id": {"app"}, "redirect_uri": {client.RedirectURIs[0]}, "response_type": {"code"}}
		if scope != nil {
			query.Set("scope", *scope)
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest("GET", "/oauth/authorize?"+query.Encode(), nil))
		require.Equal(t, 302, recorder.Code)
		location, err := url.Parse(recorder.Header().Get("Location"))
		require.NoError(t, err)
		return location.RawQuery
	}
	request := func(form url.Values) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
		req.SetBasicAuth("app", "secret")
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		router.ServeHTTP(recorder, req)
		return recorder
	}
	parse := func(recorder *httptest.ResponseRecorder) map[string]interface{} {
		require.Equal(t, 200, recorder.Code, recorder.Body.String())
		var data map[string]interface{}
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &data))
		claims, err := o.signer.parse(data["access_token"].(string))
		require.NoError(t, err)
		require.Equal(t, strings.Join(client.Audiences, " "), strings.Join(claims.Audience, " "))
		grant, err := o.refresh.Load(context.Background(), data["refresh_token"].(string))
		require.NoError(t, err)
		require.Equal(t, claims.Scope, strings.Join(grant.Scopes, " "))
		require.Equal(t, claims.Scope, dataScope(data))
		return data
	}
	var initial map[string]interface{}
	for _, tc := range []struct {
		name     string
		scope    *string
		expected string
	}{
		{name: "default", expected: "read write"}, {name: "subset", scope: new("read"), expected: "read"}, {name: "empty", scope: new(""), expected: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			query, err := url.ParseQuery(authorize(tc.scope))
			require.NoError(t, err)
			data := parse(request(url.Values{"grant_type": {"authorization_code"}, "code": {query.Get("code")}, "redirect_uri": {client.RedirectURIs[0]}}))
			require.Equal(t, tc.expected, dataScope(data))
			if tc.name == "subset" {
				initial = data
			}
		})
	}
	require.Contains(t, authorize(new("unknown")), "invalid_scope")
	// New permissions and audiences must appear even on a previously narrowed grant.
	client.AllowedScopes = []string{"read", "write", "qr:login"}
	client.Audiences = []string{"app-api", "sso-api"}
	require.NoError(t, database.Save(&client).Error)
	data := parse(request(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {initial["refresh_token"].(string)}}))
	require.Equal(t, "read write qr:login", dataScope(data))
	refresh := data["refresh_token"].(string)
	require.Equal(t, http.StatusBadRequest, request(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "scope": {"unknown"}}).Code)
	data = parse(request(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "scope": {"qr:login"}}))
	require.Equal(t, "qr:login", dataScope(data))
	data = parse(request(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {data["refresh_token"].(string)}, "scope": {""}}))
	require.Empty(t, dataScope(data))
	client.AllowedScopes = []string{}
	client.Audiences = []string{"app-api"}
	require.NoError(t, database.Save(&client).Error)
	data = parse(request(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {data["refresh_token"].(string)}}))
	require.Empty(t, dataScope(data))
	emptyQuery, err := url.ParseQuery(authorize(nil))
	require.NoError(t, err)
	emptyData := parse(request(url.Values{"grant_type": {"authorization_code"}, "code": {emptyQuery.Get("code")}, "redirect_uri": {client.RedirectURIs[0]}}))
	require.Empty(t, dataScope(emptyData))
	// A scope revoked between authorization and exchange cannot be signed.
	client.AllowedScopes = []string{"read"}
	require.NoError(t, database.Save(&client).Error)
	query, err := url.ParseQuery(authorize(nil))
	require.NoError(t, err)
	client.AllowedScopes = []string{}
	require.NoError(t, database.Save(&client).Error)
	require.NotEqual(t, 200, request(url.Values{"grant_type": {"authorization_code"}, "code": {query.Get("code")}, "redirect_uri": {client.RedirectURIs[0]}}).Code)
}

func dataScope(data map[string]interface{}) string { scope, _ := data["scope"].(string); return scope }
