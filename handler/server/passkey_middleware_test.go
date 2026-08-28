package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"sso-server/conf"
	"sso-server/dal/kv"
	"sso-server/service/reauth"
)

func TestRequirePasskeyReauth_GrantCanProtectMultipleOperations(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := reauth.NewService(&conf.Config{Passkey: conf.PasskeyConfig{ReauthTokenTTL: time.Minute}}, kv.NewMemoryStore())
	grant, err := service.Issue(t.Context(), "user-1", "session-1", "credential-1")
	require.NoError(t, err)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", "user-1")
		c.Set("session_id", "session-1")
	})
	router.POST("/confirm", RequirePasskeyReauth(service), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	router.DELETE("/unbind", RequirePasskeyReauth(service), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	for _, testCase := range []struct {
		method string
		path   string
	}{{method: http.MethodPost, path: "/confirm"}, {method: http.MethodDelete, path: "/unbind"}} {
		request := httptest.NewRequest(testCase.method, testCase.path, nil)
		request.Header.Set("X-Reauth-Token", grant.Token)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		require.Equal(t, http.StatusNoContent, response.Code)
	}
}

func TestRequirePasskeyReauth_RejectsMissingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := reauth.NewService(&conf.Config{Passkey: conf.PasskeyConfig{ReauthTokenTTL: time.Minute}}, kv.NewMemoryStore())
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", "user-1")
		c.Set("session_id", "session-1")
	})
	router.POST("/sensitive", RequirePasskeyReauth(service), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/sensitive", nil))
	require.Equal(t, http.StatusForbidden, response.Code)
	require.Contains(t, response.Body.String(), "REAUTH_REQUIRED")
}
