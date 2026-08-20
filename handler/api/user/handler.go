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
	serviceauth "sso-server/service/auth"
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
	auth *serviceauth.AuthService
}

func NewUserHandler(deps UserDeps) *UserHandler {
	return &UserHandler{
		user: serviceuser.NewUserService(deps.Config, deps.DB, deps.KV, deps.OAuth2),
		auth: serviceauth.NewAuthService(deps.Config, deps.DB, deps.KV, nil, deps.OAuth2),
	}
}

// Register handles user registration
func (h *UserHandler) Register(c *gin.Context) {
	var req struct {
		Email       string  `json:"email" binding:"required,email"`
		Password    string  `json:"password" binding:"required"`
		Username    *string `json:"username"`
		ChallengeID string  `json:"challenge_id" binding:"required"`
		Code        string  `json:"code" binding:"required,len=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误", Data: nil})
		return
	}
	deviceID, isNewDevice := serviceauth.EnsureDeviceID(c.Request)
	user, err := h.user.RegisterWithEmailChallenge(c.Request.Context(), req.Email, req.Password, req.Username, req.ChallengeID, req.Code, deviceID)
	if err != nil {
		switch {
		case errors.Is(err, common.ErrInvalidOTP):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "验证码错误", Data: nil})
		case errors.Is(err, common.ErrEmailExists):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "邮箱已存在", Data: nil})
		case errors.Is(err, common.ErrUsernameExists):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "用户名已存在", Data: nil})
		case errors.Is(err, common.ErrOTPExpired), errors.Is(err, common.ErrOTPAttemptsExceeded):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "验证码无效或已过期", Data: nil})
		case strings.Contains(err.Error(), "password"):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "密码长度必须为12至256位", Data: nil})
		default:
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "注册失败", Data: nil})
		}
		return
	}

	result, pair, err := h.auth.CompleteLoginWithContext(c.Request.Context(), user.ID, "", serviceauth.LoginMetadata{
		DeviceID:  deviceID,
		IP:        serviceauth.RequestIP(c.Request),
		UserAgent: c.Request.UserAgent(),
	}, serviceauth.AuthMethodPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "注册失败", Data: nil})
		return
	}
	if isNewDevice {
		apiauth.WriteDeviceCookie(c, deviceID)
	}
	apiauth.WriteRefreshCookie(c, pair.RefreshToken, conf.GetEnv() == conf.EnvProd, h.auth.RefreshTokenTTL())
	c.JSON(http.StatusOK, ecode.OKResponse(result))
}

func (h *UserHandler) ResetPassword(c *gin.Context) {
	var req struct {
		Email       string `json:"email" binding:"required,email"`
		Password    string `json:"password" binding:"required"`
		ChallengeID string `json:"challenge_id" binding:"required"`
		Code        string `json:"code" binding:"required,len=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误", Data: nil})
		return
	}
	deviceID, isNewDevice := serviceauth.EnsureDeviceID(c.Request)
	err := h.user.ResetPasswordWithEmailChallenge(c.Request.Context(), req.Email, req.Password, req.ChallengeID, req.Code, deviceID)
	if err != nil {
		switch {
		case errors.Is(err, common.ErrInvalidOTP), errors.Is(err, common.ErrChallengeInvalid), errors.Is(err, common.ErrOTPExpired), errors.Is(err, common.ErrOTPAttemptsExceeded):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "验证码错误", Data: nil})
		case errors.Is(err, common.ErrUserNotFound):
			c.JSON(http.StatusNotFound, ecode.Response[any]{Code: ecode.NotFound, Message: "用户不存在", Data: nil})
		default:
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "重置失败", Data: nil})
		}
		return
	}

	if isNewDevice {
		apiauth.WriteDeviceCookie(c, deviceID)
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

// UnbindThirdParty removes a third-party login method from the current user.
func (h *UserHandler) UnbindThirdParty(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, ecode.Response[any]{Code: ecode.Unauthorized, Message: "未授权", Data: nil})
		return
	}

	err := h.user.UnbindThirdParty(c.Request.Context(), userID, c.Param("provider"))
	if err != nil {
		switch {
		case errors.Is(err, common.ErrInvalidProvider):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "不支持的第三方平台", Data: nil})
		case errors.Is(err, common.ErrEmailRequiredForUnbind):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "请先设置邮箱，再撤销第三方授权", Data: nil})
		case errors.Is(err, common.ErrThirdPartyNotBound):
			c.JSON(http.StatusNotFound, ecode.Response[any]{Code: ecode.NotFound, Message: "尚未绑定该第三方平台", Data: nil})
		case errors.Is(err, common.ErrUserNotFound):
			c.JSON(http.StatusNotFound, ecode.Response[any]{Code: ecode.NotFound, Message: "用户不存在", Data: nil})
		default:
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "撤销授权失败", Data: nil})
		}
		return
	}

	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"unbound": true}))
}
