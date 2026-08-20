package auth

import (
	"errors"
	"net/http"
	"strconv"
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
)

type AuthDeps struct {
	Config        *conf.Config
	DB            *gorm.DB
	KV            kv.Store
	MessageSender auth.MessageSender
	OAuth2        *oauth2.OAuth2
}

type AuthHandler struct {
	captcha *captcha.Service
	auth    *auth.AuthService
	db      *gorm.DB
	kv      kv.Store
}

func NewAuthHandler(deps AuthDeps) *AuthHandler {
	cfg := deps.Config
	kvStore := deps.KV
	if kvStore == nil {
		kvStore = kv.NewMemoryStore()
	}

	captchaStore := captcha.NewStore(kvStore, 5*time.Minute)
	return &AuthHandler{
		captcha: captcha.NewService(captchaStore),
		auth:    auth.NewAuthService(cfg, deps.DB, kvStore, deps.MessageSender, deps.OAuth2),
		db:      deps.DB,
		kv:      kvStore,
	}
}

// Service exposes the shared authentication service to middleware wiring.
func (h *AuthHandler) Service() *auth.AuthService {
	return h.auth
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
		Purpose   string `json:"purpose" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "参数错误", Data: nil})
		return
	}

	deviceID, isNewDevice := auth.EnsureDeviceID(c.Request)
	if isNewDevice {
		WriteDeviceCookie(c, deviceID)
	}
	purpose := auth.ChallengePurpose(req.Purpose)
	if purpose != auth.ChallengePurposeLogin && purpose != auth.ChallengePurposeRegister && purpose != auth.ChallengePurposePasswordReset {
		c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "用途无效", Data: nil})
		return
	}
	challenge, err := h.auth.SendEmailOTP(c.Request.Context(), req.Email, req.CaptchaID, req.Captcha, auth.OTPRequestContext{
		DeviceID: deviceID,
		IP:       auth.RequestIP(c.Request),
	}, purpose)
	if err != nil {
		switch {
		case errors.Is(err, common.ErrInvalidCaptcha):
			c.JSON(http.StatusBadRequest, ecode.Response[any]{Code: ecode.BadRequest, Message: "验证码错误", Data: nil})
		case errors.Is(err, common.ErrRateLimited):
			writeRateLimited(c, err, "请求过于频繁")
		default:
			c.JSON(http.StatusInternalServerError, ecode.Response[any]{Code: ecode.InternalServer, Message: "发送失败", Data: nil})
		}
		return
	}

	data := gin.H{
		"sent":         true,
		"challenge_id": challenge.ChallengeID,
		"expires_in":   challenge.ExpiresIn,
		"resend_after": challenge.ResendAfter,
	}
	c.JSON(http.StatusOK, ecode.OKResponse(data))
}

func WriteDeviceCookie(c *gin.Context, deviceID string) {
	auth.WriteDeviceCookie(c.Writer, deviceID, conf.GetEnv() == conf.EnvProd)
}

func writeRateLimited(c *gin.Context, err error, message string) {
	retryAfter := 1
	var rateError common.RateLimitedError
	if errors.As(err, &rateError) && rateError.RetryAfterSeconds > 0 {
		retryAfter = rateError.RetryAfterSeconds
	}
	c.Header("Retry-After", strconv.Itoa(retryAfter))
	c.JSON(http.StatusTooManyRequests, ecode.Response[any]{Code: ecode.TooManyRequests, Message: message, Data: gin.H{"retry_after_seconds": retryAfter}})
}
