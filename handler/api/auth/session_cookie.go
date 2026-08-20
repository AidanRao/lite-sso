package auth

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	serviceauth "sso-server/service/auth"
)

func WriteRefreshCookie(c *gin.Context, token string, secure bool, ttl time.Duration) {
	if ttl <= 0 {
		ttl = 30 * 24 * time.Hour
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     serviceauth.RefreshTokenCookieName,
		Value:    token,
		Path:     "/api/auth",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   int(ttl.Seconds()),
	})
}

func ClearRefreshCookie(c *gin.Context, secure bool) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     serviceauth.RefreshTokenCookieName,
		Value:    "",
		Path:     "/api/auth",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

func WriteSessionCookie(c *gin.Context, sessionID string, secure bool) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     serviceauth.SessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   int(serviceauth.SessionTTL.Seconds()),
	})
}

func ClearSessionCookie(c *gin.Context, secure bool) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     serviceauth.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}
