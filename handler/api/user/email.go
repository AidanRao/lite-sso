package user

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"sso-server/common"
	"sso-server/common/ecode"
	"sso-server/dto"
)

// ListEmails returns all email identities owned by the current user.
func (h *UserHandler) ListEmails(c *gin.Context) {
	emails, maxAddresses, err := h.emails.List(c.Request.Context(), c.GetString("user_id"))
	if err != nil {
		writeEmailError(c, err, nil)
		return
	}
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"emails": emails, "max_addresses": maxAddresses}))
}

// AddEmail creates an unverified address and sends its verification link.
func (h *UserHandler) AddEmail(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeEmailBadRequest(c)
		return
	}
	email, err := h.emails.Add(c.Request.Context(), c.GetString("user_id"), req.Email)
	if err != nil {
		writeEmailError(c, err, email)
		return
	}
	c.JSON(http.StatusCreated, ecode.OKResponse(gin.H{"email": email, "verification_sent": true}))
}

// ResendEmailVerification rotates and resends an address verification link.
func (h *UserHandler) ResendEmailVerification(c *gin.Context) {
	if err := h.emails.Resend(c.Request.Context(), c.GetString("user_id"), c.Param("id")); err != nil {
		writeEmailError(c, err, nil)
		return
	}
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"verification_sent": true}))
}

// ConfirmEmailVerification consumes an authenticated email verification link.
func (h *UserHandler) ConfirmEmailVerification(c *gin.Context) {
	var req struct {
		Token string `json:"token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeEmailBadRequest(c)
		return
	}
	email, err := h.emails.Confirm(c.Request.Context(), c.GetString("user_id"), req.Token)
	if err != nil {
		writeEmailError(c, err, nil)
		return
	}
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"email": email, "verified": true}))
}

// SetPrimaryEmail changes the account's default email target.
func (h *UserHandler) SetPrimaryEmail(c *gin.Context) {
	if err := h.emails.SetPrimary(c.Request.Context(), c.GetString("user_id"), c.Param("id")); err != nil {
		writeEmailError(c, err, nil)
		return
	}
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"updated": true}))
}

// DeleteEmail removes an email identity. The last verified primary may be removed.
func (h *UserHandler) DeleteEmail(c *gin.Context) {
	if err := h.emails.Delete(c.Request.Context(), c.GetString("user_id"), c.Param("id")); err != nil {
		writeEmailError(c, err, nil)
		return
	}
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"deleted": true}))
}

func writeEmailError(c *gin.Context, err error, email *dto.UserEmailResponse) {
	data := gin.H{}
	if email != nil {
		data["email"] = email
	}
	write := func(status int, code string, message string) {
		data["code"] = code
		c.JSON(status, ecode.Response[any]{Code: ecode.Code(status), Message: message, Data: data})
	}

	switch {
	case errors.Is(err, common.ErrEmailInvalid):
		write(http.StatusBadRequest, "EMAIL_INVALID", "邮箱格式无效")
	case errors.Is(err, common.ErrEmailAlreadyAdded):
		write(http.StatusConflict, "EMAIL_ALREADY_ADDED", "该邮箱已添加")
	case errors.Is(err, common.ErrEmailExists):
		write(http.StatusConflict, "EMAIL_ALREADY_VERIFIED", "该邮箱已绑定其他账号")
	case errors.Is(err, common.ErrEmailLimitReached):
		write(http.StatusConflict, "EMAIL_LIMIT_REACHED", "已达到邮箱数量上限")
	case errors.Is(err, common.ErrEmailNotFound):
		write(http.StatusNotFound, "EMAIL_NOT_FOUND", "邮箱不存在")
	case errors.Is(err, common.ErrEmailNotVerified):
		write(http.StatusConflict, "EMAIL_NOT_VERIFIED", "邮箱尚未验证")
	case errors.Is(err, common.ErrPrimaryEmailDelete):
		write(http.StatusConflict, "PRIMARY_EMAIL_DELETE_FORBIDDEN", "存在其他已验证邮箱，请先切换主邮箱再删除")
	case errors.Is(err, common.ErrLastLoginMethod):
		write(http.StatusConflict, "LAST_LOGIN_METHOD", "至少需要保留一种可用登录方式")
	case errors.Is(err, common.ErrEmailVerificationInvalid):
		write(http.StatusForbidden, "EMAIL_VERIFICATION_INVALID", "验证链接无效或不属于当前账号")
	case errors.Is(err, common.ErrEmailVerificationExpired):
		write(http.StatusGone, "EMAIL_VERIFICATION_EXPIRED", "验证链接已过期，请重新发送")
	case errors.Is(err, common.ErrEmailVerificationResend):
		write(http.StatusTooManyRequests, "EMAIL_VERIFICATION_RATE_LIMITED", "请稍后再发送验证邮件")
	case errors.Is(err, common.ErrMessageNotSent):
		write(http.StatusBadGateway, "EMAIL_VERIFICATION_NOT_SENT", "验证邮件发送失败，请稍后重试")
	case errors.Is(err, common.ErrUserNotFound):
		write(http.StatusNotFound, "USER_NOT_FOUND", "用户不存在")
	default:
		write(http.StatusInternalServerError, "EMAIL_OPERATION_FAILED", "邮箱操作失败")
	}
}

func writeEmailBadRequest(c *gin.Context) {
	c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误", Data: nil})
}
