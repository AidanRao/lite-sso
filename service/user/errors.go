package user

import "errors"

var (
	ErrInvalidOTP     = errors.New("invalid otp")
	ErrEmailExists    = errors.New("email already exists")
	ErrUsernameExists = errors.New("username already exists")
	ErrUserNotFound   = errors.New("user not found")
)
