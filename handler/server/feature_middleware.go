package server

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"sso-server/common/ecode"
	"sso-server/service/feature"
)

// RequireFeature checks the current database policy after session authentication.
func RequireFeature(service *feature.Service, key string) gin.HandlerFunc {
	return func(c *gin.Context) {
		states, err := service.States(c.Request.Context(), c.GetString("user_id"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, ecode.Response[any]{Code: ecode.ServiceUnavailable, Message: "功能状态暂时不可用"})
			return
		}
		if !states[key].Enabled {
			c.AbortWithStatusJSON(http.StatusForbidden, ecode.Response[any]{Code: ecode.Forbidden, Message: "该功能暂未向你开放", Data: gin.H{"code": "FEATURE_NOT_ENABLED", "feature_key": key}})
			return
		}
		c.Next()
	}
}
