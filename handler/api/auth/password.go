package auth

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"sso-server/common"
	"sso-server/common/ecode"
	"sso-server/conf"
	"sso-server/handler/audit"
	serviceauth "sso-server/service/auth"
)

// LoginWithPassword handles password-based login.
func (h *AuthHandler) LoginWithPassword(c *gin.Context) {
	var req struct {
		Email     string `json:"email" binding:"required,email"`
		Password  string `json:"password" binding:"required"`
		Redirect  string `json:"redirect"`
		CaptchaID string `json:"captcha_id"`
		Captcha   string `json:"captcha"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误", Data: nil})
		return
	}

	audit.Email(c, req.Email)
	deviceID, isNewDevice := serviceauth.EnsureDeviceID(c.Request)
	captchaValid := false
	if req.CaptchaID != "" || req.Captcha != "" {
		var err error
		captchaValid, err = h.auth.VerifyCaptcha(c.Request.Context(), req.CaptchaID, req.Captcha)
		if err != nil || !captchaValid {
			writeAuthError(c, common.ErrInvalidCaptcha)
			return
		}
	}
	loginContext := serviceauth.PasswordLoginContext{
		DeviceID:     deviceID,
		IP:           serviceauth.RequestIP(c.Request, h.trustProxyHeaders),
		CaptchaValid: captchaValid,
	}
	user, err := h.auth.LoginWithPasswordContext(c.Request.Context(), req.Email, req.Password, loginContext)
	if errors.Is(err, common.ErrCaptchaRequired) && !captchaValid {
		c.JSON(http.StatusTooManyRequests, ecode.Response[any]{Code: ecode.TooManyRequests, Message: "请完成验证码后再登录", Data: gin.H{"code": "CAPTCHA_REQUIRED"}})
		return
	}
	if err != nil {
		writeAuthError(c, err)
		return
	}

	audit.Actor(c, user.ID, "")
	result, pair, err := h.auth.CompleteLoginWithContext(c.Request.Context(), user.ID, req.Redirect, serviceauth.LoginMetadata{
		DeviceID:  deviceID,
		IP:        serviceauth.RequestIP(c.Request, h.trustProxyHeaders),
		UserAgent: c.Request.UserAgent(),
	}, serviceauth.AuthMethodPassword)
	if err != nil {
		writeAuthError(c, err)
		return
	}
	if isNewDevice {
		WriteDeviceCookie(c, deviceID)
	}
	audit.Actor(c, user.ID, pair.SessionID)
	audit.Device(c, deviceID)
	audit.Completed(c, "session_created")
	WriteLoginCookies(c, pair, conf.GetEnv() == conf.EnvProd, h.auth.RefreshTokenTTL())
	audit.Success(c)
	c.JSON(http.StatusOK, ecode.OKResponse(result))
}

func writeAuthError(c *gin.Context, err error) {
	audit.Error(c, err)
	switch {
	case errors.Is(err, common.ErrInvalidCredentials):
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "邮箱或密码错误", Data: gin.H{"code": "INVALID_CREDENTIALS"}})
	case errors.Is(err, common.ErrInvalidOTP):
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "验证码错误", Data: gin.H{"code": "EMAIL_CODE_INVALID"}})
	case errors.Is(err, common.ErrChallengeInvalid):
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "验证码无效，请重新获取", Data: gin.H{"code": "EMAIL_CODE_INVALID"}})
	case errors.Is(err, common.ErrOTPExpired):
		c.JSON(http.StatusGone, ecode.Response[any]{Code: ecode.BadRequest, Message: "验证码已过期", Data: gin.H{"code": "EMAIL_CODE_EXPIRED"}})
	case errors.Is(err, common.ErrOTPAttemptsExceeded):
		c.JSON(http.StatusTooManyRequests, ecode.Response[any]{Code: ecode.TooManyRequests, Message: "验证码尝试次数过多", Data: gin.H{"code": "EMAIL_CODE_ATTEMPTS_EXCEEDED"}})
	case errors.Is(err, common.ErrInvalidCaptcha):
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "验证码错误", Data: gin.H{"code": "CAPTCHA_INVALID"}})
	case errors.Is(err, common.ErrCaptchaRequired):
		c.Header("Retry-After", "1")
		c.JSON(http.StatusTooManyRequests, ecode.Response[any]{Code: ecode.TooManyRequests, Message: "需要验证码", Data: gin.H{"code": "CAPTCHA_REQUIRED", "retry_after_seconds": 1}})
	case errors.Is(err, common.ErrRateLimited):
		writeRateLimited(c, err, "请求过于频繁")
	case errors.Is(err, common.ErrUserInactive):
		c.JSON(http.StatusForbidden, ecode.Response[any]{Code: ecode.Forbidden, Message: "用户已禁用", Data: nil})
	case errors.Is(err, common.ErrInvalidRedirect):
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "跳转地址无效", Data: nil})
	default:
		c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "登录失败", Data: nil})
	}
}

func writeRetryAfter(c *gin.Context, seconds int) {
	if seconds < 1 {
		seconds = 1
	}
	c.Header("Retry-After", strconv.Itoa(seconds))
}
