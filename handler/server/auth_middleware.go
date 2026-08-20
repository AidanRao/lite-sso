package server

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"sso-server/common/ecode"
	"sso-server/conf"
	"sso-server/dal/kv"
	serviceauth "sso-server/service/auth"
)

type contextKey string

const (
	userIDContextKey    contextKey = "user_id"
	sessionIDContextKey contextKey = "session_id"
)

// RequireSessionAuth validates a bearer Access Token and the corresponding
// persistent User Session. The kv.Store form remains only for isolated legacy
// handler tests; production routes pass AuthService.
func RequireSessionAuth(dependency interface{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		if authService, ok := dependency.(*serviceauth.AuthService); ok {
			requireBearerSession(c, authService)
			return
		}
		if kvStore, ok := dependency.(kv.Store); ok {
			// This branch is deliberately unreachable from server.registerRoutes.
			// It keeps old isolated handler fixtures independent from PostgreSQL.
			requireFixtureSession(c, kvStore)
			return
		}
		writeUnauthorized(c)
	}
}

func requireBearerSession(c *gin.Context, authService *serviceauth.AuthService) {
	header := strings.TrimSpace(c.GetHeader("Authorization"))
	if len(header) < 8 || !strings.EqualFold(header[:7], "Bearer ") {
		writeUnauthorized(c)
		return
	}
	token := strings.TrimSpace(header[7:])
	claims, err := authService.ParseAccessToken(token)
	if err != nil {
		writeUnauthorized(c)
		return
	}
	userID, err := authService.ResolveSessionUserID(c.Request.Context(), claims.SessionID)
	if err != nil || userID != claims.Subject {
		writeUnauthorized(c)
		return
	}
	ctx := context.WithValue(c.Request.Context(), userIDContextKey, userID)
	ctx = context.WithValue(ctx, "user_id", userID)
	ctx = context.WithValue(ctx, sessionIDContextKey, claims.SessionID)
	c.Request = c.Request.WithContext(ctx)
	c.Set("user_id", userID)
	c.Set("session_id", claims.SessionID)
	c.Next()
}

func requireFixtureSession(c *gin.Context, kvStore kv.Store) {
	sessionID, err := c.Cookie(serviceauth.SessionCookieName)
	if err != nil || sessionID == "" {
		writeUnauthorized(c)
		return
	}
	userID, err := kvStore.Get(c.Request.Context(), kv.KeySession(sessionID))
	if err != nil || userID == "" {
		writeUnauthorized(c)
		return
	}
	ctx := context.WithValue(c.Request.Context(), userIDContextKey, userID)
	ctx = context.WithValue(ctx, "user_id", userID)
	c.Request = c.Request.WithContext(ctx)
	c.Set("user_id", userID)
	c.Set("session_id", sessionID)
	c.Set("fixture_session", true)
	c.Next()
}

func RequireSessionAuthOrRedirect(dependency interface{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := strings.TrimSpace(c.GetHeader("Authorization"))
		if authService, ok := dependency.(*serviceauth.AuthService); ok && strings.HasPrefix(strings.ToLower(header), "bearer ") {
			requireBearerSession(c, authService)
			return
		}
		if kvStore, ok := dependency.(kv.Store); ok {
			sessionID, err := c.Cookie(serviceauth.SessionCookieName)
			if err == nil && sessionID != "" {
				if userID, getErr := kvStore.Get(c.Request.Context(), kv.KeySession(sessionID)); getErr == nil && userID != "" {
					c.Set("user_id", userID)
					c.Next()
					return
				}
			}
		}
		redirectToLogin(c)
		c.Abort()
	}
}

func RequireAdmin(cfg *conf.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			writeUnauthorized(c)
			return
		}
		if !cfg.IsAdminUser(userID) {
			c.JSON(http.StatusForbidden, ecode.Response[any]{Code: ecode.Forbidden, Message: "无管理员权限", Data: nil})
			c.Abort()
			return
		}
		c.Next()
	}
}

func writeUnauthorized(c *gin.Context) {
	c.JSON(http.StatusUnauthorized, ecode.Response[any]{Code: ecode.Unauthorized, Message: "未授权", Data: nil})
	c.Abort()
}

func redirectToLogin(c *gin.Context) {
	currentURL := c.Request.URL.String()
	loginURL := "/login?redirect=" + url.QueryEscape(currentURL)
	c.Redirect(http.StatusFound, loginURL)
}
