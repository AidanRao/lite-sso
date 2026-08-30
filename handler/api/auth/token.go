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

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	cookie, err := c.Cookie(serviceauth.RefreshTokenCookieName)
	if err != nil || cookie == "" {
		c.JSON(http.StatusUnauthorized, ecode.Response[any]{Code: ecode.Unauthorized, Message: "Refresh Token 无效", Data: gin.H{"code": "REFRESH_TOKEN_INVALID"}})
		return
	}
	pair, err := h.auth.RefreshTokens(c.Request.Context(), cookie)
	if err != nil {
		if errors.Is(err, common.ErrRefreshTokenInvalid) {
			ClearLoginCookies(c, conf.GetEnv() == conf.EnvProd)
			c.JSON(http.StatusUnauthorized, ecode.Response[any]{Code: ecode.Unauthorized, Message: "Refresh Token 无效", Data: gin.H{"code": "REFRESH_TOKEN_INVALID"}})
			return
		}
		c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "刷新失败", Data: nil})
		return
	}
	WriteLoginCookies(c, pair, conf.GetEnv() == conf.EnvProd, h.auth.RefreshTokenTTL())
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{
		"access_token": pair.AccessToken,
		"token_type":   "Bearer",
		"expires_in":   pair.ExpiresIn,
	}))
}
