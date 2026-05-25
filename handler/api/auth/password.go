package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"sso-server/common"
	"sso-server/common/ecode"
	"sso-server/conf"
)

// LoginWithPassword handles password-based login
func (h *AuthHandler) LoginWithPassword(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
		Redirect string `json:"redirect"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误", Data: nil})
		return
	}

	user, err := h.auth.LoginWithPassword(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, common.ErrInvalidCredentials):
			c.JSON(http.StatusUnauthorized, ecode.Response[any]{Code: ecode.Unauthorized, Message: "邮箱或密码错误", Data: nil})
		case errors.Is(err, common.ErrUserInactive):
			c.JSON(http.StatusForbidden, ecode.Response[any]{Code: ecode.Forbidden, Message: "用户已禁用", Data: nil})
		default:
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "登录失败", Data: nil})
		}
		return
	}

	result, sessionID, err := h.auth.CompleteLogin(c.Request.Context(), user.ID, req.Redirect)
	if err != nil {
		switch {
		case errors.Is(err, common.ErrInvalidRedirect):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "跳转地址无效", Data: nil})
		default:
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "登录失败", Data: nil})
		}
		return
	}
	WriteSessionCookie(c, sessionID, conf.GetEnv() == conf.EnvProd)

	c.JSON(http.StatusOK, ecode.OKResponse(result))
}
