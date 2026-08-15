package auth

import (
	"context"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/conf"
	"sso-server/dal/db"
	"sso-server/dal/kv"
	"sso-server/handler/oauth2"
	"sso-server/model"
)

const verifyCodeEmailTemplateKey = "sso-verify-code-email"

// MessageSender sends a templated message to one recipient.
type MessageSender interface {
	Send(ctx context.Context, recipient string, templateKey string, variables map[string]string) error
}

type AuthService struct {
	cfg           *conf.Config
	db            *gorm.DB
	kv            kv.Store
	messageSender MessageSender
	oauth2        *oauth2.OAuth2
}

func NewAuthService(cfg *conf.Config, db *gorm.DB, kvStore kv.Store, messageSender MessageSender, oauth2Impl *oauth2.OAuth2) *AuthService {
	return &AuthService{
		cfg:           cfg,
		db:            db,
		kv:            kvStore,
		messageSender: messageSender,
		oauth2:        oauth2Impl,
	}
}

func (s *AuthService) SendEmailOTP(ctx context.Context, email string, captchaID string, captchaAnswer string) (string, error) {
	log.Printf("SendEmailOTP: email=%s, captchaID=%s", email, captchaID)

	if ok, err := s.verifyCaptcha(ctx, captchaID, captchaAnswer); err != nil || !ok {
		log.Printf("SendEmailOTP: invalid captcha, err=%v, ok=%v", err, ok)
		return "", common.ErrInvalidCaptcha
	}

	ok, err := s.kv.SetNX(ctx, kv.KeyRateLimitEmail(email), "1", time.Minute)
	if err != nil {
		return "", err
	}
	if !ok {
		log.Printf("SendEmailOTP: rate limited for email=%s", email)
		return "", common.ErrRateLimited
	}

	otp, err := s.emailOTP()
	if err != nil {
		log.Printf("SendEmailOTP: failed to generate OTP, err=%v", err)
		return "", err
	}
	if err := s.kv.Set(ctx, kv.KeyOTP(email), otp, 5*time.Minute); err != nil {
		log.Printf("SendEmailOTP: failed to set OTP, err=%v", err)
		return "", err
	}

	if s.skipMessageSend() {
		log.Printf("SendEmailOTP: skipping message send in local mode")
		return "", nil
	}

	if s.messageSender == nil {
		return "", common.ErrMessageNotSent
	}

	if err := s.messageSender.Send(ctx, email, verifyCodeEmailTemplateKey, map[string]string{"code": otp}); err != nil {
		log.Printf("SendEmailOTP: failed to send message, err=%v", err)
		return "", err
	}

	log.Printf("SendEmailOTP: message accepted for %s", email)
	return "", nil
}

func (s *AuthService) skipMessageSend() bool {
	return s.cfg != nil && conf.GetEnvironmentName() == string(conf.EnvLocal) && s.cfg.Dev.SkipSendMessage
}

func (s *AuthService) emailOTP() (string, error) {
	if s.useFixedEmailOTP() {
		return strings.TrimSpace(s.cfg.Dev.FixedEmailOTP), nil
	}
	return GenerateNumericOTP(6)
}

func (s *AuthService) useFixedEmailOTP() bool {
	return s.cfg != nil && conf.GetEnvironmentName() == string(conf.EnvLocal) && strings.TrimSpace(s.cfg.Dev.FixedEmailOTP) != ""
}

func (s *AuthService) verifyCaptcha(ctx context.Context, captchaID string, captchaAnswer string) (bool, error) {
	val, err := s.kv.Get(ctx, kv.KeyCaptcha(captchaID))
	if err != nil {
		return false, err
	}
	if !strings.EqualFold(strings.TrimSpace(val), strings.TrimSpace(captchaAnswer)) {
		return false, nil
	}
	_ = s.kv.Del(ctx, kv.KeyCaptcha(captchaID))
	return true, nil
}

func (s *AuthService) verifyOTP(ctx context.Context, email string, otp string) (bool, error) {
	val, err := s.kv.Get(ctx, kv.KeyOTP(email))
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(val) != strings.TrimSpace(otp) {
		return false, nil
	}
	_ = s.kv.Del(ctx, kv.KeyOTP(email))
	return true, nil
}

// LoginWithEmailOTP authenticates a user with email and OTP
func (s *AuthService) LoginWithEmailOTP(ctx context.Context, email, otp string) (*model.User, error) {
	// Verify OTP
	if ok, err := s.verifyOTP(ctx, email, otp); err != nil || !ok {
		return nil, common.ErrInvalidOTP
	}

	// Find user by email
	userRepo := db.NewUserRepository(s.db)
	user, err := userRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, common.ErrInvalidCredentials
	}

	if !user.IsActive {
		return nil, common.ErrUserInactive
	}

	return user, nil
}
