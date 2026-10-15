package user

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/common/ecode"
	"sso-server/dto"
	"sso-server/handler/api/auth"
	"sso-server/handler/audit"
)

// GetAccountDeletion returns the current user's deletion status.
func (h *UserHandler) GetAccountDeletion(c *gin.Context) {
	result, err := h.deletions.Status(c.Request.Context(), c.GetString("user_id"))
	if err != nil {
		writeAccountDeletionError(c, err)
		return
	}
	c.JSON(http.StatusOK, ecode.OKResponse(result))
}

// RequestAccountDeletion begins the cooling period after server-side re-authentication.
func (h *UserHandler) RequestAccountDeletion(c *gin.Context) {
	result, err := h.deletions.Request(c.Request.Context(), c.GetString("user_id"), c.GetString("session_id"))
	if err != nil {
		writeAccountDeletionError(c, err)
		return
	}
	auth.ClearLoginCookies(c, h.cookieSecure)
	audit.Completed(c, "sessions_revoked")
	audit.Success(c)
	c.JSON(http.StatusAccepted, ecode.OKResponse(result))
}

// CheckAccountDeletionWindow avoids prompting for verification after cancellation has expired.
func (h *UserHandler) CheckAccountDeletionWindow(c *gin.Context) {
	result, err := h.deletions.Status(c.Request.Context(), c.GetString("user_id"))
	if err != nil {
		writeAccountDeletionError(c, err)
		c.Abort()
		return
	}
	if result.Status != "active" && !result.CanCancel {
		writeAccountDeletionError(c, common.ErrAccountDeletionDue)
		c.Abort()
		return
	}
	c.Next()
}

// CancelAccountDeletion cancels a pending request and requires a fresh normal login.
func (h *UserHandler) CancelAccountDeletion(c *gin.Context) {
	if err := h.deletions.Cancel(c.Request.Context(), c.GetString("user_id"), c.GetString("session_id")); err != nil {
		writeAccountDeletionError(c, err)
		return
	}
	auth.ClearLoginCookies(c, h.cookieSecure)
	audit.Success(c)
	c.JSON(http.StatusOK, ecode.OKResponse(dto.AccountDeletionResponse{Status: "active"}))
}

func writeAccountDeletionError(c *gin.Context, err error) {
	audit.Error(c, err)
	switch {
	case errors.Is(err, common.ErrAccountDeletionDue):
		c.JSON(http.StatusConflict, ecode.Response[any]{Code: ecode.Conflict, Message: "注销冷静期已结束，无法撤销", Data: gin.H{"code": "ACCOUNT_DELETION_DUE"}})
	case errors.Is(err, common.ErrSessionRevoked), errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusUnauthorized, ecode.Response[any]{Code: ecode.Unauthorized, Message: "请重新登录", Data: nil})
	default:
		c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "账号注销操作失败，请重试", Data: nil})
	}
}
