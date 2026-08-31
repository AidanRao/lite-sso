// Package admin exposes system administration HTTP APIs.
package admin

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/common/ecode"
	"sso-server/conf"
	"sso-server/dto"
	"sso-server/handler/audit"
	manageross "sso-server/manager/oss"
	"sso-server/service/systemadmin"
)

type AdminDeps struct {
	Config     *conf.Config
	DB         *gorm.DB
	ImageStore manageross.ImageStore
}

// AdminHandler handles system administration API requests.
type AdminHandler struct {
	admin *systemadmin.AdminService
}

// NewAdminHandler creates a system administration handler.
func NewAdminHandler(deps AdminDeps) *AdminHandler {
	return &AdminHandler{
		admin: systemadmin.NewAdminService(deps.Config, deps.DB, deps.ImageStore),
	}
}

const (
	maxOAuthClientLogoFileSize      int64 = 1 * 1024 * 1024
	maxOAuthClientLogoMultipartSize       = maxOAuthClientLogoFileSize + 64*1024
)

// ListUsers returns all users for administrators.
func (h *AdminHandler) ListUsers(c *gin.Context) {
	users, err := h.admin.ListUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "获取用户列表失败", Data: nil})
		return
	}

	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"users": users}))
}

// GetUserDetail returns profile detail for a user.
func (h *AdminHandler) GetUserDetail(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误", Data: nil})
		return
	}

	detail, err := h.admin.GetUserDetail(c.Request.Context(), userID)
	if err != nil {
		switch {
		case errors.Is(err, common.ErrUserNotFound):
			c.JSON(http.StatusNotFound, ecode.Response[any]{Code: ecode.NotFound, Message: "用户不存在", Data: nil})
		default:
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "获取用户详情失败", Data: nil})
		}
		return
	}

	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"profile": detail}))
}

// ListOAuthClients returns all connected platforms for administrators.
func (h *AdminHandler) ListOAuthClients(c *gin.Context) {
	clients, err := h.admin.ListOAuthClients(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "获取平台列表失败", Data: nil})
		return
	}

	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"clients": clients}))
}

// GetOAuthClientSecret returns a connected platform secret for administrators.
func (h *AdminHandler) GetOAuthClientSecret(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id64 == 0 {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误", Data: nil})
		return
	}

	secret, err := h.admin.GetOAuthClientSecret(c.Request.Context(), uint(id64))
	if err != nil {
		writeOAuthClientError(c, err, "获取平台密钥失败")
		return
	}

	audit.Client(c, secret.ClientID)
	audit.Success(c)
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"secret": secret}))
}

// CreateOAuthClient creates a connected platform.
func (h *AdminHandler) CreateOAuthClient(c *gin.Context) {
	var req dto.CreateOAuthClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误", Data: nil})
		return
	}

	client, err := h.admin.CreateOAuthClient(c.Request.Context(), req)
	if err != nil {
		writeOAuthClientError(c, err, "新增平台失败")
		return
	}

	audit.Target(c, "oauth_client", strconv.FormatUint(uint64(client.ID), 10))
	audit.Client(c, client.ClientID)
	audit.Changed(c, "name", "client_secret", "homepage_url", "redirect_uri", "logout_uri")
	audit.Success(c)
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"client": client}))
}

// UpdateOAuthClient updates a connected platform.
func (h *AdminHandler) UpdateOAuthClient(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id64 == 0 {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误", Data: nil})
		return
	}

	var req dto.UpdateOAuthClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误", Data: nil})
		return
	}

	client, err := h.admin.UpdateOAuthClient(c.Request.Context(), uint(id64), req)
	if err != nil {
		writeOAuthClientError(c, err, "更新平台失败")
		return
	}

	audit.Target(c, "oauth_client", strconv.FormatUint(uint64(client.ID), 10))
	audit.Client(c, client.ClientID)
	audit.Changed(c, "name", "homepage_url", "redirect_uri", "logout_uri")
	if req.ClientSecret != nil {
		audit.Changed(c, "client_secret")
	}
	audit.Success(c)
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"client": client}))
}

