package auth

import (
	"embed"
	"encoding/json"
	"html/template"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"

	"sso-server/common/ecode"
	"sso-server/conf"
	"sso-server/dal/db"
	"sso-server/handler/oauth2"
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

	logoutURIs := h.getLogoutURIs(c)
	redirectURI := c.Query("redirect")

	allowedRedirectURIs := h.getAllowedRedirectURIs(c)
	if redirectURI != "" && oauth2.ValidateRedirectURI(allowedRedirectURIs, redirectURI) != nil {
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

func (h *AuthHandler) getLogoutURIs(c *gin.Context) []string {
	if h.db == nil {
		return nil
	}

	clientRepo := db.NewOAuthClientRepository(h.db)
	clients, err := clientRepo.FindAll(c.Request.Context())
	if err != nil {
		return nil
	}

	var uris []string
	for _, client := range clients {
		if client.LogoutURIs == "" {
			continue
		}
		var clientUris []string
		if err := json.Unmarshal([]byte(client.LogoutURIs), &clientUris); err != nil {
			continue
		}
		uris = append(uris, clientUris...)
	}
	return uris
}

func (h *AuthHandler) getAllowedRedirectURIs(c *gin.Context) string {
	if h.db == nil {
		return ""
	}

	clientRepo := db.NewOAuthClientRepository(h.db)
	clients, err := clientRepo.FindAll(c.Request.Context())
	if err != nil {
		return ""
	}

	var uris []string
	for _, client := range clients {
		if client.RedirectURIs == "" {
			continue
		}
		var clientUris []string
		if err := json.Unmarshal([]byte(client.RedirectURIs), &clientUris); err != nil {
			continue
		}
		uris = append(uris, clientUris...)
	}
	if len(uris) == 0 {
		return ""
	}
	result, _ := json.Marshal(uris)
	return string(result)
}
