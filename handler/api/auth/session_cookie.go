package auth

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	serviceauth "sso-server/service/auth"
)

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
