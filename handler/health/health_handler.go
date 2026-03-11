package health

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"sso-server/common/ecode"
)

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

func (h *HealthHandler) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, ecode.OKResponse("ok"))
}
