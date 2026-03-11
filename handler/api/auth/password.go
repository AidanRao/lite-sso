package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"sso-server/common/ecode"
	"sso-server/service/auth"
)

// LoginWithPassword handles password-based login
func (h *AuthHandler) LoginWithPassword(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误", Data: nil})
		return
	}

	user, tokenData, err := h.auth.LoginWithPassword(c.Request.Context(), c.Request, req.Email, req.Password)
	if err != nil {
		switch err {
		case auth.ErrInvalidCredentials:
			c.JSON(http.StatusUnauthorized, ecode.Response[any]{Code: ecode.Unauthorized, Message: "邮箱或密码错误", Data: nil})
		case auth.ErrUserInactive:
			c.JSON(http.StatusForbidden, ecode.Response[any]{Code: ecode.Forbidden, Message: "用户已禁用", Data: nil})
		default:
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "登录失败", Data: nil})
		}
		return
	}

	data := gin.H{
		"user": gin.H{
			"id":         user.ID,
			"email":      user.Email,
			"username":   user.Username,
			"avatar_url": user.AvatarURL,
		},
	}

	if tokenData != nil {
		if v, ok := tokenData["access_token"]; ok {
			data["access_token"] = v
		}
		if v, ok := tokenData["token_type"]; ok {
			data["token_type"] = v
		}
		if v, ok := tokenData["expires_in"]; ok {
			data["expires_in"] = v
		}
	}

	c.JSON(http.StatusOK, ecode.OKResponse(data))
}
