package user

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"sso-server/common/ecode"
	"sso-server/dto"
	"sso-server/service/user"
)

// ListAuditLogs exposes only the current authenticated user's audit facts.
func (h *UserHandler) ListAuditLogs(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, ecode.Response[any]{Code: ecode.Unauthorized, Message: "UNAUTHORIZED"})
		return
	}
	var query dto.AuditLogQuery
	if err := c.ShouldBindQuery(&query); err != nil || c.Request.URL.Query().Has("user_id") {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "INVALID_AUDIT_QUERY"})
		return
	}
	page, err := h.user.ListAuditLogs(c.Request.Context(), userID, query)
	if errors.Is(err, user.ErrAuditQueryInvalid) {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "INVALID_AUDIT_QUERY"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "AUDIT_QUERY_FAILED"})
		return
	}
	c.JSON(http.StatusOK, ecode.OKResponse(page))
}
