package auth

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	serviceauth "sso-server/service/auth"
)

// WriteLoginCookies writes the browser authorization session and refresh token.
func WriteLoginCookies(c *gin.Context, pair *serviceauth.TokenPair, secure bool, refreshTTL time.Duration) {
	writeSessionCookie(c, pair.SessionID, secure)
	writeRefreshCookie(c, pair.RefreshToken, secure, refreshTTL)
}

// ClearLoginCookies removes both browser credentials associated with a login.
func ClearLoginCookies(c *gin.Context, secure bool) {
	clearSessionCookie(c, secure)
	clearRefreshCookie(c, secure)
}

func writeRefreshCookie(c *gin.Context, token string, secure bool, ttl time.Duration) {
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

func clearRefreshCookie(c *gin.Context, secure bool) {
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

func writeSessionCookie(c *gin.Context, sessionID string, secure bool) {
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

func clearSessionCookie(c *gin.Context, secure bool) {
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
