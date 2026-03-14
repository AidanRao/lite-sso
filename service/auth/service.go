package auth

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/conf"
	"sso-server/dal/kv"
	"sso-server/handler/oauth2"
	"sso-server/util/mailer"
)

type AuthService struct {
	cfg    *conf.Config
	db     *gorm.DB
	kv     kv.Store
	mailer mailer.Mailer
	oauth2 *oauth2.OAuth2
}

func NewAuthService(cfg *conf.Config, db *gorm.DB, kvStore kv.Store, mailerImpl mailer.Mailer, oauth2Impl *oauth2.OAuth2) *AuthService {
	return &AuthService{
		cfg:    cfg,
		db:     db,
		kv:     kvStore,
		mailer: mailerImpl,
		oauth2: oauth2Impl,
	}
}

func (s *AuthService) SendEmailOTP(ctx context.Context, email string, captchaID string, captchaAnswer string) (string, error) {
	if ok, err := s.verifyCaptcha(ctx, captchaID, captchaAnswer); err != nil || !ok {
		return "", common.ErrInvalidCaptcha
	}

	ok, err := s.kv.SetNX(ctx, kv.KeyRateLimitEmail(email), "1", time.Minute)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", common.ErrRateLimited
	}

	otp, err := GenerateNumericOTP(6)
	if err != nil {
		return "", err
	}
	if err := s.kv.Set(ctx, kv.KeyOTP(email), otp, 5*time.Minute); err != nil {
		return "", err
	}

	if s.mailer == nil {
		if s.cfg != nil && s.cfg.Dev.EchoOTP {
			return otp, nil
		}
		return "", mailer.ErrNotConfigured
	}

	if err := s.mailer.SendOTP(ctx, email, otp); err != nil {
		if err == mailer.ErrNotConfigured && s.cfg != nil && s.cfg.Dev.EchoOTP {
			return otp, nil
		}
		return "", err
	}

	if s.cfg != nil && s.cfg.Dev.EchoOTP {
		return otp, nil
	}
	return "", nil
}


func (s *AuthService) verifyCaptcha(ctx context.Context, captchaID string, captchaAnswer string) (bool, error) {
	val, err := s.kv.Get(ctx, kv.KeyCaptcha(captchaID))
	if err != nil {
		return false, err
	}
	if strings.ToLower(strings.TrimSpace(val)) != strings.ToLower(strings.TrimSpace(captchaAnswer)) {
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
