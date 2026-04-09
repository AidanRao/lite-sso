package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"sso-server/common/ecode"
	"sso-server/conf"
	serviceauth "sso-server/service/auth"
)

func (h *AuthHandler) Logout(c *gin.Context) {
	sessionID, err := c.Cookie(serviceauth.SessionCookieName)
	if err != nil || sessionID == "" {
		c.JSON(http.StatusUnauthorized, ecode.Response[any]{Code: ecode.Unauthorized, Message: "未授权", Data: nil})
		return
	}

	if err := h.auth.InvalidateSession(c.Request.Context(), sessionID); err != nil {
		c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "退出失败", Data: nil})
		return
	}

	ClearSessionCookie(c, conf.GetEnv() == conf.EnvProd)
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"logged_out": true}))
}
