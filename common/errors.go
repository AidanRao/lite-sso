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
)

// OAuth related errors
var (
	ErrInvalidProvider    = errors.New("invalid provider")
	ErrBindingExists      = errors.New("third-party account already bound")
	ErrProviderAuthFailed = errors.New("provider authentication failed")
)

// QR Code related errors
var (
	ErrQRCodeExpired       = errors.New("qr code expired")
	ErrQRCodeInvalidStatus = errors.New("qr code invalid status")
	ErrQRCodeInvalidUser   = errors.New("qr code invalid user")
)
