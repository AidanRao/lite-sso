package user

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"sso-server/common/ecode"
	"sso-server/dto"
)

// GetPermissions returns uncached access decisions for the authenticated user.
func (h *UserHandler) GetPermissions(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, ecode.Response[any]{Code: ecode.Unauthorized, Message: "未授权"})
		return
	}
	states, err := h.permissions.States(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, ecode.Response[any]{Code: ecode.ServiceUnavailable, Message: "权限状态暂时不可用"})
		return
	}
	c.JSON(http.StatusOK, ecode.OKResponse(dto.PermissionsResponse{IsAdmin: h.config.IsAdminUser(userID), Features: states}))
}
