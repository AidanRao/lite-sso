package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"sso-server/common/ecode"
)

// LoginWithEmailOTP handles email OTP-based login
func (h *AuthHandler) LoginWithEmailOTP(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
		OTP   string `json:"otp" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误", Data: nil})
		return
	}

	// Login with email OTP
	_, tokenData, err := h.auth.LoginWithEmailOTP(c.Request.Context(), c.Request, req.Email, req.OTP)
	if err != nil {
		c.JSON(http.StatusUnauthorized, ecode.Response[any]{Code: ecode.Unauthorized, Message: "登录失败", Data: nil})
		return
	}

	c.JSON(http.StatusOK, ecode.Response[map[string]interface{}]{Code: ecode.OK, Message: "登录成功", Data: tokenData})
}
