package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"sso-server/handler/api/auth"
	serviceauth "sso-server/service/auth"
)

func Test_WriteLoginCookies_UsesResolvedSessionPolicy(t *testing.T) {
	for _, remember := range []bool{false, true} {
		name := "temporary"
		if remember {
			name = "remembered"
		}
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(response)
			expiry := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
			pair := &serviceauth.TokenPair{SessionID: "ses-test", RefreshToken: "refresh-secret", RememberMe: remember, SessionExpiresAt: expiry}
			auth.WriteLoginCookies(context, pair, true)
			cookies := response.Result().Cookies()
			require.Len(t, cookies, 2)
			for _, cookie := range cookies {
				require.True(t, cookie.HttpOnly)
				require.True(t, cookie.Secure)
				require.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
				if cookie.Name == serviceauth.SessionCookieName {
					require.Equal(t, "/", cookie.Path)
					require.Equal(t, pair.SessionID, cookie.Value)
				} else {
					require.Equal(t, serviceauth.RefreshTokenCookieName, cookie.Name)
					require.Equal(t, "/api/auth", cookie.Path)
					require.Equal(t, pair.RefreshToken, cookie.Value)
				}
				if remember {
					require.InDelta(t, 7200, cookie.MaxAge, 2)
					require.True(t, expiry.Equal(cookie.Expires))
				} else {
					require.Zero(t, cookie.MaxAge)
					require.True(t, cookie.Expires.IsZero())
				}
			}
		})
	}
}

func Test_WriteLoginCookies_ExpiredRememberedSessionClearsCredentials(t *testing.T) {
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	auth.WriteLoginCookies(context, &serviceauth.TokenPair{SessionID: "expired", RememberMe: true, SessionExpiresAt: time.Now().Add(-time.Second)}, true)
	for _, cookie := range response.Result().Cookies() {
		require.Empty(t, cookie.Value)
		require.Equal(t, -1, cookie.MaxAge)
	}
}
