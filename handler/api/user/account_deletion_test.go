package user_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	"sso-server/handler/api/auth"
	"sso-server/handler/api/user"
	"sso-server/handler/server"
	"sso-server/model"
	serviceauth "sso-server/service/auth"
	"sso-server/service/reauth"
	serviceuser "sso-server/service/user"
)

func Test_AccountDeletionHTTP_VerificationRestrictionAndPayloadSeparation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	connection, err := database.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = connection.Close() })
	require.NoError(t, database.AutoMigrate(&model.User{}, &model.UserDeletion{}, &model.UserCleanupTask{}, &model.UserEmail{}, &model.UserSession{}))
	require.NoError(t, database.Create(&model.User{ID: "owner", Status: model.UserStatusActive}).Error)
	cfg := &conf.Config{Auth: conf.AuthConfig{JWTSecret: "test-secret"}, Tokens: conf.TokenConfig{Session: conf.SessionTokenConfig{AccessTokenTTL: time.Hour, RefreshTokenTTL: 48 * time.Hour}, Reauth: conf.ReauthTokenConfig{GrantTTL: 5 * time.Minute}}}
	store := kv.NewMemoryStore()
	authService := serviceauth.NewAuthService(cfg, database, store, nil, nil, nil)
	reauthService := reauth.NewService(reauth.Deps{Config: cfg, DB: database, Store: store, Auth: authService})
	handler := user.NewUserHandler(user.UserDeps{Config: cfg, DB: database, KV: store, Deletion: serviceuser.NewDeletionService(database, nil, nil)})
	authHandler := auth.NewAuthHandler(auth.AuthDeps{Config: cfg, DB: database, KV: store})
	router := gin.New()
	protected := router.Group("/api/user", server.RequireSessionAuth(authService))
	protected.GET("/profile", handler.GetProfile)
	protected.GET("/account-deletion", handler.GetAccountDeletion)
	protected.POST("/account-deletion", server.RequireReauth(reauthService), handler.RequestAccountDeletion)
	protected.DELETE("/account-deletion", handler.CheckAccountDeletionWindow, server.RequireReauth(reauthService), handler.CancelAccountDeletion)
	router.POST("/api/auth/token/refresh", authHandler.RefreshToken)
	router.GET("/oauth/authorize", server.RequireSessionAuthOrRedirect(authService), func(c *gin.Context) { c.Status(200) })
	call := func(method, path, token, grant string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, path, nil)
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		if grant != "" {
			request.Header.Set("X-Reauth-Token", grant)
		}
		for _, cookie := range cookies {
			request.AddCookie(cookie)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}
	require.Equal(t, 401, call("POST", "/api/user/account-deletion", "", "").Code)
	_, original, err := authService.CompleteLoginWithContext(t.Context(), "owner", "", serviceauth.LoginMetadata{}, "GITHUB")
	require.NoError(t, err)
	require.NoError(t, database.Model(&model.UserSession{}).Where("id = ?", original.SessionID).Update("created_at", time.Now().Add(-time.Hour)).Error)
	denied := call("POST", "/api/user/account-deletion", original.AccessToken, "")
	require.Equal(t, 403, denied.Code)
	require.Contains(t, denied.Body.String(), "REAUTH_REQUIRED")
	var count int64
	require.NoError(t, database.Model(&model.UserDeletion{}).Count(&count).Error)
	require.Zero(t, count)
	proof, err := reauthService.Issue(t.Context(), "owner", original.SessionID, reauth.MethodPasskey, "credential")
	require.NoError(t, err)
	created := call("POST", "/api/user/account-deletion", original.AccessToken, proof.Token)
	require.Equal(t, 202, created.Code, created.Body.String())
	require.Contains(t, created.Body.String(), "deletion_scheduled_at")
	for _, cookie := range created.Result().Cookies() {
		require.Negative(t, cookie.MaxAge)
	}
	require.Equal(t, 401, call("GET", "/api/user/profile", original.AccessToken, "").Code)
	result, limited, err := authService.CompleteLoginWithContext(t.Context(), "owner", "/oauth/authorize", serviceauth.LoginMetadata{}, "GITHUB")
	require.NoError(t, err)
	payload, err := json.Marshal(result)
	require.NoError(t, err)
	require.Contains(t, string(payload), `"status":"pending_deletion"`)
	require.NotContains(t, string(payload), "deletion_scheduled_at")
	require.NotContains(t, string(payload), "deletion_requested_at")
	blocked := call("GET", "/api/user/profile", limited.AccessToken, "")
	require.Equal(t, 403, blocked.Code)
	require.Contains(t, blocked.Body.String(), "ACCOUNT_PENDING_DELETION")
	details := call("GET", "/api/user/account-deletion", limited.AccessToken, "")
	require.Equal(t, 200, details.Code)
	require.Contains(t, details.Body.String(), "deletion_scheduled_at")
	browser := call("GET", "/oauth/authorize", "", "", &http.Cookie{Name: serviceauth.SessionCookieName, Value: limited.SessionID})
	require.Equal(t, "/account/deletion", browser.Header().Get("Location"))
	refreshed := call("POST", "/api/auth/token/refresh", "", "", &http.Cookie{Name: serviceauth.RefreshTokenCookieName, Value: limited.RefreshToken})
	require.Equal(t, 200, refreshed.Code)
	require.Contains(t, refreshed.Body.String(), `"status":"pending_deletion"`)
	require.NotContains(t, refreshed.Body.String(), "deletion_scheduled_at")
	cancelled := call("DELETE", "/api/user/account-deletion", limited.AccessToken, "")
	require.Equal(t, 200, cancelled.Code, cancelled.Body.String())
	require.Equal(t, 401, call("GET", "/api/user/account-deletion", limited.AccessToken, "").Code)
	_, fresh, err := authService.CompleteLoginWithContext(t.Context(), "owner", "", serviceauth.LoginMetadata{}, "GITHUB")
	require.NoError(t, err)
	profile := call("GET", "/api/user/profile", fresh.AccessToken, "")
	require.Equal(t, 200, profile.Code)
	require.Contains(t, profile.Body.String(), `"status":"active"`)
	require.NotContains(t, profile.Body.String(), "deletion_")
}
