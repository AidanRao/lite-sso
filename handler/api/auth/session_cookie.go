package auth

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	serviceauth "sso-server/service/auth"
)

// WriteLoginCookies writes browser credentials according to the persisted session policy.
func WriteLoginCookies(c *gin.Context, pair *serviceauth.TokenPair, secure bool) {
	remaining := time.Until(pair.SessionExpiresAt)
	if remaining <= 0 {
		ClearLoginCookies(c, secure)
		return
	}
	maxAge := 0
	var expires time.Time
	if pair.RememberMe {
		maxAge = int(remaining / time.Second)
		// MaxAge zero means a session cookie in net/http, so expire sub-second sessions immediately.
		if maxAge < 1 {
			ClearLoginCookies(c, secure)
			return
		}
		expires = pair.SessionExpiresAt.UTC()
	}
	for _, credential := range []struct {
		name  string
		value string
		path  string
	}{
		{name: serviceauth.SessionCookieName, value: pair.SessionID, path: "/"},
		{name: serviceauth.RefreshTokenCookieName, value: pair.RefreshToken, path: "/api/auth"},
	} {
		http.SetCookie(c.Writer, &http.Cookie{Name: credential.name, Value: credential.value, Path: credential.path,
			HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: secure, MaxAge: maxAge, Expires: expires})
	}
}

// ClearLoginCookies removes both browser credentials associated with a login.
func ClearLoginCookies(c *gin.Context, secure bool) {
	clearSessionCookie(c, secure)
	clearRefreshCookie(c, secure)
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
