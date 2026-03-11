package user

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"sso-server/common/ecode"
	"sso-server/conf"
	"sso-server/dal/kv"
	"sso-server/handler/oauth2"
	serviceuser "sso-server/service/user"
)

type UserDeps struct {
	Config *conf.Config
	DB     *gorm.DB
	KV     kv.Store
	OAuth2 *oauth2.OAuth2
}

type UserHandler struct {
	user *serviceuser.UserService
	cfg  *conf.Config
}

func NewUserHandler(deps UserDeps) *UserHandler {
	return &UserHandler{
		user: serviceuser.NewUserService(deps.Config, deps.DB, deps.KV, deps.OAuth2),
		cfg:  deps.Config,
	}
}

// Register handles user registration
func (h *UserHandler) Register(c *gin.Context) {
	var req struct {
		Email    string  `json:"email" binding:"required,email"`
		Password string  `json:"password" binding:"required"`
		Username *string `json:"username"`
		OTP      string  `json:"otp" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误", Data: nil})
		return
	}
	if len(strings.TrimSpace(req.Password)) < 8 {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "密码长度至少8位", Data: nil})
		return
	}

	user, tokenData, err := h.user.RegisterWithEmailOTP(c.Request.Context(), c.Request, req.Email, req.Password, req.Username, req.OTP)
	if err != nil {
		switch err {
		case serviceuser.ErrInvalidOTP:
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "验证码错误", Data: nil})
		case serviceuser.ErrEmailExists:
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "邮箱已存在", Data: nil})
		case serviceuser.ErrUsernameExists:
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "用户名已存在", Data: nil})
		default:
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "注册失败", Data: nil})
		}
		return
	}

	data := gin.H{"user": user}
	if tokenData != nil {
		if v, ok := tokenData["access_token"]; ok {
			data["access_token"] = v
		}
		if v, ok := tokenData["token_type"]; ok {
			data["token_type"] = v
		}
		if v, ok := tokenData["expires_in"]; ok {
			data["expires_in"] = v
		}
	}

	c.JSON(http.StatusOK, ecode.OKResponse(data))
}

// GetProfile retrieves user profile
func (h *UserHandler) GetProfile(c *gin.Context) {
	// TODO: Get user ID from context (from auth middleware)
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, ecode.Response[any]{Code: ecode.Unauthorized, Message: "未授权", Data: nil})
		return
	}

	user, err := h.user.GetProfile(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, ecode.Response[any]{Code: ecode.NotFound, Message: "用户不存在", Data: nil})
		return
	}

	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"user": user}))
}

// UpdateProfile updates user profile
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	var req struct {
		Username  *string `json:"username"`
		AvatarURL *string `json:"avatar_url"`
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

	user, err := h.user.UpdateProfile(c.Request.Context(), userID, req.Username, req.AvatarURL)
	if err != nil {
		switch err {
		case serviceuser.ErrUserNotFound:
			c.JSON(http.StatusNotFound, ecode.Response[any]{Code: ecode.NotFound, Message: "用户不存在", Data: nil})
		default:
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "更新失败", Data: nil})
		}
		return
	}

	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"user": user}))
}
