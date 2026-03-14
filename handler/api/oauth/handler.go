package oauth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/common/ecode"
	"sso-server/conf"
	"sso-server/dal/db"
	"sso-server/dal/kv"
	"sso-server/handler/oauth2"
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
	oauth2       *oauth2.OAuth2
	cfg          *conf.Config
}

func NewOAuthHandler(deps OAuthDeps) *OAuthHandler {
	userRepo := db.NewUserRepository(deps.DB)
	return &OAuthHandler{
		oauthService: oauth.NewOAuthService(deps.Config, deps.DB, deps.KV, deps.OAuth2, userRepo),
		oauth2:       deps.OAuth2,
		cfg:          deps.Config,
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

// ThirdPartyLogin initiates third-party OAuth login
func (h *OAuthHandler) ThirdPartyLogin(c *gin.Context) {
	provider := c.Param("provider")
	if provider == "" {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误", Data: nil})
		return
	}

	redirectURL, err := h.oauthService.HandleThirdPartyLogin(c.Request.Context(), provider)
	if err != nil {
		switch err {
		case common.ErrInvalidProvider:
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "不支持的第三方平台", Data: nil})
		default:
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "登录失败", Data: nil})
		}
		return
	}

	c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

// ThirdPartyCallback handles callback from third-party OAuth
func (h *OAuthHandler) ThirdPartyCallback(c *gin.Context) {
	provider := c.Param("provider")
	code := c.Query("code")

	if provider == "" || code == "" {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误", Data: nil})
		return
	}

	user, err := h.oauthService.HandleThirdPartyCallback(c.Request.Context(), provider, code)
	if err != nil {
		switch err {
		case common.ErrProviderAuthFailed:
			c.JSON(http.StatusUnauthorized, ecode.Response[any]{Code: ecode.Unauthorized, Message: "第三方认证失败", Data: nil})
		default:
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "登录失败", Data: nil})
		}
		return
	}

	// TODO: Issue token for the user
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"user": user}))
}

// BindThirdPartyAccount binds a third-party account to current user
func (h *OAuthHandler) BindThirdPartyAccount(c *gin.Context) {
	var req struct {
		Provider string `json:"provider" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误", Data: nil})
		return
	}

	// TODO: Get user ID from context (from auth middleware)
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, ecode.Response[any]{Code: ecode.Unauthorized, Message: "未授权", Data: nil})
		return
	}

	err := h.oauthService.BindThirdPartyAccount(c.Request.Context(), userID, req.Provider)
	if err != nil {
		switch {
		case errors.Is(err, common.ErrInvalidProvider):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "不支持的第三方平台", Data: nil})
		case errors.Is(err, common.ErrBindingExists):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "已绑定该平台账号", Data: nil})
		default:
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "绑定失败", Data: nil})
		}
		return
	}

	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{
		"bound": true,
	}))
}
