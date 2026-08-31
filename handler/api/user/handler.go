package user

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	apiauth "sso-server/handler/api/auth"

	"sso-server/common"
	"sso-server/common/ecode"
	"sso-server/conf"
	"sso-server/dal/kv"
	"sso-server/handler/audit"
	"sso-server/handler/oauth2"
	manageross "sso-server/manager/oss"
	serviceauth "sso-server/service/auth"
	serviceuser "sso-server/service/user"
)

type UserDeps struct {
	Config        *conf.Config
	DB            *gorm.DB
	KV            kv.Store
	OAuth2        *oauth2.OAuth2
	ImageStore    manageross.ImageStore
	MessageSender serviceauth.MessageSender
}

type UserHandler struct {
	user              *serviceuser.UserService
	auth              *serviceauth.AuthService
	emails            *serviceuser.EmailService
	trustProxyHeaders bool
}

func NewUserHandler(deps UserDeps) *UserHandler {
	trustProxyHeaders := deps.Config != nil && deps.Config.Server.TrustProxyHeaders
	return &UserHandler{
		user:              serviceuser.NewUserService(deps.Config, deps.DB, deps.KV, deps.OAuth2, deps.ImageStore),
		auth:              serviceauth.NewAuthService(deps.Config, deps.DB, deps.KV, nil, deps.OAuth2),
		emails:            serviceuser.NewEmailService(serviceuser.EmailDeps{Config: deps.Config, DB: deps.DB, MessageSender: deps.MessageSender}),
		trustProxyHeaders: trustProxyHeaders,
	}
}

const (
	maxAvatarFileSize      int64 = 2 * 1024 * 1024
	maxAvatarMultipartSize       = maxAvatarFileSize + 64*1024
)

func passwordPolicyMessage(err error) string {
	switch {
	case errors.Is(err, common.ErrPasswordLengthInvalid):
		return "需为10至256位"
	case errors.Is(err, common.ErrPasswordLetterRequired):
		return "需包含英文字符"
	case errors.Is(err, common.ErrPasswordDigitRequired):
		return "需包含数字"
	default:
		return ""
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
	audit.Email(c, req.Email)
	deviceID, isNewDevice := serviceauth.EnsureDeviceID(c.Request)
	audit.Device(c, deviceID)
	user, err := h.user.RegisterWithEmailChallenge(c.Request.Context(), req.Email, req.Password, req.Username, req.ChallengeID, req.Code, deviceID)
	if err != nil {
		audit.Error(c, err)
		switch {
		case errors.Is(err, common.ErrInvalidOTP):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "验证码错误", Data: nil})
		case errors.Is(err, common.ErrEmailExists):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "邮箱已存在", Data: nil})
		case errors.Is(err, common.ErrUsernameExists):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "用户名已存在", Data: nil})
		case errors.Is(err, common.ErrOTPExpired), errors.Is(err, common.ErrOTPAttemptsExceeded):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "验证码无效或已过期", Data: nil})
		case passwordPolicyMessage(err) != "":
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: passwordPolicyMessage(err), Data: nil})
		default:
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "注册失败", Data: nil})
		}
		return
	}

	audit.Actor(c, user.ID, "")
	audit.Completed(c, "user_created")
	result, pair, err := h.auth.CompleteLoginWithContext(c.Request.Context(), user.ID, "", serviceauth.LoginMetadata{
		DeviceID:  deviceID,
		IP:        serviceauth.RequestIP(c.Request, h.trustProxyHeaders),
		UserAgent: c.Request.UserAgent(),
	}, serviceauth.AuthMethodPassword)
	if err != nil {
		audit.Error(c, err)
		c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "注册失败", Data: nil})
		return
	}
	if isNewDevice {
		apiauth.WriteDeviceCookie(c, deviceID)
	}
	audit.Actor(c, user.ID, pair.SessionID)
	audit.AuthMethod(c, "password")
	audit.Completed(c, "session_created")
	apiauth.WriteLoginCookies(c, pair, conf.GetEnv() == conf.EnvProd, h.auth.RefreshTokenTTL())
	audit.Success(c)
	c.JSON(http.StatusOK, ecode.OKResponse(result))
}

