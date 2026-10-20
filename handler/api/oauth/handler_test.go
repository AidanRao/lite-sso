package oauth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"sso-server/conf"
	"sso-server/dal/kv"
	"sso-server/handler/api/oauth"
	"sso-server/handler/server"
	"sso-server/manager/thirdparty"
	"sso-server/model"
	"sso-server/service/auth"
)

type googleTestProvider struct {
	calls int
	email string
}

func Test_OAuthHandler_GoogleCallback_CreatesBrowserSessionAndRejectsReplay(t *testing.T) {
	for _, email := range []string{"alice@gmail.com", ""} {
		t.Run("email="+email, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
			require.NoError(t, err)
			require.NoError(t, database.AutoMigrate(&model.User{}, &model.UserDeletion{}, &model.UserEmail{}, &model.UserEmailSource{}, &model.UserThirdParty{}, &model.UserSession{}))
			provider := &googleTestProvider{email: email}
			registry := thirdparty.NewRegistry()
			require.NoError(t, registry.Register(thirdparty.Google, "GOOGLE", provider))
			registry.Freeze()
			config := &conf.Config{
				Server: conf.ServerConfig{CookieSecure: true},
				Auth:   conf.AuthConfig{JWTSecret: "google-test-session-secret-at-least-32-bytes"},
				Tokens: conf.TokenConfig{Session: conf.SessionTokenConfig{AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 24 * time.Hour}},
			}
			store := kv.NewMemoryStore()
			handler := oauth.NewOAuthHandler(oauth.OAuthDeps{Config: config, DB: database, KV: store, ProviderRegistry: registry})
			router := gin.New()
			router.GET("/api/auth/third/:provider", handler.ThirdPartyLogin)
			router.GET("/api/auth/third/:provider/callback", handler.ThirdPartyCallback)
			router.GET("/oauth/authorize", server.RequireSessionAuthOrRedirect(auth.NewAuthService(config, database, store, nil, nil, nil)), func(c *gin.Context) { c.Status(http.StatusNoContent) })

			redirect := "/oauth/authorize?client_id=demo"
			start := httptest.NewRecorder()
			router.ServeHTTP(start, httptest.NewRequest(http.MethodGet, "/api/auth/third/google?redirect="+url.QueryEscape(redirect), nil))
			require.Equal(t, http.StatusTemporaryRedirect, start.Code)
			location, err := url.Parse(start.Header().Get("Location"))
			require.NoError(t, err)
			state := location.Query().Get("state")
			require.NotEmpty(t, state)
			callbackURL := "/api/auth/third/google/callback?code=auth-code&state=" + url.QueryEscape(state)
			callback := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, callbackURL, nil)
			for _, cookie := range start.Result().Cookies() {
				request.AddCookie(cookie)
			}
			router.ServeHTTP(callback, request)
			require.Equal(t, http.StatusTemporaryRedirect, callback.Code)
			if email == "" {
				setup, err := url.Parse(callback.Header().Get("Location"))
				require.NoError(t, err)
				require.Equal(t, "/setup-email", setup.Path)
				require.Equal(t, redirect, setup.Query().Get("redirect"))
			} else {
				require.Equal(t, redirect, callback.Header().Get("Location"))
			}
			var sessionCookie *http.Cookie
			foundRefresh := false
			for _, cookie := range callback.Result().Cookies() {
				switch cookie.Name {
				case auth.SessionCookieName:
					sessionCookie = cookie
					require.Equal(t, "/", cookie.Path)
					require.True(t, cookie.Secure)
					require.True(t, cookie.HttpOnly)
				case auth.RefreshTokenCookieName:
					foundRefresh = true
				}
			}
			require.NotNil(t, sessionCookie)
			require.True(t, foundRefresh)
			var session model.UserSession
			require.NoError(t, database.First(&session, "id = ?", sessionCookie.Value).Error)
			require.Equal(t, "GOOGLE", session.AuthMethod)
			var binding model.UserThirdParty
			require.NoError(t, database.First(&binding, "user_id = ? AND provider = ?", session.UserID, thirdparty.Google).Error)
			require.NotNil(t, binding.BoundAt)
			require.NotNil(t, binding.LastUsedAt)
			require.True(t, session.CreatedAt.Equal(*binding.LastUsedAt))
			lastUsed := *binding.LastUsedAt
			authorize := httptest.NewRecorder()
			request = httptest.NewRequest(http.MethodGet, redirect, nil)
			request.AddCookie(sessionCookie)
			router.ServeHTTP(authorize, request)
			require.Equal(t, http.StatusNoContent, authorize.Code)

			replay := httptest.NewRecorder()
			router.ServeHTTP(replay, httptest.NewRequest(http.MethodGet, callbackURL, nil))
			require.Empty(t, replay.Result().Cookies())
			require.NotEqual(t, redirect, replay.Header().Get("Location"))
			require.Equal(t, 1, provider.calls)
			require.NoError(t, database.First(&binding, binding.ID).Error)
			require.True(t, lastUsed.Equal(*binding.LastUsedAt))
			if email == "" {
				// The established binding must prompt again on the next login, without creating another user.
				nextStart := httptest.NewRecorder()
				router.ServeHTTP(nextStart, httptest.NewRequest(http.MethodGet, "/api/auth/third/google?redirect="+url.QueryEscape(redirect), nil))
				nextLocation, err := url.Parse(nextStart.Header().Get("Location"))
				require.NoError(t, err)
				nextRequest := httptest.NewRequest(http.MethodGet, "/api/auth/third/google/callback?code=next-code&state="+url.QueryEscape(nextLocation.Query().Get("state")), nil)
				for _, cookie := range nextStart.Result().Cookies() {
					nextRequest.AddCookie(cookie)
				}
				nextCallback := httptest.NewRecorder()
				router.ServeHTTP(nextCallback, nextRequest)
				require.Equal(t, callback.Header().Get("Location"), nextCallback.Header().Get("Location"))
				require.Equal(t, 2, provider.calls)
				var userCount int64
				require.NoError(t, database.Model(&model.User{}).Count(&userCount).Error)
				require.Equal(t, int64(1), userCount)
			}
		})
	}
}

func (p *googleTestProvider) Configured() bool {
	return true
}

func (p *googleTestProvider) AuthCodeURL(state string) string {
	return "https://accounts.google.com/o/oauth2/v2/auth?state=" + url.QueryEscape(state)
}

func (p *googleTestProvider) FetchProfile(_ context.Context, _ string) (*thirdparty.Profile, error) {
	p.calls++
	return &thirdparty.Profile{Provider: thirdparty.Google, ProviderUID: "google-sub", Email: p.email}, nil
}
