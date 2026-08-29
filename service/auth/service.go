package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/conf"
	"sso-server/dal/db"
	"sso-server/dal/kv"
	"sso-server/model"
)

const verifyCodeEmailTemplateKey = "sso-verify-code-email"

// MessageSender sends a templated message to one target.
type MessageSender interface {
	Send(ctx context.Context, target string, templateKey string, variables map[string]string) error
}

// AuthService coordinates authentication flows and their shared security controls.
type AuthService struct {
	cfg           *conf.Config
	db            *gorm.DB
	kv            kv.Store
	security      kv.SecurityStore
	messageSender MessageSender
	oauth2        OAuthTokenIssuer
	guard         *loginGuard
}

// OAuthTokenIssuer is retained for the external OAuth2 protocol boundary.
type OAuthTokenIssuer interface {
	IssueTokenForUser(ctx context.Context, request *http.Request, userID string) (map[string]interface{}, error)
}

func NewAuthService(cfg *conf.Config, database *gorm.DB, kvStore kv.Store, messageSender MessageSender, oauth2Impl OAuthTokenIssuer) *AuthService {
	if kvStore == nil {
		kvStore = kv.NewMemoryStore()
	}
	service := &AuthService{
		cfg:           cfg,
		db:            database,
		kv:            kvStore,
		security:      kv.NewSecurityStore(kvStore),
		messageSender: messageSender,
		oauth2:        oauth2Impl,
	}
	service.guard = newLoginGuard(service.security, service)
	return service
}

type OTPRequestContext struct {
	DeviceID string
	IP       string
}

// SendEmailOTP creates an independent purpose-bound challenge and sends its code.
func (s *AuthService) SendEmailOTP(ctx context.Context, email string, captchaID string, captchaAnswer string, requestContext OTPRequestContext, purpose ChallengePurpose) (*ChallengeResult, error) {
	email = normalizeEmail(email)
	if ok, err := s.verifyCaptcha(ctx, captchaID, captchaAnswer); err != nil || !ok {
		return nil, common.ErrInvalidCaptcha
	}
	if err := s.guard.allowOTPSend(ctx, email, requestContext.DeviceID, requestContext.IP); err != nil {
		return nil, err
	}

	code, err := s.emailOTP()
	if err != nil {
		return nil, err
	}
	challenge, err := s.createChallengeWithCode(ctx, email, requestContext.DeviceID, purpose, code)
	if err != nil {
		return nil, err
	}

	if s.skipMessageSend() {
		return challenge, nil
	}
	if s.messageSender == nil {
		return nil, common.ErrMessageNotSent
	}
	if err := s.messageSender.Send(ctx, email, verifyCodeEmailTemplateKey, map[string]string{"code": code}); err != nil {
		return nil, err
	}
	return challenge, nil
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
	return true, s.kv.Del(ctx, kv.KeyCaptcha(captchaID))
}

// VerifyCaptcha consumes one image CAPTCHA when the caller has supplied it.
func (s *AuthService) VerifyCaptcha(ctx context.Context, captchaID string, captchaAnswer string) (bool, error) {
	return s.verifyCaptcha(ctx, captchaID, captchaAnswer)
}

// VerifyChallengeForPurpose verifies a challenge and returns the bound email.
func (s *AuthService) VerifyChallengeForPurpose(ctx context.Context, challengeID string, code string, deviceID string, purpose ChallengePurpose) (string, error) {
	return s.verifyChallenge(ctx, challengeID, code, deviceID, purpose)
}

// EmailOTPLoginContext carries the device and network context for OTP login.
type EmailOTPLoginContext struct {
	DeviceID     string
	IP           string
	CaptchaValid bool
}

// OTPVerifyContext carries request attributes used by OTP verification limits.
type OTPVerifyContext struct {
	DeviceID string
	IP       string
}

// VerifyEmailOTPForPurpose verifies a purpose-bound challenge with the shared
// device, email, and IP controls used by email login.
func (s *AuthService) VerifyEmailOTPForPurpose(ctx context.Context, challengeID string, otp string, verifyContext OTPVerifyContext, purpose ChallengePurpose) (string, error) {
	challengeEmail, err := s.challengeEmail(ctx, challengeID)
	if err != nil {
		return "", err
	}
	if err := s.guard.allowOTPVerify(ctx, challengeEmail, verifyContext.DeviceID, verifyContext.IP); err != nil {
		return "", err
	}
	email, err := s.verifyChallenge(ctx, challengeID, otp, verifyContext.DeviceID, purpose)
	if err != nil {
		if recordErr := s.guard.recordOTPFailure(ctx, challengeEmail, verifyContext.DeviceID, verifyContext.IP); recordErr != nil {
			return "", recordErr
		}
		return "", err
	}
	return email, nil
}

