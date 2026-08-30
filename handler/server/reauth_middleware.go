package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"sso-server/common"
	"sso-server/common/ecode"
	"sso-server/service/reauth"
)

// RequireReauth protects endpoints that require a recent explicit verification.
func RequireReauth(authorizer reauth.ReauthAuthorizer) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimSpace(c.GetHeader("X-Reauth-Token"))
		var err error
		if token == "" {
			_, err = authorizer.AuthorizeSession(c.Request.Context(), c.GetString("user_id"), c.GetString("session_id"))
		} else {
			_, err = authorizer.Authorize(c.Request.Context(), token, c.GetString("user_id"), c.GetString("session_id"))
		}
		if err == nil {
			c.Next()
			return
		}
		if !errors.Is(err, common.ErrReauthRequired) && !errors.Is(err, common.ErrReauthTokenInvalid) {
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "重新验证检查失败", Data: nil})
			c.Abort()
			return
		}
		descriptor, describeErr := authorizer.Describe(c.Request.Context(), c.GetString("user_id"))
		if describeErr != nil {
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "重新验证检查失败", Data: nil})
			c.Abort()
			return
		}
		machineCode := "REAUTH_REQUIRED"
		if errors.Is(err, common.ErrReauthTokenInvalid) {
			machineCode = "REAUTH_TOKEN_INVALID"
		}
		c.JSON(http.StatusForbidden, ecode.Response[any]{
			Code:    ecode.Forbidden,
			Message: "需要重新验证",
			Data:    gin.H{"code": machineCode, "reauth": descriptor},
		})
		c.Abort()
	}
}
