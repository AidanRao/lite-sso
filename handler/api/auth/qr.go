package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"sso-server/common"
	"sso-server/common/ecode"
	"sso-server/conf"
	serviceauth "sso-server/service/auth"
)

// GenerateQRCode generates a QR code for login
func (h *AuthHandler) GenerateQRCode(c *gin.Context) {
	deviceID, isNewDevice := serviceauth.EnsureDeviceID(c.Request)
	code, err := h.auth.GenerateQRCodeWithDevice(c.Request.Context(), c.Query("redirect"), deviceID)
	if err != nil {
		switch {
		case errors.Is(err, common.ErrInvalidRedirect):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "跳转地址无效", Data: nil})
		default:
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "生成二维码失败", Data: nil})
		}
		return
	}
	if isNewDevice {
		WriteDeviceCookie(c, deviceID)
	}

	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{
		"code": code,
	}))
}

// PollQRCode polls the status of a QR code
func (h *AuthHandler) PollQRCode(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误", Data: nil})
		return
	}

	qrData, err := h.auth.PollQRCode(c.Request.Context(), code)
	if err != nil {
		switch {
		case errors.Is(err, common.ErrQRCodeExpired):
			c.JSON(http.StatusGone, ecode.Response[any]{Code: ecode.InternalServer, Message: "二维码已过期", Data: nil})
		default:
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "查询失败", Data: nil})
		}
		return
	}

	data := gin.H{
		"status": qrData.Status,
	}

	if qrData.Status == serviceauth.QRCodeStatusConfirmed {
		data["login_ticket"] = qrData.LoginTicket
	}

	c.JSON(http.StatusOK, ecode.OKResponse(data))
}

// ScanQRCode scans a QR code
func (h *AuthHandler) ScanQRCode(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
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
	err := h.auth.ScanQRCode(c.Request.Context(), req.Code, userID)
	if err != nil {
		switch {
		case errors.Is(err, common.ErrQRCodeExpired):
			c.JSON(http.StatusGone, ecode.Response[any]{Code: ecode.InternalServer, Message: "二维码已过期", Data: nil})
		case errors.Is(err, common.ErrQRCodeInvalidStatus):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "二维码状态无效", Data: nil})
		default:
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "扫码失败", Data: nil})
		}
		return
	}

	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{
		"scanned": true,
	}))
}

// ConfirmQRCode confirms a QR code login
func (h *AuthHandler) ConfirmQRCode(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
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
	err := h.auth.ConfirmQRCode(c.Request.Context(), req.Code, userID)
	if err != nil {
		switch {
		case errors.Is(err, common.ErrQRCodeExpired):
			c.JSON(http.StatusGone, ecode.Response[any]{Code: ecode.InternalServer, Message: "二维码已过期", Data: nil})
		case errors.Is(err, common.ErrQRCodeInvalidStatus):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "二维码状态无效", Data: nil})
		case errors.Is(err, common.ErrQRCodeInvalidUser):
			c.JSON(http.StatusForbidden, ecode.Response[any]{Code: ecode.Forbidden, Message: "用户不匹配", Data: nil})
		default:
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "确认失败", Data: nil})
		}
		return
	}

	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{
		"confirmed": true,
	}))
}

func (h *AuthHandler) CompleteQRCode(c *gin.Context) {
	var req struct {
		Code        string `json:"code" binding:"required"`
		LoginTicket string `json:"login_ticket" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误", Data: nil})
		return
	}

	deviceID, isNewDevice := serviceauth.EnsureDeviceID(c.Request)
	result, pair, err := h.auth.CompleteQRCodeLoginWithMetadata(c.Request.Context(), req.Code, req.LoginTicket, serviceauth.LoginMetadata{
		DeviceID:  deviceID,
		IP:        serviceauth.RequestIP(c.Request, h.trustProxyHeaders),
		UserAgent: c.Request.UserAgent(),
	})
	if err != nil {
		switch {
		case errors.Is(err, common.ErrQRCodeExpired):
			c.JSON(http.StatusGone, ecode.Response[any]{Code: ecode.InternalServer, Message: "二维码已过期", Data: nil})
		case errors.Is(err, common.ErrQRCodeInvalidStatus), errors.Is(err, common.ErrQRCodeInvalidTicket):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "二维码状态无效", Data: nil})
		case errors.Is(err, common.ErrUserInactive):
			c.JSON(http.StatusForbidden, ecode.Response[any]{Code: ecode.Forbidden, Message: "用户已禁用", Data: nil})
		default:
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "登录失败", Data: nil})
		}
		return
	}

	if isNewDevice {
		WriteDeviceCookie(c, deviceID)
	}
	WriteLoginCookies(c, pair, conf.GetEnv() == conf.EnvProd, h.auth.RefreshTokenTTL())
	c.JSON(http.StatusOK, ecode.OKResponse(result))
}
