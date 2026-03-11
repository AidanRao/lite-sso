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

	// TODO: Implement email OTP login in auth service
	// user, tokenData, err := h.auth.LoginWithEmailOTP(c.Request.Context(), c.Request, req.Email, req.OTP)

	c.JSON(http.StatusNotImplemented, ecode.Response[any]{Code: ecode.InternalServer, Message: "功能未实现", Data: nil})
}
