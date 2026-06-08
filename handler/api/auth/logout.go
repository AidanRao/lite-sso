package auth

import (
	"embed"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"sso-server/common/ecode"
	"sso-server/conf"
	"sso-server/dal/db"
	"sso-server/model"
	serviceauth "sso-server/service/auth"
)

//go:embed templates/logout.html
var logoutTemplateFS embed.FS

var (
	logoutTemplate     *template.Template
	logoutTemplateOnce sync.Once
)

func getLogoutTemplate() *template.Template {
	logoutTemplateOnce.Do(func() {
		logoutTemplate = template.Must(template.ParseFS(logoutTemplateFS, "templates/logout.html"))
	})
	return logoutTemplate
}

func (h *AuthHandler) Logout(c *gin.Context) {
	sessionID, err := c.Cookie(serviceauth.SessionCookieName)
	if err != nil || sessionID == "" {
		c.JSON(http.StatusUnauthorized, ecode.Response[any]{Code: ecode.Unauthorized, Message: "未授权", Data: nil})
		return
	}

	if err := h.auth.InvalidateSession(c.Request.Context(), sessionID); err != nil {
		c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "退出失败", Data: nil})
		return
	}

	ClearSessionCookie(c, conf.GetEnv() == conf.EnvProd)

	clients := h.getLogoutClients(c)
	logoutURIs := getLogoutURIs(clients)
	redirectURI := c.Query("redirect")

	if redirectURI != "" && !isAllowedLogoutRedirect(clients, redirectURI) {
		log.Printf("Logout: invalid redirect, redirect=%q", redirectURI)
		redirectURI = ""
	}

	if len(logoutURIs) == 0 {
		if redirectURI != "" {
			c.Redirect(http.StatusFound, redirectURI)
			return
		}
		c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"logged_out": true}))
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	logoutURIsJSON, _ := json.Marshal(logoutURIs)
	getLogoutTemplate().Execute(c.Writer, gin.H{
		"LogoutURIs":  template.JS(logoutURIsJSON),
		"RedirectURI": redirectURI,
	})
}

func (h *AuthHandler) getLogoutClients(c *gin.Context) []model.OAuthClient {
	if h.db == nil {
		return nil
	}

	userID := c.GetString("user_id")
	clientRepo := db.NewOAuthClientRepository(h.db)
	clients, err := clientRepo.FindByUserID(c.Request.Context(), userID)
	if err != nil {
		return nil
	}
	return clients
}

func getLogoutURIs(clients []model.OAuthClient) []string {
	var uris []string
	for _, client := range clients {
		if client.LogoutURI == "" {
			continue
		}
		uris = append(uris, client.LogoutURI)
	}
	return uris
}

func isAllowedLogoutRedirect(clients []model.OAuthClient, redirectURI string) bool {
	if isRelativeLogoutRedirect(redirectURI) {
		return true
	}

	for _, client := range clients {
		if client.HomepageURL == "" {
			continue
		}
		if isSameHostname(client.HomepageURL, redirectURI) {
			return true
		}
	}
	return false
}

func isRelativeLogoutRedirect(redirectURI string) bool {
	redirect, err := url.Parse(strings.TrimSpace(redirectURI))
	if err != nil {
		return false
	}
	return !redirect.IsAbs() && redirect.Host == "" && strings.HasPrefix(redirect.Path, "/")
}

func isSameHostname(homepageURL string, redirectURI string) bool {
	homepage, err := url.Parse(strings.TrimSpace(homepageURL))
	if err != nil || homepage.Hostname() == "" {
		return false
	}

	redirect, err := url.Parse(strings.TrimSpace(redirectURI))
	if err != nil || redirect.Hostname() == "" {
		return false
	}

	return strings.EqualFold(homepage.Hostname(), redirect.Hostname())
}