// UploadOAuthClientLogo uploads a logo for an OAuth client.
func (h *AdminHandler) UploadOAuthClientLogo(c *gin.Context) {
	id, ok := parseOAuthClientID(c)
	if !ok {
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxOAuthClientLogoMultipartSize)
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		writeOAuthClientLogoFormError(c, err)
		return
	}
	defer file.Close()

	if header.Size > maxOAuthClientLogoFileSize {
		writeOAuthClientLogoTooLarge(c)
		return
	}

	contentType, extension, err := manageross.ValidateImage(file)
	if err != nil {
		audit.Error(c, err)
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "仅支持 JPEG、PNG 或 WebP 图片", Data: nil})
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "应用 Logo 上传失败", Data: nil})
		return
	}

	client, err := h.admin.UploadOAuthClientLogo(c.Request.Context(), id, contentType, extension, file, header.Size)
	if err != nil {
		writeOAuthClientLogoError(c, err, "应用 Logo 上传失败")
		return
	}

	audit.Target(c, "oauth_client", strconv.FormatUint(uint64(client.ID), 10))
	audit.Client(c, client.ClientID)
	audit.Changed(c, "logo")
	audit.Success(c)
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"client": client}))
}

// ClearOAuthClientLogo removes an OAuth client logo.
func (h *AdminHandler) ClearOAuthClientLogo(c *gin.Context) {
	id, ok := parseOAuthClientID(c)
	if !ok {
		return
	}

	client, err := h.admin.ClearOAuthClientLogo(c.Request.Context(), id)
	if err != nil {
		writeOAuthClientLogoError(c, err, "清除应用 Logo 失败")
		return
	}

	audit.Target(c, "oauth_client", strconv.FormatUint(uint64(client.ID), 10))
	audit.Client(c, client.ClientID)
	audit.Changed(c, "logo")
	audit.Success(c)
	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{"client": client}))
}

func parseOAuthClientID(c *gin.Context) (uint, bool) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id64 == 0 {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误", Data: nil})
		return 0, false
	}
	return uint(id64), true
}

func writeOAuthClientLogoFormError(c *gin.Context, err error) {
	audit.Error(c, err)
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		writeOAuthClientLogoTooLarge(c)
		return
	}
	c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "请选择应用 Logo 图片", Data: nil})
}

func writeOAuthClientLogoTooLarge(c *gin.Context) {
	c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "应用 Logo 大小不能超过 1MB", Data: nil})
}

func writeOAuthClientLogoError(c *gin.Context, err error, fallback string) {
	audit.Error(c, err)
	switch {
	case errors.Is(err, common.ErrOAuthClientNotFound):
		c.JSON(http.StatusNotFound, ecode.Response[any]{Code: ecode.NotFound, Message: "平台不存在", Data: nil})
	case errors.Is(err, common.ErrLogoStorageUnavailable):
		c.JSON(http.StatusServiceUnavailable, ecode.Response[any]{Code: ecode.ServiceUnavailable, Message: "应用 Logo 服务暂未配置", Data: nil})
	default:
		c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: fallback, Data: nil})
	}
}

func writeOAuthClientError(c *gin.Context, err error, fallback string) {
	audit.Error(c, err)
	switch {
	case errors.Is(err, common.ErrInvalidOAuthClient):
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "平台参数无效", Data: nil})
	case errors.Is(err, common.ErrOAuthClientExists):
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "Client ID 已存在", Data: nil})
	case errors.Is(err, common.ErrOAuthClientNotFound):
		c.JSON(http.StatusNotFound, ecode.Response[any]{Code: ecode.NotFound, Message: "平台不存在", Data: nil})
	default:
		c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: fallback, Data: nil})
	}
}