// LoginWithEmailOTP authenticates a login challenge without accepting an email
// from the client, preventing a challenge/email mix-up.
func (s *AuthService) LoginWithEmailOTP(ctx context.Context, challengeID string, otp string, loginContext EmailOTPLoginContext) (*model.User, error) {
	email, err := s.VerifyEmailOTPForPurpose(ctx, challengeID, otp, OTPVerifyContext{DeviceID: loginContext.DeviceID, IP: loginContext.IP}, ChallengePurposeLogin)
	if err != nil {
		return nil, err
	}

	user, err := db.NewUserRepository(s.db).FindByEmail(ctx, email)
	if err != nil {
		return nil, common.ErrInvalidCredentials
	}
	if !user.IsActive {
		return nil, common.ErrUserInactive
	}
	return user, nil
}

func (s *AuthService) challengeEmail(ctx context.Context, challengeID string) (string, error) {
	data, err := s.security.Get(ctx, kv.KeyChallenge(challengeID))
	if err != nil {
		return "", common.ErrChallengeInvalid
	}
	var challenge loginChallenge
	if err := json.Unmarshal([]byte(data), &challenge); err != nil {
		return "", common.ErrChallengeInvalid
	}
	if challenge.Status != "ACTIVE" {
		return "", common.ErrChallengeInvalid
	}
	return challenge.Email, nil
}

func (s *AuthService) registerUser(ctx context.Context, email string, password string, username *string, challengeID string, code string, deviceID string) (*model.User, error) {
	if err := s.validatePassword(password); err != nil {
		return nil, err
	}
	challengeEmail, err := s.verifyChallenge(ctx, challengeID, code, deviceID, ChallengePurposeRegister)
	if err != nil || challengeEmail != normalizeEmail(email) {
		return nil, common.ErrInvalidOTP
	}
	userRepo := db.NewUserRepository(s.db)
	if exists, err := userRepo.ExistsEmail(ctx, normalizeEmail(email)); err != nil {
		return nil, err
	} else if exists {
		return nil, common.ErrEmailExists
	}
	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	userEmail := normalizeEmail(email)
	user := &model.User{ID: "u_" + strings.ReplaceAll(uuid.NewString(), "-", ""), Email: &userEmail, PasswordHash: &hash, IsActive: true}
	if username != nil && strings.TrimSpace(*username) != "" {
		value := strings.TrimSpace(*username)
		if exists, err := userRepo.ExistsUsername(ctx, value); err != nil {
			return nil, err
		} else if exists {
			return nil, common.ErrUsernameExists
		}
		user.Username = &value
	}
	if err := userRepo.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// RegisterWithEmailChallenge creates a password user after verifying a
// purpose-bound registration challenge.
func (s *AuthService) RegisterWithEmailChallenge(ctx context.Context, email string, password string, username *string, challengeID string, code string, deviceID string) (*model.User, error) {
	return s.registerUser(ctx, email, password, username, challengeID, code, deviceID)
}

// ResetPasswordWithEmailChallenge changes a password and revokes all sessions.
func (s *AuthService) ResetPasswordWithEmailChallenge(ctx context.Context, email string, password string, challengeID string, code string, deviceID string) error {
	if err := s.validatePassword(password); err != nil {
		return err
	}
	challengeEmail, err := s.verifyChallenge(ctx, challengeID, code, deviceID, ChallengePurposePasswordReset)
	if err != nil || challengeEmail != normalizeEmail(email) {
		return common.ErrInvalidOTP
	}
	user, err := db.NewUserRepository(s.db).FindByEmail(ctx, normalizeEmail(email))
	if err != nil {
		return common.ErrUserNotFound
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	user.PasswordHash = &hash
	if err := db.NewUserRepository(s.db).Update(ctx, user); err != nil {
		return err
	}
	return s.InvalidateAllSessions(ctx, user.ID, "password_reset")
}
