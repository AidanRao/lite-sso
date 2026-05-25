package oauth

import (
	"errors"
	"log"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/common/ecode"
	"sso-server/conf"
	"sso-server/dal/db"
	"sso-server/dal/kv"
	apiauth "sso-server/handler/api/auth"
	"sso-server/handler/oauth2"
	serviceauth "sso-server/service/auth"
	"sso-server/service/oauth"
)

type OAuthDeps struct {
	Config *conf.Config
	DB     *gorm.DB
	KV     kv.Store
	OAuth2 *oauth2.OAuth2
}

type OAuthHandler struct {
	oauthService *oauth.OAuthService
	authService  *serviceauth.AuthService
	oauth2       *oauth2.OAuth2
	db           *gorm.DB
}

func NewOAuthHandler(deps OAuthDeps) *OAuthHandler {
	userRepo := db.NewUserRepository(deps.DB)
	return &OAuthHandler{
		oauthService: oauth.NewOAuthService(deps.Config, deps.DB, deps.KV, userRepo),
		authService:  serviceauth.NewAuthService(deps.Config, deps.DB, deps.KV, nil, deps.OAuth2),
		oauth2:       deps.OAuth2,
		db:           deps.DB,
	}
}

// HandleUserinfo returns user information from OAuth2 token
// This replaces the direct DAL access in the oauth2 handler
func (h *OAuthHandler) HandleUserinfo(c *gin.Context) {
	ti, err := h.oauth2.ValidateToken(c.Request)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_token"})
		return
	}

	userInfo, err := h.oauthService.GetUserInfo(c.Request.Context(), ti.GetUserID())
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}

	c.JSON(http.StatusOK, userInfo)
}

func (h *OAuthHandler) ClientInfo(c *gin.Context) {
	clientID := c.Query("client_id")
	if clientID == "" {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误", Data: nil})
		return
	}

	client, err := db.NewOAuthClientRepository(h.db).FindByClientID(c.Request.Context(), clientID)
	if err != nil {
		c.JSON(http.StatusNotFound, ecode.Response[any]{Code: ecode.NotFound, Message: "应用不存在", Data: nil})
		return
	}

	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{
		"client_id": client.ClientID,
		"name":      client.Name,
	}))
}

// ThirdPartyLogin initiates third-party OAuth login
func (h *OAuthHandler) ThirdPartyLogin(c *gin.Context) {
	provider := c.Param("provider")
	if provider == "" {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误", Data: nil})
		return
	}

	redirectURL, err := h.oauthService.HandleThirdPartyLogin(c.Request.Context(), provider, c.Query("redirect"))
	if err != nil {
		switch {
		case errors.Is(err, common.ErrInvalidProvider):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "不支持的第三方平台", Data: nil})
		case errors.Is(err, common.ErrInvalidRedirect):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "跳转地址无效", Data: nil})
		default:
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "登录失败", Data: nil})
		}
		return
	}

	c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

func (h *OAuthHandler) ThirdPartyBind(c *gin.Context) {
	provider := c.Param("provider")
	userID := c.GetString("user_id")
	if provider == "" || userID == "" {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误", Data: nil})
		return
	}

	redirectURL, err := h.oauthService.HandleThirdPartyBind(c.Request.Context(), userID, provider, c.Query("redirect"))
	if err != nil {
		switch {
		case errors.Is(err, common.ErrInvalidProvider):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "不支持的第三方平台", Data: nil})
		case errors.Is(err, common.ErrInvalidRedirect):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "跳转地址无效", Data: nil})
		case errors.Is(err, common.ErrUserNotFound):
			c.JSON(http.StatusUnauthorized, ecode.Response[any]{Code: ecode.Unauthorized, Message: "未授权", Data: nil})
		default:
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "绑定失败", Data: nil})
		}
		return
	}

	c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

// ThirdPartyCallback handles callback from third-party OAuth
func (h *OAuthHandler) ThirdPartyCallback(c *gin.Context) {
	provider := c.Param("provider")
	code := c.Query("code")
	state := c.Query("state")

	if provider == "" || code == "" {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误", Data: nil})
		return
	}

	result, err := h.oauthService.HandleThirdPartyCallbackWithState(c.Request.Context(), provider, code, state)
	if err != nil {
		switch {
		case errors.Is(err, common.ErrThirdPartyAlreadyBound):
			log.Printf("ThirdPartyCallback: third party already bound, provider=%s, has_state=%t, err=%v", provider, state != "", err)
			c.Redirect(http.StatusTemporaryRedirect, "/profile?bind_error="+url.QueryEscape("该账号已绑定此第三方登录方式"))
		case errors.Is(err, common.ErrThirdPartyBoundToAnother):
			log.Printf("ThirdPartyCallback: third party bound to another user, provider=%s, has_state=%t, err=%v", provider, state != "", err)
			c.Redirect(http.StatusTemporaryRedirect, "/profile?bind_error="+url.QueryEscape("该第三方账号已被其他账号绑定"))
		case errors.Is(err, common.ErrProviderAuthFailed):
			log.Printf("ThirdPartyCallback: provider authentication failed, provider=%s, has_state=%t, err=%v", provider, state != "", err)
			c.Redirect(http.StatusTemporaryRedirect, "/oauth/callback?error="+url.QueryEscape("第三方认证失败"))
		case errors.Is(err, common.ErrInvalidProvider):
			log.Printf("ThirdPartyCallback: invalid provider, provider=%s, err=%v", provider, err)
			c.Redirect(http.StatusTemporaryRedirect, "/oauth/callback?error="+url.QueryEscape("不支持的第三方平台"))
		default:
			log.Printf("ThirdPartyCallback: login failed, provider=%s, has_state=%t, err=%v", provider, state != "", err)
			c.Redirect(http.StatusTemporaryRedirect, "/oauth/callback?error="+url.QueryEscape("登录失败"))
		}
		return
	}

	if result.Action == oauth.ThirdPartyActionBind {
		redirectURL := result.Redirect
		if redirectURL == "" {
			redirectURL = "/profile"
		}
		c.Redirect(http.StatusTemporaryRedirect, redirectURL)
		return
	}

	_, sessionID, err := h.authService.CompleteLogin(c.Request.Context(), result.User.ID, result.Redirect)
	if err != nil {
		log.Printf("ThirdPartyCallback: complete login failed, provider=%s, user_id=%s, err=%v", provider, result.User.ID, err)
		c.Redirect(http.StatusTemporaryRedirect, "/oauth/callback?error="+url.QueryEscape("登录失败"))
		return
	}

	apiauth.WriteSessionCookie(c, sessionID, conf.GetEnv() == conf.EnvProd)
	c.Redirect(http.StatusTemporaryRedirect, result.Redirect)
}
