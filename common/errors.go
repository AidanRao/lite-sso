package common

import "errors"

// User related errors
var (
	ErrUserNotFound   = errors.New("user not found")
	ErrEmailExists    = errors.New("email already exists")
	ErrUsernameExists = errors.New("username already exists")
	ErrUserInactive   = errors.New("user inactive")
)

// Authentication related errors
var (
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrInvalidOTP          = errors.New("invalid otp")
	ErrOTPExpired          = errors.New("otp challenge expired")
	ErrOTPAttemptsExceeded = errors.New("otp challenge attempts exceeded")
	ErrChallengeInvalid    = errors.New("invalid login challenge")
	ErrInvalidCaptcha      = errors.New("invalid captcha")
	ErrRateLimited         = errors.New("rate limited")
	ErrCaptchaRequired     = errors.New("captcha required")
	ErrSessionRevoked      = errors.New("session revoked")
	ErrRefreshTokenInvalid = errors.New("invalid refresh token")
	ErrAccountLocked       = errors.New("account locked")
	ErrMessageNotSent      = errors.New("message not sent")
	ErrInvalidRedirect     = errors.New("invalid redirect")
)

type AccountLockedError struct {
	RetryAfterSeconds int `json:"retry_after_seconds"`
}

type RateLimitedError struct {
	RetryAfterSeconds int
	Reason            string
}

func (e RateLimitedError) Error() string { return ErrRateLimited.Error() }
func (e RateLimitedError) Unwrap() error { return ErrRateLimited }

type CaptchaRequiredError struct {
	Reason string
}

func (e CaptchaRequiredError) Error() string { return ErrCaptchaRequired.Error() }
func (e CaptchaRequiredError) Unwrap() error { return ErrCaptchaRequired }

func (e AccountLockedError) Error() string {
	return ErrAccountLocked.Error()
}

func (e AccountLockedError) Unwrap() error {
	return ErrAccountLocked
}

// OAuth related errors
var (
	ErrInvalidProvider          = errors.New("invalid provider")
	ErrProviderAuthFailed       = errors.New("provider authentication failed")
	ErrThirdPartyAlreadyBound   = errors.New("third party already bound")
	ErrThirdPartyBoundToAnother = errors.New("third party bound to another user")
	ErrThirdPartyNotBound       = errors.New("third party not bound")
	ErrEmailRequiredForUnbind   = errors.New("email required to unbind third party")
	ErrOAuthClientExists        = errors.New("oauth client already exists")
	ErrOAuthClientNotFound      = errors.New("oauth client not found")
	ErrInvalidOAuthClient       = errors.New("invalid oauth client")
)

// QR Code related errors
var (
	ErrQRCodeExpired       = errors.New("qr code expired")
	ErrQRCodeInvalidStatus = errors.New("qr code invalid status")
	ErrQRCodeInvalidUser   = errors.New("qr code invalid user")
	ErrQRCodeInvalidTicket = errors.New("qr code invalid ticket")
)