// ChangePassword updates the current user's configured password after verifying the old password.
func (h *UserHandler) ChangePassword(c *gin.Context) {
	userID := c.GetString("user_id")
	sessionID := c.GetString("session_id")
	if userID == "" || sessionID == "" {
		c.JSON(http.StatusUnauthorized, ecode.Response[any]{Code: ecode.Unauthorized, Message: "未授权", Data: nil})
		return
	}

	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误", Data: nil})
		return
	}
	err := h.auth.ChangePassword(c.Request.Context(), userID, sessionID, req.OldPassword, req.NewPassword)
	if err != nil {
		audit.Error(c, err)
		switch {
		case errors.Is(err, common.ErrCurrentPasswordInvalid):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "旧密码错误", Data: nil})
		case errors.Is(err, common.ErrPasswordNotSet):
			c.JSON(http.StatusConflict, ecode.Response[any]{Code: ecode.Conflict, Message: "当前账号尚未设置密码", Data: nil})
		case passwordPolicyMessage(err) != "":
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: passwordPolicyMessage(err), Data: nil})
		case errors.Is(err, common.ErrUserNotFound):
			c.JSON(http.StatusNotFound, ecode.Response[any]{Code: ecode.NotFound, Message: "用户不存在", Data: nil})
		case errors.Is(err, common.ErrSessionRevoked):
			c.JSON(http.StatusUnauthorized, ecode.Response[any]{Code: ecode.Unauthorized, Message: "未授权", Data: nil})
		default:
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "修改密码失败", Data: nil})
		}
		return
	}

	audit.Changed(c, "password")
	audit.Completed(c, "password_updated", "sessions_revoked")
	audit.Success(c)
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"updated": true}))
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
	audit.Email(c, req.Email)
	deviceID, isNewDevice := serviceauth.EnsureDeviceID(c.Request)
	audit.Device(c, deviceID)
	result, err := h.user.ResetPasswordWithEmailChallenge(c.Request.Context(), req.Email, req.Password, req.ChallengeID, req.Code, deviceID)
	if result != nil {
		audit.Actor(c, result.UserID, "")
		if result.PasswordUpdated {
			audit.Changed(c, "password")
			audit.Completed(c, "password_updated")
		}
		if result.SessionsRevoked {
			audit.Completed(c, "sessions_revoked")
		}
	}
	if err != nil {
		audit.Error(c, err)
		switch {
		case errors.Is(err, common.ErrInvalidOTP), errors.Is(err, common.ErrChallengeInvalid), errors.Is(err, common.ErrOTPExpired), errors.Is(err, common.ErrOTPAttemptsExceeded):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "验证码错误", Data: nil})
		case errors.Is(err, common.ErrUserNotFound):
			c.JSON(http.StatusNotFound, ecode.Response[any]{Code: ecode.NotFound, Message: "用户不存在", Data: nil})
		case passwordPolicyMessage(err) != "":
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: passwordPolicyMessage(err), Data: nil})
		default:
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "重置失败", Data: nil})
		}
		return
	}

	if isNewDevice {
		apiauth.WriteDeviceCookie(c, deviceID)
	}
	audit.Success(c)
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"reset": true}))
}

// GetProfile retrieves user profile
func (h *UserHandler) GetProfile(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, ecode.Response[any]{Code: ecode.Unauthorized, Message: "未授权", Data: nil})
		return
	}

	profile, err := h.user.GetProfile(c.Request.Context(), userID)
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

// GetLoginMethods returns system-supported sign-in methods and the current user's availability.
func (h *UserHandler) GetLoginMethods(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, ecode.Response[any]{Code: ecode.Unauthorized, Message: "未授权", Data: nil})
		return
	}

	methods, err := h.user.GetLoginMethods(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, common.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, ecode.Response[any]{Code: ecode.NotFound, Message: "用户不存在", Data: nil})
			return
		}
		c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "获取登录方式失败", Data: nil})
		return
	}

	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"methods": methods}))
}

// GetApplications returns OAuth applications used by the current user.
func (h *UserHandler) GetApplications(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, ecode.Response[any]{Code: ecode.Unauthorized, Message: "未授权", Data: nil})
		return
	}

	applications, err := h.user.GetApplications(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, common.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, ecode.Response[any]{Code: ecode.NotFound, Message: "用户不存在", Data: nil})
			return
		}
		c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "获取应用失败", Data: nil})
		return
	}

	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"applications": applications}))
}

// GetLoginDevices lists the current user's active browser devices.
func (h *UserHandler) GetLoginDevices(c *gin.Context) {
	userID := c.GetString("user_id")
	sessionID := c.GetString("session_id")
	if userID == "" || sessionID == "" {
		c.JSON(http.StatusUnauthorized, ecode.Response[any]{Code: ecode.Unauthorized, Message: "未授权", Data: nil})
		return
	}

	devices, err := h.auth.ListLoginDevices(c.Request.Context(), userID, sessionID)
	if err != nil {
		if errors.Is(err, common.ErrSessionRevoked) {
			c.JSON(http.StatusUnauthorized, ecode.Response[any]{Code: ecode.Unauthorized, Message: "未授权", Data: nil})
			return
		}
		c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "获取登录设备失败", Data: nil})
		return
	}

	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"devices": devices}))
}

