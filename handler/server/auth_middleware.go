package server

import (
	"context"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"

	"sso-server/common/ecode"
	"sso-server/dal/kv"
	serviceauth "sso-server/service/auth"
)

func RequireSessionAuth(kvStore kv.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID, err := c.Cookie(serviceauth.SessionCookieName)
		if err != nil || sessionID == "" {
			c.JSON(http.StatusUnauthorized, ecode.Response[any]{Code: ecode.Unauthorized, Message: "未授权", Data: nil})
			c.Abort()
			return
		}

		userID, err := kvStore.Get(c.Request.Context(), kv.KeySession(sessionID))
		if err != nil || userID == "" {
			c.JSON(http.StatusUnauthorized, ecode.Response[any]{Code: ecode.Unauthorized, Message: "未授权", Data: nil})
			c.Abort()
			return
		}

		ctx := context.WithValue(c.Request.Context(), "user_id", userID)
		c.Request = c.Request.WithContext(ctx)
		c.Set("user_id", userID)
		c.Next()
	}
}

func RequireSessionAuthOrRedirect(kvStore kv.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID, err := c.Cookie(serviceauth.SessionCookieName)
		if err != nil || sessionID == "" {
			redirectToLogin(c)
			c.Abort()
			return
		}

		userID, err := kvStore.Get(c.Request.Context(), kv.KeySession(sessionID))
		if err != nil || userID == "" {
			redirectToLogin(c)
			c.Abort()
			return
		}

		ctx := context.WithValue(c.Request.Context(), "user_id", userID)
		c.Request = c.Request.WithContext(ctx)
		c.Set("user_id", userID)
		c.Next()
	}
}

func redirectToLogin(c *gin.Context) {
	currentURL := c.Request.URL.String()
	loginURL := "/?redirect=" + url.QueryEscape(currentURL)
	c.Redirect(http.StatusFound, loginURL)
}
