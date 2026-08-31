// Package passkey exposes authenticated Passkey lifecycle APIs.
package passkey

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/common/ecode"
	"sso-server/conf"
	apiauth "sso-server/handler/api/auth"
	"sso-server/handler/audit"
	serviceauth "sso-server/service/auth"
	servicepasskey "sso-server/service/passkey"
)

// Deps contains Passkey handler dependencies.
type Deps struct {
	Config  *conf.Config
	Service *servicepasskey.Service
}

// Handler handles Passkey APIs.
type Handler struct {
	service           *servicepasskey.Service
	trustProxyHeaders bool
}

// NewHandler creates a Passkey lifecycle handler.
func NewHandler(deps Deps) *Handler {
	return &Handler{
		service:           deps.Service,
		trustProxyHeaders: deps.Config != nil && deps.Config.Server.TrustProxyHeaders,
	}
}

// List lists the current user's Passkeys.
func (h *Handler) List(c *gin.Context) {
	credentials, err := h.service.List(c.Request.Context(), c.GetString("user_id"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"passkeys": credentials}))
}

// SendRegistrationEmail sends an OTP to the current user's existing email.
func (h *Handler) SendRegistrationEmail(c *gin.Context) {
	var req struct {
		CaptchaID string `json:"captcha_id" binding:"required"`
		Captcha   string `json:"captcha" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c)
		return
	}
	deviceID, isNewDevice := serviceauth.EnsureDeviceID(c.Request)
	if isNewDevice {
		apiauth.WriteDeviceCookie(c, deviceID)
	}
	result, err := h.service.SendRegistrationEmail(c.Request.Context(), c.GetString("user_id"), req.CaptchaID, req.Captcha, serviceauth.OTPRequestContext{
		DeviceID: deviceID,
		IP:       serviceauth.RequestIP(c.Request, h.trustProxyHeaders),
	})
	if err != nil {
		writeError(c, err)
		return
	}
	audit.Success(c)
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"sent": true, "challenge_id": result.ChallengeID, "expires_in": result.ExpiresIn, "resend_after": result.ResendAfter}))
}

// RegistrationOptions verifies the email OTP and returns WebAuthn creation options.
func (h *Handler) RegistrationOptions(c *gin.Context) {
	var req struct {
		ChallengeID string `json:"challenge_id" binding:"required"`
		Code        string `json:"code" binding:"required,len=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c)
		return
	}
	deviceID, _ := serviceauth.EnsureDeviceID(c.Request)
	result, err := h.service.BeginRegistration(c.Request.Context(), c.GetString("user_id"), c.GetString("session_id"), c.GetHeader("Origin"), c.Request.UserAgent(), req.ChallengeID, req.Code, deviceID)
	if err != nil {
		writeError(c, err)
		return
	}
	audit.Success(c)
	c.JSON(http.StatusOK, ecode.OKResponse(result))
}

// RegistrationVerify completes a registration ceremony.
func (h *Handler) RegistrationVerify(c *gin.Context) {
	var req struct {
		CeremonyID string          `json:"ceremony_id" binding:"required"`
		Response   json.RawMessage `json:"response" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c)
		return
	}
	credential, err := h.service.FinishRegistration(c.Request.Context(), c.GetString("user_id"), c.GetString("session_id"), c.GetHeader("Origin"), req.CeremonyID, req.Response)
	if err != nil {
		writeError(c, err)
		return
	}
	audit.Target(c, "passkey", credential.ID)
	audit.Changed(c, "passkey")
	audit.Success(c)
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"passkey": credential}))
}

// Rename changes a Passkey name.
func (h *Handler) Rename(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c)
		return
	}
	if err := h.service.Rename(c.Request.Context(), c.GetString("user_id"), c.Param("id"), req.Name); err != nil {
		writeError(c, err)
		return
	}
	audit.Changed(c, "name")
	audit.Success(c)
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"updated": true}))
}

// Delete removes a Passkey, including the final credential.
func (h *Handler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.GetString("user_id"), c.Param("id")); err != nil {
		writeError(c, err)
		return
	}
	audit.Changed(c, "passkey")
	audit.Success(c)
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"deleted": true}))
}

func writeError(c *gin.Context, err error) {
	audit.Error(c, err)
	switch {
	case errors.Is(err, common.ErrEmailRequiredForPasskey):
		writeMachineError(c, http.StatusConflict, "EMAIL_REQUIRED_FOR_PASSKEY", "当前账号需要先绑定邮箱")
	case errors.Is(err, common.ErrWebAuthnCeremonyInvalid), errors.Is(err, common.ErrPasskeyCloneWarning):
		writeMachineError(c, http.StatusForbidden, "WEBAUTHN_CEREMONY_INVALID", "Passkey 验证失败或已过期")
	case errors.Is(err, common.ErrInvalidCaptcha):
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "验证码错误", Data: nil})
	case errors.Is(err, common.ErrInvalidOTP), errors.Is(err, common.ErrChallengeInvalid), errors.Is(err, common.ErrOTPExpired), errors.Is(err, common.ErrOTPAttemptsExceeded):
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "邮箱验证码错误或已过期", Data: nil})
	case errors.Is(err, common.ErrRateLimited):
		c.JSON(http.StatusTooManyRequests, ecode.Response[any]{Code: ecode.TooManyRequests, Message: "请求过于频繁", Data: nil})
	case errors.Is(err, common.ErrPasskeyNotFound), errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, ecode.Response[any]{Code: ecode.NotFound, Message: "Passkey 不存在", Data: nil})
	case errors.Is(err, common.ErrPasskeyNameInvalid):
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "Passkey 名称不能为空且不能超过 64 个字符", Data: nil})
	default:
		c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "Passkey 操作失败", Data: nil})
	}
}

func writeBadRequest(c *gin.Context) {
	c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误", Data: nil})
}

func writeMachineError(c *gin.Context, status int, code string, message string) {
	c.JSON(status, ecode.Response[any]{Code: ecode.Code(status), Message: message, Data: gin.H{"code": code}})
}
