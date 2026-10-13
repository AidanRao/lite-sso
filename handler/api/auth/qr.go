package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"sso-server/common"
	"sso-server/common/ecode"
	"sso-server/handler/audit"
	"sso-server/service/auth"
)

// GenerateQRCode creates the payload and separate browser credentials.
func (h *AuthHandler) GenerateQRCode(c *gin.Context) {
	deviceID, isNewDevice := auth.EnsureDeviceID(c.Request)
	challenge, err := h.auth.GenerateQRCode(c.Request.Context(), c.Query("redirect"), h.qrMetadata(c, deviceID))
	if err != nil {
		writeQRError(c, err)
		return
	}
	if isNewDevice {
		WriteDeviceCookie(c, deviceID, h.cookieSecure)
	}
	qrNoCache(c)
	c.JSON(http.StatusOK, ecode.OKResponse(challenge))
}

// PollQRCode only discloses a ticket to the requesting browser.
func (h *AuthHandler) PollQRCode(c *gin.Context) {
	if c.Query("code") == "" {
		writeQRError(c, common.ErrQRCodeInvalidStatus)
		return
	}
	deviceID, _ := auth.DeviceIDFromRequest(c.Request)
	data, err := h.auth.PollQRCode(c.Request.Context(), c.Query("code"), deviceID, c.GetHeader("X-QR-Browser-Token"))
	if err != nil {
		writeQRError(c, err)
		return
	}
	response := gin.H{"status": data.Status, "expires_at": data.ExpiresAt}
	if data.Status == auth.QRCodeStatusConfirmed {
		response["login_ticket"] = data.LoginTicket
	}
	qrNoCache(c)
	c.JSON(http.StatusOK, ecode.OKResponse(response))
}

// ScanQRCode returns a preview for an authenticated OAuth scanner.
func (h *AuthHandler) ScanQRCode(c *gin.Context) {
	actor, ok := h.qrActor(c)
	if !ok {
		return
	}
	code, _, ok := qrRequest(c)
	if !ok {
		return
	}
	preview, err := h.auth.ScanQRCode(c.Request.Context(), code, actor)
	if err != nil {
		writeQRError(c, err)
		return
	}
	audit.Success(c)
	qrNoCache(c)
	c.JSON(http.StatusOK, ecode.OKResponse(preview))
}

// ConfirmQRCode approves a request bound to this OAuth user and application.
func (h *AuthHandler) ConfirmQRCode(c *gin.Context) { h.qrScannerAction(c, false) }

// RejectQRCode rejects a request bound to this OAuth user and application.
func (h *AuthHandler) RejectQRCode(c *gin.Context) { h.qrScannerAction(c, true) }

// CancelQRCode invalidates the original browser's request.
func (h *AuthHandler) CancelQRCode(c *gin.Context) {
	code, _, ok := qrRequest(c)
	if !ok {
		return
	}
	deviceID, _ := auth.DeviceIDFromRequest(c.Request)
	if err := h.auth.CancelQRCode(c.Request.Context(), code, deviceID, c.GetHeader("X-QR-Browser-Token")); err != nil {
		writeQRError(c, err)
		return
	}
	audit.Success(c)
	qrNoCache(c)
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"cancelled": true}))
}

// CompleteQRCode establishes the SSO browser session after one-time consumption.
func (h *AuthHandler) CompleteQRCode(c *gin.Context) {
	code, ticket, ok := qrRequest(c)
	if !ok {
		return
	}
	deviceID, _ := auth.DeviceIDFromRequest(c.Request)
	qrData, err := h.auth.PollQRCode(c.Request.Context(), code, deviceID, c.GetHeader("X-QR-Browser-Token"))
	if err != nil {
		writeQRError(c, err)
		return
	}
	audit.Client(c, qrData.ClientID)
	result, pair, err := h.auth.CompleteQRCodeLogin(c.Request.Context(), code, ticket, c.GetHeader("X-QR-Browser-Token"), h.qrMetadata(c, deviceID))
	if err != nil {
		writeQRError(c, err)
		return
	}
	audit.Actor(c, result.User.ID, pair.SessionID)
	audit.Device(c, deviceID)
	audit.Completed(c, "session_created")
	WriteLoginCookies(c, pair, h.cookieSecure, h.auth.RefreshTokenTTL())
	audit.Success(c)
	qrNoCache(c)
	c.JSON(http.StatusOK, ecode.OKResponse(result))
}

