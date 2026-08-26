package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"sso-server/common"
	"sso-server/dal/kv"
)

//go:embed challenge_verify.lua
var challengeScripts embed.FS

type ChallengePurpose string

const (
	ChallengePurposeLogin         ChallengePurpose = "LOGIN"
	ChallengePurposeRegister      ChallengePurpose = "REGISTER"
	ChallengePurposePasswordReset ChallengePurpose = "PASSWORD_RESET"
)

type loginChallenge struct {
	Email          string           `json:"email"`
	DeviceID       string           `json:"device_id"`
	CodeMAC        string           `json:"code_mac"`
	Purpose        ChallengePurpose `json:"purpose"`
	FailedAttempts int              `json:"failed_attempts"`
	Status         string           `json:"status"`
	CreatedAt      int64            `json:"created_at"`
	ExpiresAt      int64            `json:"expires_at"`
}

type ChallengeResult struct {
	ChallengeID string
	Email       string
	ExpiresIn   int
	ResendAfter int
}

var challengeFallbackMu sync.Mutex

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func identifierHash(secret string, value string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

func otpMAC(secret string, challengeID string, code string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(challengeID + ":" + code))
	return hex.EncodeToString(mac.Sum(nil))
}

// createChallengeWithCode creates a Challenge while keeping the OTP only in the
// caller's local scope. The Redis value contains only its HMAC.
func (s *AuthService) createChallengeWithCode(ctx context.Context, email string, deviceID string, purpose ChallengePurpose, code string) (*ChallengeResult, error) {
	if s.security == nil {
		return nil, errors.New("security store is nil")
	}
	challengeID := "chl_" + uuid.NewString()
	now := time.Now()
	expiresIn := s.authOTPExpire()
	challenge := loginChallenge{
		Email:     normalizeEmail(email),
		DeviceID:  deviceID,
		CodeMAC:   otpMAC(s.authSecret(), challengeID, code),
		Purpose:   purpose,
		Status:    "ACTIVE",
		CreatedAt: now.Unix(),
		ExpiresAt: now.Add(expiresIn).Unix(),
	}
	data, err := json.Marshal(challenge)
	if err != nil {
		return nil, err
	}
	if err := s.security.Set(ctx, kv.KeyChallenge(challengeID), string(data), expiresIn); err != nil {
		return nil, err
	}
	return &ChallengeResult{ChallengeID: challengeID, Email: challenge.Email, ExpiresIn: int(expiresIn / time.Second), ResendAfter: 60}, nil
}

func (s *AuthService) verifyChallenge(ctx context.Context, challengeID string, code string, deviceID string, purpose ChallengePurpose) (string, error) {
	if s.security == nil {
		return "", errors.New("security store is nil")
	}
	key := kv.KeyChallenge(challengeID)
	mac := otpMAC(s.authSecret(), challengeID, strings.TrimSpace(code))
	scriptBytes, err := challengeScripts.ReadFile("challenge_verify.lua")
	if err != nil {
		return "", err
	}
	result, err := s.security.Eval(ctx, string(scriptBytes), []string{key}, deviceID, string(purpose), time.Now().Unix(), mac, s.authOTPMaxAttempts())
	if err == nil {
		return s.parseChallengeResult(ctx, key, result)
	}
	if !errors.Is(err, kv.ErrScriptUnsupported) {
		return "", err
	}

	challengeFallbackMu.Lock()
	defer challengeFallbackMu.Unlock()
	return s.verifyChallengeFallback(ctx, key, challengeID, code, deviceID, purpose)
}

func (s *AuthService) parseChallengeResult(ctx context.Context, key string, result interface{}) (string, error) {
	values, ok := result.([]interface{})
	if !ok || len(values) != 2 {
		return "", common.ErrChallengeInvalid
	}
	statusCode := fmt.Sprint(values[0])
	reason := fmt.Sprint(values[1])
	if statusCode != "1" {
		switch reason {
		case "expired":
			return "", common.ErrOTPExpired
		case "attempts":
			return "", common.ErrOTPAttemptsExceeded
		default:
			return "", common.ErrInvalidOTP
		}
	}
	data, err := s.security.Get(ctx, key)
	if err != nil {
		return "", common.ErrChallengeInvalid
	}
	var challenge loginChallenge
	if err := json.Unmarshal([]byte(data), &challenge); err != nil {
		return "", err
	}
	return challenge.Email, nil
}

func (s *AuthService) verifyChallengeFallback(ctx context.Context, key string, challengeID string, code string, deviceID string, purpose ChallengePurpose) (string, error) {
	data, err := s.security.Get(ctx, key)
	if err != nil {
		return "", common.ErrChallengeInvalid
	}
	var challenge loginChallenge
	if err := json.Unmarshal([]byte(data), &challenge); err != nil {
		return "", err
	}
	if challenge.DeviceID != deviceID || challenge.Purpose != purpose || challenge.Status != "ACTIVE" {
		return "", common.ErrChallengeInvalid
	}
	if time.Now().Unix() >= challenge.ExpiresAt {
		challenge.Status = "EXPIRED"
		return "", common.ErrOTPExpired
	}
	expected := otpMAC(s.authSecret(), challengeID, strings.TrimSpace(code))
	if subtle.ConstantTimeCompare([]byte(expected), []byte(challenge.CodeMAC)) != 1 {
		challenge.FailedAttempts++
		if challenge.FailedAttempts >= s.authOTPMaxAttempts() {
			challenge.Status = "FAILED"
		}
		updated, _ := json.Marshal(challenge)
		_ = s.security.Set(ctx, key, string(updated), time.Until(time.Unix(challenge.ExpiresAt, 0)))
		if challenge.Status == "FAILED" {
			return "", common.ErrOTPAttemptsExceeded
		}
		return "", common.ErrInvalidOTP
	}
	challenge.Status = "VERIFIED"
	updated, _ := json.Marshal(challenge)
	if err := s.security.Set(ctx, key, string(updated), time.Until(time.Unix(challenge.ExpiresAt, 0))); err != nil {
		return "", err
	}
	return challenge.Email, nil
}

func (s *AuthService) authSecret() string {
	if s.cfg != nil && strings.TrimSpace(s.cfg.Auth.OTPSecret) != "" {
		return s.cfg.Auth.OTPSecret
	}
	return "test-only-otp-secret"
}

func (s *AuthService) authOTPExpire() time.Duration {
	if s.cfg != nil && s.cfg.Auth.OTPExpire > 0 {
		return s.cfg.Auth.OTPExpire
	}
	return 5 * time.Minute
}

func (s *AuthService) authOTPMaxAttempts() int {
	if s.cfg != nil && s.cfg.Auth.OTPMaxAttempts > 0 {
		return s.cfg.Auth.OTPMaxAttempts
	}
	return 5
}
