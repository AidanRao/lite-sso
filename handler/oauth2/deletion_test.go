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
	"sso-server/manager/oauth"
	"sso-server/model"
	"sso-server/service/auth"
)

func Test_OAuthDeletion_DurableCutoffBlocksOldCodeRefreshAndOnlineJWT(t *testing.T) {
	database, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	connection, err := database.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = connection.Close() })
	require.NoError(t, database.AutoMigrate(&model.User{}, &model.UserDeletion{}, &model.UserEmail{}, &model.OAuthClient{}, &model.UserSession{}, &model.UserOAuthClient{}))
	require.NoError(t, database.Create(&model.User{ID: "owner", Status: model.UserStatusActive}).Error)
	cfg := &conf.Config{Auth: conf.AuthConfig{JWTSecret: "secret"}, Tokens: conf.TokenConfig{Session: conf.SessionTokenConfig{AccessTokenTTL: time.Hour, RefreshTokenTTL: time.Hour}, OAuth: conf.OAuthTokenConfig{Issuer: "https://sso.test", SigningKID: "test", SigningPrivateKeyPEM: testPrivateKeyPEM(t), AccessTokenTTL: time.Hour, RefreshTokenTTL: time.Hour, AuthorizationCodeTTL: time.Minute}}}
	client := model.OAuthClient{ClientID: "app", Name: "App", ClientType: model.OAuthClientTypeConfidential, ClientSecret: "secret", RedirectURIs: []string{"https://app.test/callback"}, Audiences: []string{"app"}, AllowedScopes: []string{"read"}}
	require.NoError(t, database.Create(&client).Error)
	server, err := NewWithStores(cfg, database, nil)
	require.NoError(t, err)
	login := auth.NewAuthService(cfg, database, kv.NewMemoryStore(), nil, nil, nil)
	_, pair, err := login.CompleteLoginWithContext(t.Context(), "owner", "", auth.LoginMetadata{}, auth.AuthMethodPassword, auth.LoginOptions{})
	require.NoError(t, err)
	router := gin.New()
	router.GET("/oauth/authorize", func(c *gin.Context) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), "user_id", "owner"))
		c.Set("session_id", pair.SessionID)
		server.HandleAuthorize(c)
	})
	router.POST("/oauth/token", server.HandleToken)
	authorize := func() string {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", "/oauth/authorize?response_type=code&client_id=app&redirect_uri="+url.QueryEscape(client.RedirectURIs[0]), nil))
		require.Equal(t, 302, response.Code, response.Body.String())
		location, err := url.Parse(response.Header().Get("Location"))
		require.NoError(t, err)
		return location.Query().Get("code")
	}
	request := func(form url.Values) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
		r.SetBasicAuth("app", "secret")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, r)
		return response
	}
	code := authorize()
	pendingCode := authorize()
	response := request(url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {client.RedirectURIs[0]}})
	require.Equal(t, 200, response.Code, response.Body.String())
	var tokens map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &tokens))
	oldRefresh := tokens["refresh_token"].(string)
	oldAccess := tokens["access_token"].(string)
	cutoff := time.Now()
	deletion := model.UserDeletion{ID: uuid.NewString(), UserID: "owner", RequestedAt: cutoff, ScheduledAt: cutoff.Add(24 * time.Hour), Status: "pending"}
	require.NoError(t, database.Create(&deletion).Error)
	require.NoError(t, database.Model(&model.User{}).Where("id = ?", "owner").Update("status", model.UserStatusPendingDeletion).Error)
	require.Equal(t, 401, request(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {oldRefresh}}).Code)
	require.Equal(t, 401, request(url.Values{"grant_type": {"authorization_code"}, "code": {pendingCode}, "redirect_uri": {client.RedirectURIs[0]}}).Code)
	r := httptest.NewRequest(http.MethodGet, "/oauth/userinfo", nil)
	r.Header.Set("Authorization", "Bearer "+oldAccess)
	_, err = server.ValidateToken(r)
	require.Error(t, err)
	// Simulate cancellation while Redis is unavailable: no physical refresh cleanup has occurred.
	require.NoError(t, database.Model(&model.UserDeletion{}).Where("id = ?", deletion.ID).Update("status", "cancelled").Error)
	require.NoError(t, database.Model(&model.User{}).Where("id = ?", "owner").Update("status", model.UserStatusActive).Error)
	require.Equal(t, 401, request(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {oldRefresh}}).Code)
	_, err = server.ValidateToken(r)
	require.Error(t, err)
	var association model.UserOAuthClient
	require.NoError(t, database.First(&association, "user_id = ? AND client_id = ?", "owner", "app").Error)
	grant := oauth.RefreshGrant{AuthorizationID: association.AuthorizationID, UserID: "owner", ClientID: "app", AuthorizedAt: cutoff.Add(time.Second).UnixMicro(), ExpiresAt: time.Now().Add(time.Hour)}
	require.NoError(t, server.refresh.Issue(t.Context(), "fresh", grant))
	require.NoError(t, server.refresh.RevokeUserBefore(t.Context(), "owner", cutoff))
	require.Equal(t, 200, request(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {"fresh"}}).Code)
}
