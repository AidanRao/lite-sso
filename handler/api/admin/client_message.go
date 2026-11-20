package admin

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"

	"sso-server/common/ecode"
	"sso-server/dto"
	"sso-server/handler/audit"
	"sso-server/manager/messagecenter"
	"sso-server/service/systemadmin"
)

// SendOAuthClientUserMessage submits one email; the browser coordinates multi-user sends.
func (h *AdminHandler) SendOAuthClientUserMessage(c *gin.Context) {
	id, ok := parseOAuthClientID(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 128*1024)
	var request dto.ClientMessageRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeClientMessageError(c, http.StatusBadRequest, "INVALID_MESSAGE", "消息参数无效，请填写 sender-key、template-key 和 JSON 对象参数")
		return
	}
	err := h.messages.Send(c.Request.Context(), id, c.Param("userID"), c.GetHeader("Idempotency-Key"), request)
	if err == nil {
		audit.Success(c)
		c.JSON(http.StatusAccepted, ecode.OKResponse(gin.H{"user_id": c.Param("userID"), "status": "accepted"}))
		return
	}
	var submission *messagecenter.SubmissionError
	var timeout net.Error
	switch {
	case errors.Is(err, systemadmin.ErrClientMessageInvalid):
		writeClientMessageError(c, http.StatusBadRequest, "INVALID_MESSAGE", "消息参数或幂等键无效；参数必须是至多 64 KiB 的 JSON 对象")
	case errors.Is(err, systemadmin.ErrClientMessageRecipientNotFound):
		writeClientMessageError(c, http.StatusNotFound, "MESSAGE_RECIPIENT_NOT_FOUND", "客户端、用户或用户关联已不存在")
	case errors.Is(err, systemadmin.ErrClientMessageEmailRequired):
		writeClientMessageError(c, http.StatusConflict, "MESSAGE_EMAIL_REQUIRED", "用户没有已验证的主邮箱")
	case errors.Is(err, systemadmin.ErrClientMessageUnavailable):
		writeClientMessageError(c, http.StatusServiceUnavailable, "MESSAGE_SENDING_UNAVAILABLE", "消息发送当前不可用")
	case errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &timeout) && timeout.Timeout()):
		writeClientMessageError(c, http.StatusGatewayTimeout, "MESSAGE_SUBMISSION_UNKNOWN", "提交超时，结果未知；可以使用原发送记录重试")
	case errors.As(err, &submission):
		if submission.Unknown {
			writeClientMessageError(c, http.StatusBadGateway, "MESSAGE_SUBMISSION_UNKNOWN", "消息中心暂不可用，提交结果未知；可以使用原发送记录重试")
		} else {
			writeClientMessageError(c, http.StatusBadGateway, "MESSAGE_SUBMISSION_REJECTED", "消息中心拒绝提交，请检查 sender-key、template-key、参数及授权配置")
		}
	default:
		writeClientMessageError(c, http.StatusInternalServerError, "MESSAGE_SUBMISSION_FAILED", "消息提交失败")
	}
}

func writeClientMessageError(c *gin.Context, status int, code, message string) {
	audit.Failure(c, code)
	c.JSON(status, ecode.Response[any]{Code: ecode.Code(status), Message: message, Data: gin.H{"code": code}})
}
