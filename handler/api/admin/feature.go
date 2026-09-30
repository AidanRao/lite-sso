package admin

import (
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"sso-server/common/ecode"
	"sso-server/dto"
	"sso-server/handler/audit"
	"sso-server/service/feature"
)

// ListFeatures returns registered release policies for administrators.
func (h *AdminHandler) ListFeatures(c *gin.Context) {
	policies, err := h.features.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "获取功能发布配置失败"})
		return
	}
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"features": policies}))
}

// UpdateFeature saves a complete policy and audits only the changed field names.
func (h *AdminHandler) UpdateFeature(c *gin.Context) {
	var req dto.FeatureUpdate
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误"})
		return
	}
	fields, err := h.features.Update(c.Request.Context(), c.Param("key"), req)
	if err != nil {
		audit.Error(c, err)
		if errors.Is(err, feature.ErrInvalidPolicy) {
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "功能配置无效，请检查开放范围、百分比及用户"})
			return
		}
		c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "保存功能发布配置失败"})
		return
	}
	audit.Changed(c, fields...)
	audit.Success(c)
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{}))
}
