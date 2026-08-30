package reauth

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"sso-server/common"
	"sso-server/common/ecode"
)

func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, common.ErrInvalidCaptcha):
		writeMachineError(c, http.StatusBadRequest, "CAPTCHA_INVALID", "验证码错误")
	case errors.Is(err, common.ErrInvalidOTP):
		writeMachineError(c, http.StatusBadRequest, "EMAIL_CODE_INVALID", "邮箱验证码错误")
	case errors.Is(err, common.ErrOTPExpired):
		writeMachineError(c, http.StatusGone, "EMAIL_CODE_EXPIRED", "邮箱验证码已过期")
	case errors.Is(err, common.ErrOTPAttemptsExceeded):
		writeMachineError(c, http.StatusTooManyRequests, "EMAIL_CODE_ATTEMPTS_EXCEEDED", "验证码尝试次数过多")
	case errors.Is(err, common.ErrChallengeInvalid):
		writeMachineError(c, http.StatusBadRequest, "EMAIL_CODE_INVALID", "验证码无效，请重新获取")
	case errors.Is(err, common.ErrRateLimited):
		writeRateLimited(c, err)
	case errors.Is(err, common.ErrReauthMethodUnavailable):
		writeMachineError(c, http.StatusConflict, "REAUTH_METHOD_UNAVAILABLE", "当前账号无法使用邮箱验证")
	case errors.Is(err, common.ErrPasskeyRequired):
		writeMachineError(c, http.StatusConflict, "PASSKEY_REQUIRED", "请先注册 Passkey")
	case errors.Is(err, common.ErrWebAuthnCeremonyInvalid), errors.Is(err, common.ErrPasskeyCloneWarning):
		writeMachineError(c, http.StatusForbidden, "WEBAUTHN_CEREMONY_INVALID", "Passkey 验证失败或已过期")
	default:
		c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "重新验证失败", Data: nil})
	}
}

func writeRateLimited(c *gin.Context, err error) {
	retryAfter := 1
	var rateError common.RateLimitedError
	if errors.As(err, &rateError) && rateError.RetryAfterSeconds > 0 {
		retryAfter = rateError.RetryAfterSeconds
	}
	c.Header("Retry-After", strconv.Itoa(retryAfter))
	c.JSON(http.StatusTooManyRequests, ecode.Response[any]{
		Code:    ecode.TooManyRequests,
		Message: "请求过于频繁",
		Data:    gin.H{"code": "RATE_LIMITED", "retry_after_seconds": retryAfter},
	})
}

func writeBadRequest(c *gin.Context) {
	c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误", Data: nil})
}

func writeMachineError(c *gin.Context, status int, code string, message string) {
	c.JSON(status, ecode.Response[any]{Code: ecode.Code(status), Message: message, Data: gin.H{"code": code}})
}
