package server

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"sso-server/common"
	"sso-server/common/ecode"
	"sso-server/service/reauth"
)

// RequirePasskeyReauth protects any endpoint that explicitly requires a recent Passkey verification.
func RequirePasskeyReauth(authorizer reauth.ReauthAuthorizer) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("X-Reauth-Token")
		_, err := authorizer.Authorize(c.Request.Context(), token, c.GetString("user_id"), c.GetString("session_id"))
		if err == nil {
			c.Next()
			return
		}
		machineCode := "REAUTH_TOKEN_INVALID"
		message := "Passkey 授权无效或已过期"
		if errors.Is(err, common.ErrReauthRequired) {
			machineCode = "REAUTH_REQUIRED"
			message = "需要 Passkey 验证"
		}
		c.JSON(http.StatusForbidden, ecode.Response[any]{Code: ecode.Forbidden, Message: message, Data: gin.H{"code": machineCode}})
		c.Abort()
	}
}
