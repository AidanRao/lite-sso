package user

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	apiauth "sso-server/handler/api/auth"

	"sso-server/common"
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
}

func NewUserHandler(deps UserDeps) *UserHandler {
	return &UserHandler{
		user: serviceuser.NewUserService(deps.Config, deps.DB, deps.KV, deps.OAuth2),
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
		switch {
		case errors.Is(err, common.ErrInvalidOTP):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "验证码错误", Data: nil})
		case errors.Is(err, common.ErrEmailExists):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "邮箱已存在", Data: nil})
		case errors.Is(err, common.ErrUsernameExists):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "用户名已存在", Data: nil})
		default:
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "注册失败", Data: nil})
		}
		return
	}

	sessionID, err := h.user.CreateSession(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "注册失败", Data: nil})
		return
	}
	apiauth.WriteSessionCookie(c, sessionID, conf.GetEnv() == conf.EnvProd)

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

func (h *UserHandler) ResetPassword(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
		OTP      string `json:"otp" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误", Data: nil})
		return
	}
	if len(strings.TrimSpace(req.Password)) < 8 {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "密码长度至少8位", Data: nil})
		return
	}

	err := h.user.ResetPasswordWithEmailOTP(c.Request.Context(), req.Email, req.Password, req.OTP)
	if err != nil {
		switch {
		case errors.Is(err, common.ErrInvalidOTP):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "验证码错误", Data: nil})
		case errors.Is(err, common.ErrUserNotFound):
			c.JSON(http.StatusNotFound, ecode.Response[any]{Code: ecode.NotFound, Message: "用户不存在", Data: nil})
		default:
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "重置失败", Data: nil})
		}
		return
	}

	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"reset": true}))
}

// GetProfile retrieves user profile
func (h *UserHandler) GetProfile(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, ecode.Response[any]{Code: ecode.Unauthorized, Message: "未授权", Data: nil})
		return
	}

	profile, err := h.user.GetProfileOverview(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, common.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, ecode.Response[any]{Code: ecode.NotFound, Message: "用户不存在", Data: nil})
			return
		}
		c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "获取资料失败", Data: nil})
		return
	}

	c.JSON(http.StatusOK, ecode.OKResponse(profile))
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

	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, ecode.Response[any]{Code: ecode.Unauthorized, Message: "未授权", Data: nil})
		return
	}

	user, err := h.user.UpdateProfile(c.Request.Context(), userID, req.Username, req.AvatarURL)
	if err != nil {
		switch {
		case errors.Is(err, common.ErrUserNotFound):
			c.JSON(http.StatusNotFound, ecode.Response[any]{Code: ecode.NotFound, Message: "用户不存在", Data: nil})
		case errors.Is(err, common.ErrUsernameExists):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "用户名已存在", Data: nil})
		default:
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "更新失败", Data: nil})
		}
		return
	}

	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"user": user}))
}
