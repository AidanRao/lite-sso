package auth

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/conf"
	"sso-server/dal/db"
	"sso-server/dal/kv"
	"sso-server/handler/oauth2"
	"sso-server/model"
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

	otp, err := GenerateNumericOTP(6)
	if err != nil {
		log.Printf("SendEmailOTP: failed to generate OTP, err=%v", err)
		return "", err
	}
	if err := s.kv.Set(ctx, kv.KeyOTP(email), otp, 5*time.Minute); err != nil {
		log.Printf("SendEmailOTP: failed to set OTP, err=%v", err)
		return "", err
	}

	if s.mailer == nil {
		if s.cfg != nil && s.cfg.Dev.EchoOTP {
			return otp, nil
		}
		return "", mailer.ErrNotConfigured
	}

	// In development mode with EchoOTP, skip template loading
	if s.cfg != nil && s.cfg.Dev.EchoOTP {
		log.Printf("SendEmailOTP: returning OTP in dev mode, otp=%s", otp)
		return otp, nil
	}

	// Load email templates
	templatePath := "templates/mail"
	txtPath := filepath.Join(templatePath, "otp.txt")
	htmlPath := filepath.Join(templatePath, "otp.html")

	// Load text template
	txtContent, err := os.ReadFile(txtPath)
	if err != nil {
		log.Printf("SendEmailOTP: failed to load text template, path=%s, err=%v", txtPath, err)
		return "", fmt.Errorf("failed to load text template: %w", err)
	}
	txtTemplate, err := template.New("otp").Parse(string(txtContent))
	if err != nil {
		log.Printf("SendEmailOTP: failed to parse text template, err=%v", err)
		return "", fmt.Errorf("failed to parse text template: %w", err)
	}

	// Load HTML template
	htmlContent, err := os.ReadFile(htmlPath)
	if err != nil {
		log.Printf("SendEmailOTP: failed to load HTML template, path=%s, err=%v", htmlPath, err)
		return "", fmt.Errorf("failed to load HTML template: %w", err)
	}
	htmlTemplate, err := template.New("otp").Parse(string(htmlContent))
	if err != nil {
		log.Printf("SendEmailOTP: failed to parse HTML template, err=%v", err)
		return "", fmt.Errorf("failed to parse HTML template: %w", err)
	}

	// Template data
	data := struct {
		OTP string
	}{
		OTP: otp,
	}

	// Execute text template
	var textBody strings.Builder
	if err := txtTemplate.Execute(&textBody, data); err != nil {
		log.Printf("SendEmailOTP: failed to execute text template, err=%v", err)
		return "", fmt.Errorf("failed to execute text template: %w", err)
	}

	// Execute HTML template
	var htmlBody strings.Builder
	if err := htmlTemplate.Execute(&htmlBody, data); err != nil {
		log.Printf("SendEmailOTP: failed to execute HTML template, err=%v", err)
		return "", fmt.Errorf("failed to execute HTML template: %w", err)
	}

	// Send email
	if err := s.mailer.SendEmail(ctx, email, "Your verification code", textBody.String(), htmlBody.String()); err != nil {
		log.Printf("SendEmailOTP: failed to send email, err=%v", err)
		if errors.Is(err, mailer.ErrNotConfigured) && s.cfg != nil && s.cfg.Dev.EchoOTP {
			log.Printf("SendEmailOTP: returning OTP in dev mode, otp=%s", otp)
			return otp, nil
		}
		return "", err
	}

	log.Printf("SendEmailOTP: email sent successfully to %s", email)
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
func (s *AuthService) LoginWithEmailOTP(ctx context.Context, r *http.Request, email, otp string) (*model.User, map[string]interface{}, error) {
	// Verify OTP
	if ok, err := s.verifyOTP(ctx, email, otp); err != nil || !ok {
		return nil, nil, common.ErrInvalidOTP
	}

	// Find user by email
	userRepo := db.NewUserRepository(s.db)
	user, err := userRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, nil, common.ErrInvalidCredentials
	}

	if !user.IsActive {
		return nil, nil, common.ErrUserInactive
	}

	if s.oauth2 == nil || r == nil {
		return user, nil, nil
	}

	// Issue token
	tokenData, err := s.oauth2.IssueTokenForUser(ctx, r, user.ID)
	if err != nil {
		return nil, nil, err
	}

	return user, tokenData, nil
}
