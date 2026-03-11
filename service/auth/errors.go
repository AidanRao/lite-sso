package auth

import "errors"

var (
	ErrRateLimited         = errors.New("rate limited")
	ErrInvalidCaptcha      = errors.New("invalid captcha")
	ErrInvalidOTP          = errors.New("invalid otp")
	ErrEmailExists         = errors.New("email already exists")
	ErrUsernameExists      = errors.New("username already exists")
	ErrEmailNotSent        = errors.New("email not sent")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrUserInactive        = errors.New("user inactive")
	ErrQRCodeExpired       = errors.New("qr code expired")
	ErrQRCodeInvalidStatus = errors.New("qr code invalid status")
	ErrQRCodeInvalidUser   = errors.New("qr code invalid user")
)
