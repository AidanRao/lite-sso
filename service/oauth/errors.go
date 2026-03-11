package oauth

import "errors"

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidProvider    = errors.New("invalid provider")
	ErrBindingExists      = errors.New("third-party account already bound")
	ErrProviderAuthFailed = errors.New("provider authentication failed")
)
