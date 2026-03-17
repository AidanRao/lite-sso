package auth

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/common/ecode"
	"sso-server/conf"
	"sso-server/dal/kv"
	"sso-server/handler/oauth2"
	"sso-server/service/auth"
	"sso-server/util/captcha"
	"sso-server/util/mailer"
)

type AuthDeps struct {
	Config *conf.Config
	DB     *gorm.DB
	KV     kv.Store
	Mailer mailer.Mailer
	OAuth2 *oauth2.OAuth2
}

type AuthHandler struct {
	captcha *captcha.Service
	auth    *auth.AuthService
	cfg     *conf.Config
}

func NewAuthHandler(deps AuthDeps) *AuthHandler {
	cfg := deps.Config
	kvStore := deps.KV
	if kvStore == nil {
		kvStore = kv.NewMemoryStore()
	}

	mailerImpl := deps.Mailer
	if mailerImpl == nil && cfg != nil {
		mailerImpl = mailer.NewSMTPMailer(mailer.SMTPConfig{
			Host: cfg.Email.SMTPHost,
			Port: cfg.Email.SMTPPort,
			User: cfg.Email.SMTPUser,
			Pass: cfg.Email.SMTPPass,
			From: cfg.Email.SMTPFrom,
		})
	}

	captchaStore := captcha.NewStore(kvStore, 5*time.Minute)
	return &AuthHandler{
		captcha: captcha.NewService(captchaStore),
		auth:    auth.NewAuthService(cfg, deps.DB, kvStore, mailerImpl, deps.OAuth2),
		cfg:     cfg,
	}
}

func (h *AuthHandler) GenerateCaptcha(c *gin.Context) {
	id, pngB64, err := h.captcha.Generate()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "生成验证码失败", Data: nil})
		return
	}

	c.JSON(http.StatusOK, ecode.OKResponse(gin.H{
		"captcha_id":         id,
		"captcha_png_base64": pngB64,
	}))
}

func (h *AuthHandler) SendEmailOTP(c *gin.Context) {
	var req struct {
		Email     string `json:"email" binding:"required,email"`
		CaptchaID string `json:"captcha_id" binding:"required"`
		Captcha   string `json:"captcha" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误", Data: nil})
		return
	}

	otp, err := h.auth.SendEmailOTP(c.Request.Context(), req.Email, req.CaptchaID, req.Captcha)
	if err != nil {
		switch {
		case errors.Is(err, common.ErrInvalidCaptcha):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "验证码错误", Data: nil})
		case errors.Is(err, common.ErrRateLimited):
			c.JSON(http.StatusTooManyRequests, ecode.Response[any]{Code: ecode.TooManyRequests, Message: "请求过于频繁", Data: nil})
		case errors.Is(err, mailer.ErrNotConfigured):
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "邮件服务未配置", Data: nil})
		default:
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "发送失败", Data: nil})
		}
		return
	}

	data := gin.H{"sent": true}
	if h.cfg != nil && h.cfg.Dev.EchoOTP && otp != "" {
		data["otp"] = otp
	}
	c.JSON(http.StatusOK, ecode.OKResponse(data))
}