// RevokeLoginDevice revokes all active sessions for another browser device.
func (h *UserHandler) RevokeLoginDevice(c *gin.Context) {
	userID := c.GetString("user_id")
	sessionID := c.GetString("session_id")
	if userID == "" || sessionID == "" {
		c.JSON(http.StatusUnauthorized, ecode.Response[any]{Code: ecode.Unauthorized, Message: "未授权", Data: nil})
		return
	}

	err := h.auth.RevokeLoginDevice(c.Request.Context(), userID, sessionID, c.Param("device_id"))
	if err != nil {
		audit.Error(c, err)
		switch {
		case errors.Is(err, common.ErrCurrentDevice):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "当前设备请使用退出登录", Data: nil})
		case errors.Is(err, common.ErrDeviceNotFound):
			c.JSON(http.StatusNotFound, ecode.Response[any]{Code: ecode.NotFound, Message: "登录设备不存在", Data: nil})
		case errors.Is(err, common.ErrSessionRevoked):
			c.JSON(http.StatusUnauthorized, ecode.Response[any]{Code: ecode.Unauthorized, Message: "未授权", Data: nil})
		default:
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "踢出设备失败", Data: nil})
		}
		return
	}

	audit.Changed(c, "session")
	audit.Success(c)
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"revoked": true}))
}

// UpdateProfile updates user profile
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	var req struct {
		Username *string `json:"username"`
	}
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误", Data: nil})
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, ecode.Response[any]{Code: ecode.Unauthorized, Message: "未授权", Data: nil})
		return
	}

	user, err := h.user.UpdateProfile(c.Request.Context(), userID, req.Username)
	if err != nil {
		audit.Error(c, err)
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

	if req.Username != nil {
		audit.Changed(c, "username")
	}
	audit.Success(c)
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"user": user}))
}

// UploadAvatar uploads an avatar for the current user.
func (h *UserHandler) UploadAvatar(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, ecode.Response[any]{Code: ecode.Unauthorized, Message: "未授权", Data: nil})
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxAvatarMultipartSize)
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		writeAvatarUploadFormError(c, err)
		return
	}
	defer file.Close()

	if header.Size > maxAvatarFileSize {
		writeAvatarTooLarge(c)
		return
	}

	contentType, extension, err := manageross.ValidateImage(file)
	if err != nil {
		audit.Error(c, err)
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "仅支持 JPEG、PNG 或 WebP 图片", Data: nil})
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "头像上传失败", Data: nil})
		return
	}

	user, err := h.user.UploadAvatar(c.Request.Context(), userID, contentType, extension, file, header.Size)
	if err != nil {
		audit.Error(c, err)
		switch {
		case errors.Is(err, common.ErrAvatarStorageUnavailable):
			c.JSON(http.StatusServiceUnavailable, ecode.Response[any]{Code: ecode.ServiceUnavailable, Message: "头像服务暂未配置", Data: nil})
		case errors.Is(err, common.ErrUserNotFound):
			c.JSON(http.StatusNotFound, ecode.Response[any]{Code: ecode.NotFound, Message: "用户不存在", Data: nil})
		default:
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "头像上传失败", Data: nil})
		}
		return
	}

	audit.Changed(c, "avatar")
	audit.Success(c)
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"user": user}))
}

func writeAvatarUploadFormError(c *gin.Context, err error) {
	audit.Error(c, err)
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		writeAvatarTooLarge(c)
		return
	}
	c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "请选择头像图片", Data: nil})
}

func writeAvatarTooLarge(c *gin.Context) {
	c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "头像大小不能超过 2MB", Data: nil})
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
		audit.Error(c, err)
		switch {
		case errors.Is(err, common.ErrInvalidProvider):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "不支持的第三方平台", Data: nil})
		case errors.Is(err, common.ErrLastLoginMethod):
			c.JSON(http.StatusConflict, ecode.Response[any]{Code: ecode.Conflict, Message: "至少需要保留一种可用登录方式", Data: nil})
		case errors.Is(err, common.ErrThirdPartyNotBound):
			c.JSON(http.StatusNotFound, ecode.Response[any]{Code: ecode.NotFound, Message: "尚未绑定该第三方平台", Data: nil})
		case errors.Is(err, common.ErrUserNotFound):
			c.JSON(http.StatusNotFound, ecode.Response[any]{Code: ecode.NotFound, Message: "用户不存在", Data: nil})
		default:
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "撤销授权失败", Data: nil})
		}
		return
	}

	audit.Changed(c, "provider")
	audit.Success(c)
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"unbound": true}))
}
