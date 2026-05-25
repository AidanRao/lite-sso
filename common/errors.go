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
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidOTP         = errors.New("invalid otp")
	ErrInvalidCaptcha     = errors.New("invalid captcha")
	ErrRateLimited        = errors.New("rate limited")
	ErrEmailNotSent       = errors.New("email not sent")
	ErrInvalidRedirect    = errors.New("invalid redirect")
)

// OAuth related errors
var (
	ErrInvalidProvider          = errors.New("invalid provider")
	ErrProviderAuthFailed       = errors.New("provider authentication failed")
	ErrThirdPartyAlreadyBound   = errors.New("third party already bound")
	ErrThirdPartyBoundToAnother = errors.New("third party bound to another user")
)

// QR Code related errors
var (
	ErrQRCodeExpired       = errors.New("qr code expired")
	ErrQRCodeInvalidStatus = errors.New("qr code invalid status")
	ErrQRCodeInvalidUser   = errors.New("qr code invalid user")
	ErrQRCodeInvalidTicket = errors.New("qr code invalid ticket")
)
