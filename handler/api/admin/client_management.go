package admin

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"sso-server/common/ecode"
	"sso-server/handler/audit"
	"sso-server/service/systemadmin"
)

// GetOAuthClient returns details without loading secrets into the response.
func (h *AdminHandler) GetOAuthClient(c *gin.Context) {
	id, ok := parseOAuthClientID(c)
	if !ok {
		return
	}
	client, err := h.admin.GetOAuthClient(c.Request.Context(), id)
	if err != nil {
		writeOAuthClientError(c, err, "获取客户端失败")
		return
	}
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"client": client}))
}

// ListOAuthClientUsers returns one page of client associations.
func (h *AdminHandler) ListOAuthClientUsers(c *gin.Context) {
	id, ok := parseOAuthClientID(c)
	if !ok {
		return
	}
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, sizeErr := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if err != nil || sizeErr != nil || page < 1 || page > 1000000 || pageSize < 1 || pageSize > 100 {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "分页参数无效"})
		return
	}
	result, err := h.admin.ListOAuthClientUsers(c.Request.Context(), id, page, pageSize)
	if err != nil {
		writeOAuthClientError(c, err, "获取客户端用户失败")
		return
	}
	c.JSON(http.StatusOK, ecode.OKResponse(result))
}

// RemoveOAuthClientUsers revokes one association or all associations, including other pages.
func (h *AdminHandler) RemoveOAuthClientUsers(c *gin.Context) {
	id, ok := parseOAuthClientID(c)
	if !ok {
		return
	}
	result, err := h.admin.RemoveOAuthClientUsers(c.Request.Context(), id, c.Param("userID"))
	if err != nil {
		writeOAuthClientError(c, err, "移除客户端用户失败")
		return
	}
	audit.Changed(c, "user_ids")
	audit.Success(c)
	c.JSON(http.StatusOK, ecode.OKResponse(result))
}

// RotateOAuthClientSecret returns a newly persisted single client secret.
func (h *AdminHandler) RotateOAuthClientSecret(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	id, ok := parseOAuthClientID(c)
	if !ok {
		return
	}
	secret, err := h.admin.RotateOAuthClientSecret(c.Request.Context(), id)
	if err != nil {
		writeOAuthClientError(c, err, "轮换客户端密钥失败")
		return
	}
	audit.Client(c, secret.ClientID)
	audit.Changed(c, "client_secret")
	audit.Success(c)
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"secret": secret}))
}

// DeleteOAuthClient refuses nonempty clients and permanently deletes empty ones.
func (h *AdminHandler) DeleteOAuthClient(c *gin.Context) {
	id, ok := parseOAuthClientID(c)
	if !ok {
		return
	}
	client, err := h.admin.DeleteOAuthClient(c.Request.Context(), id)
	if err != nil {
		var conflict *systemadmin.ClientHasUsersError
		if errors.As(err, &conflict) {
			audit.Denied(c, "CLIENT_HAS_USERS")
			c.JSON(http.StatusConflict, ecode.Response[any]{Code: ecode.Conflict, Message: "请先在 Users 中移除全部用户", Data: gin.H{"code": "CLIENT_HAS_USERS", "user_count": conflict.UserCount}})
			return
		}
		writeOAuthClientError(c, err, "删除客户端失败")
		return
	}
	audit.Client(c, client.ClientID)
	audit.Success(c)
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"deleted": true}))
}
