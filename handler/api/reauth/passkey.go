package reauth

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"sso-server/common/ecode"
)

// PasskeyOptions returns assertion options for a Passkey re-authentication ceremony.
func (h *Handler) PasskeyOptions(c *gin.Context) {
	result, err := h.passkey.BeginReauth(c.Request.Context(), c.GetString("user_id"), c.GetString("session_id"), c.GetHeader("Origin"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, ecode.OKResponse(result))
}

// VerifyPasskey verifies an assertion and returns a short-lived grant.
func (h *Handler) VerifyPasskey(c *gin.Context) {
	var req struct {
		CeremonyID string          `json:"ceremony_id" binding:"required"`
		Response   json.RawMessage `json:"response" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBadRequest(c)
		return
	}
	result, err := h.passkey.FinishReauth(c.Request.Context(), c.GetString("user_id"), c.GetString("session_id"), c.GetHeader("Origin"), req.CeremonyID, req.Response)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, ecode.OKResponse(result))
}
