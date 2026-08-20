package auth

import (
	"crypto/sha256"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"sso-server/common/ecode"
	"sso-server/conf"
	serviceauth "sso-server/service/auth"
)

// LoginWithEmailOTP handles challenge-based email login.
func (h *AuthHandler) LoginWithEmailOTP(c *gin.Context) {
	var req struct {
		ChallengeID string `json:"challenge_id" binding:"required"`
		OTP         string `json:"code" binding:"required,len=6"`
		Redirect    string `json:"redirect"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误", Data: nil})
		return
	}

	deviceID, ok := serviceauth.DeviceIDFromRequest(c.Request)
	if !ok {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "设备标识已失效，请重新获取验证码", Data: gin.H{"code": "DEVICE_REQUIRED"}})
		return
	}
	user, err := h.auth.LoginWithEmailOTP(c.Request.Context(), req.ChallengeID, req.OTP, serviceauth.EmailOTPLoginContext{
		DeviceID: deviceID,
		IP:       serviceauth.RequestIP(c.Request),
	})
	if err != nil {
		log.Printf("auth email login failed: stage=challenge_verify challenge_id_hash=%s ip=%s error=%v", hashIdentifier(req.ChallengeID), serviceauth.RequestIP(c.Request), err)
		writeAuthError(c, err)
		return
	}
	result, pair, err := h.auth.CompleteLoginWithContext(c.Request.Context(), user.ID, req.Redirect, serviceauth.LoginMetadata{
		DeviceID:  deviceID,
		IP:        serviceauth.RequestIP(c.Request),
		UserAgent: c.Request.UserAgent(),
	}, serviceauth.AuthMethodEmailOTP)
	if err != nil {
		log.Printf("auth email login failed: stage=session_create user_id=%s ip=%s error=%v", user.ID, serviceauth.RequestIP(c.Request), err)
		writeAuthError(c, err)
		return
	}
	WriteRefreshCookie(c, pair.RefreshToken, conf.GetEnv() == conf.EnvProd, h.auth.RefreshTokenTTL())
	c.JSON(http.StatusOK, ecode.OKResponse(result))
}

func hashIdentifier(value string) string {
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", sum[:])
}