func (h *AuthHandler) qrMetadata(c *gin.Context, deviceID string) auth.LoginMetadata {
	return auth.LoginMetadata{DeviceID: deviceID, IP: auth.RequestIP(c.Request, h.trustProxyHeaders), UserAgent: c.Request.UserAgent()}
}

func (h *AuthHandler) qrActor(c *gin.Context) (auth.QRActor, bool) {
	if h.oauth2 == nil {
		writeQRFailure(c, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录 SSO")
		return auth.QRActor{}, false
	}
	actor, permitted, err := h.oauth2.ValidateQRCodeToken(c.Request)
	if err != nil {
		writeQRFailure(c, http.StatusUnauthorized, "UNAUTHORIZED", "SSO 令牌无效或已过期")
		return actor, false
	}
	audit.Actor(c, actor.UserID, "")
	audit.Client(c, actor.ClientID)
	if err := h.auth.ValidateQRCodeActor(c.Request.Context(), actor); err != nil {
		writeQRError(c, err)
		return actor, false
	}
	if !permitted {
		writeQRError(c, common.ErrQRCodePermissionMissing)
		return actor, false
	}
	return actor, true
}

func (h *AuthHandler) qrScannerAction(c *gin.Context, reject bool) {
	actor, ok := h.qrActor(c)
	if !ok {
		return
	}
	code, _, ok := qrRequest(c)
	if !ok {
		return
	}
	var err error
	if reject {
		err = h.auth.RejectQRCode(c.Request.Context(), code, actor)
	} else {
		err = h.auth.ConfirmQRCode(c.Request.Context(), code, actor)
	}
	if err != nil {
		writeQRError(c, err)
		return
	}
	audit.Success(c)
	qrNoCache(c)
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"confirmed": !reject, "cancelled": reject}))
}

func qrRequest(c *gin.Context) (string, string, bool) {
	var request struct {
		Code        string `json:"code" binding:"required,max=64"`
		LoginTicket string `json:"login_ticket"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		writeQRFailure(c, http.StatusBadRequest, "BAD_REQUEST", "参数错误")
		return "", "", false
	}
	return request.Code, request.LoginTicket, true
}

func qrNoCache(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
}

func writeQRFailure(c *gin.Context, status int, code, message string) {
	qrNoCache(c)
	audit.Denied(c, code)
	c.JSON(status, ecode.Response[any]{Code: ecode.Code(status), Message: message, Data: gin.H{"code": code}})
}

func writeQRError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, common.ErrQRCodeExpired):
		writeQRFailure(c, http.StatusGone, "QR_CODE_EXPIRED", "二维码已过期，请重新扫描")
	case errors.Is(err, common.ErrQRCodeClientForbidden):
		writeQRFailure(c, http.StatusForbidden, "QR_CLIENT_FORBIDDEN", "此应用未获准扫码登录，请联系管理员")
	case errors.Is(err, common.ErrQRCodePermissionMissing):
		writeQRFailure(c, http.StatusForbidden, "QR_TOKEN_PERMISSION_MISSING", "请刷新 SSO 令牌以获取扫码权限")
	case errors.Is(err, common.ErrQRCodeBrowserInvalid):
		writeQRFailure(c, http.StatusForbidden, "QR_BROWSER_INVALID", "登录设备或浏览器凭证不匹配")
	case errors.Is(err, common.ErrUserInactive), errors.Is(err, common.ErrUserNotFound):
		writeQRFailure(c, http.StatusForbidden, "USER_INACTIVE", "账号不存在或已停用")
	case errors.Is(err, common.ErrQRCodeInvalidUser):
		writeQRFailure(c, http.StatusForbidden, "QR_CODE_INVALID_USER", "二维码已由其他账号或应用扫描")
	case errors.Is(err, common.ErrQRCodeInvalidStatus), errors.Is(err, common.ErrQRCodeInvalidTicket):
		writeQRFailure(c, http.StatusConflict, "QR_CODE_INVALID_STATUS", "二维码状态无效，请重新生成")
	case errors.Is(err, common.ErrInvalidRedirect):
		writeQRFailure(c, http.StatusBadRequest, "INVALID_REDIRECT", "跳转地址无效")
	default:
		qrNoCache(c)
		audit.Error(c, err)
		c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "扫码服务暂时不可用", Data: gin.H{"code": "QR_SERVICE_UNAVAILABLE"}})
	}
}
