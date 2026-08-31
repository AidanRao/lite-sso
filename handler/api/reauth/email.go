package reauth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"sso-server/common/ecode"
	apiauth "sso-server/handler/api/auth"
	"sso-server/handler/audit"
	serviceauth "sso-server/service/auth"
)

// SendEmail sends a verification code to the current account email.
func (h *Handler) SendEmail(c *gin.Context) {
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
	result, err := h.reauth.BeginEmail(c.Request.Context(), c.GetString("user_id"), c.GetString("session_id"), deviceID, req.CaptchaID, req.Captcha, serviceauth.OTPRequestContext{
		DeviceID: deviceID,
		IP:       serviceauth.RequestIP(c.Request, h.trustProxyHeaders),
	})
	if err != nil {
		writeError(c, err)
		return
	}
	audit.Success(c)
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{
		"sent":         true,
		"challenge_id": result.ChallengeID,
		"expires_in":   result.ExpiresIn,
		"resend_after": result.ResendAfter,
	}))
}

// VerifyEmail verifies a code and returns a short-lived grant.
func (h *Handler) VerifyEmail(c *gin.Context) {
	var req struct {
		ChallengeID string `json:"challenge_id" binding:"required"`
		Code        string `json:"code" binding:"required,len=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c)
		return
	}
	deviceID, ok := serviceauth.DeviceIDFromRequest(c.Request)
	if !ok {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "设备标识已失效，请重新获取验证码", Data: gin.H{"code": "DEVICE_REQUIRED"}})
		return
	}
	result, err := h.reauth.FinishEmail(c.Request.Context(), c.GetString("user_id"), c.GetString("session_id"), deviceID, serviceauth.RequestIP(c.Request, h.trustProxyHeaders), req.ChallengeID, req.Code)
	if err != nil {
		writeError(c, err)
		return
	}
	audit.Success(c)
	c.JSON(http.StatusOK, ecode.OKResponse(result))
}
