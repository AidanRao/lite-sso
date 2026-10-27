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
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"sso-server/conf"
	"sso-server/dal/kv"
	"sso-server/handler/api/auth"
	"sso-server/model"
	serviceauth "sso-server/service/auth"
)

func Test_LoginHandlers_PersistChoiceAndRefreshCookies(t *testing.T) {
	for _, method := range []string{"password", "email"} {
		for _, remember := range []bool{false, true} {
			name := method + "/temporary"
			if remember {
				name = method + "/remembered"
			}
			t.Run(name, func(t *testing.T) {
				database, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
				require.NoError(t, err)
				connection, err := database.DB()
				require.NoError(t, err)
				t.Cleanup(func() { _ = connection.Close() })
				require.NoError(t, database.AutoMigrate(&model.User{}, &model.UserDeletion{}, &model.UserEmail{}, &model.UserSession{}))
				email := "person@example.com"
				hash, err := serviceauth.HashPassword("password123456")
				require.NoError(t, err)
				require.NoError(t, database.Create(&model.User{ID: "u1", Email: &email, PasswordHash: &hash, Status: model.UserStatusActive}).Error)
				cfg := &conf.Config{Auth: conf.AuthConfig{JWTSecret: "test-secret"}, Server: conf.ServerConfig{CookieSecure: true}, Dev: conf.DevConfig{FixedEmailOTP: "123456", SkipSendMessage: true}, Tokens: conf.TokenConfig{Session: conf.SessionTokenConfig{AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 30 * 24 * time.Hour, TemporarySessionTTL: 12 * time.Hour}}}
				store := kv.NewMemoryStore()
				handler := auth.NewAuthHandler(auth.AuthDeps{Config: cfg, DB: database, KV: store})
				router := gin.New()
				router.POST("/api/auth/login/password", handler.LoginWithPassword)
				router.POST("/api/auth/login/email", handler.LoginWithEmailOTP)
				router.POST("/api/auth/token/refresh", handler.RefreshToken)
				input := map[string]any{"email": email, "password": "password123456", "remember_me": remember}
				if method == "email" {
					require.NoError(t, store.Set(context.Background(), kv.KeyCaptcha("send"), "1234", time.Minute))
					challenge, err := handler.Service().SendEmailOTP(context.Background(), email, "send", "1234", serviceauth.OTPRequestContext{DeviceID: "dev_browser"}, serviceauth.ChallengePurposeLogin)
					require.NoError(t, err)
					input = map[string]any{"challenge_id": challenge.ChallengeID, "code": "123456", "remember_me": remember}
				}
				body, err := json.Marshal(input)
				require.NoError(t, err)
				request := httptest.NewRequest(http.MethodPost, "/api/auth/login/"+method, strings.NewReader(string(body)))
				request.Header.Set("Content-Type", "application/json")
				request.AddCookie(&http.Cookie{Name: serviceauth.DeviceCookieName, Value: "dev_browser"})
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)
				require.Equal(t, http.StatusOK, response.Code, response.Body.String())
				var session model.UserSession
				require.NoError(t, database.First(&session).Error)
				require.Equal(t, remember, session.RememberMe)
				expected := 12 * time.Hour
				if remember {
					expected = 30 * 24 * time.Hour
				}
				require.Equal(t, expected, session.ExpiresAt.Sub(session.CreatedAt))
				var refresh *http.Cookie
				for _, cookie := range response.Result().Cookies() {
					if cookie.Name == serviceauth.RefreshTokenCookieName {
						refresh = cookie
					}
					if cookie.Name != serviceauth.RefreshTokenCookieName && cookie.Name != serviceauth.SessionCookieName {
						continue
					}
					require.Equal(t, remember, cookie.MaxAge > 0)
					require.Equal(t, !remember, cookie.Expires.IsZero())
				}
				require.NotNil(t, refresh)
				// A client cannot upgrade the persisted choice during refresh.
				request = httptest.NewRequest(http.MethodPost, "/api/auth/token/refresh", strings.NewReader(`{"remember_me":true}`))
				request.Header.Set("Content-Type", "application/json")
				request.AddCookie(refresh)
				response = httptest.NewRecorder()
				router.ServeHTTP(response, request)
				require.Equal(t, http.StatusOK, response.Code, response.Body.String())
				require.Len(t, response.Result().Cookies(), 2)
				for _, cookie := range response.Result().Cookies() {
					require.Equal(t, remember, cookie.MaxAge > 0)
					if remember {
						require.WithinDuration(t, session.ExpiresAt, cookie.Expires, time.Second)
					} else {
						require.True(t, cookie.Expires.IsZero())
					}
				}
			})
		}
	}
}
