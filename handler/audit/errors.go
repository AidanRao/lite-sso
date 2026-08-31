package audit

import (
	"errors"

	"github.com/gin-gonic/gin"

	"sso-server/common"
)

var safeErrors = []struct {
	err  error
	code string
}{
	{err: common.ErrUserNotFound, code: "USER_NOT_FOUND"},
	{err: common.ErrEmailExists, code: "EMAIL_EXISTS"},
	{err: common.ErrUsernameExists, code: "USERNAME_EXISTS"},
	{err: common.ErrUserInactive, code: "USER_INACTIVE"},
	{err: common.ErrAvatarStorageUnavailable, code: "AVATAR_STORAGE_UNAVAILABLE"},
	{err: common.ErrEmailAlreadyAdded, code: "EMAIL_ALREADY_ADDED"},
	{err: common.ErrEmailInvalid, code: "EMAIL_INVALID"},
	{err: common.ErrEmailLimitReached, code: "EMAIL_LIMIT_REACHED"},
	{err: common.ErrEmailNotFound, code: "EMAIL_NOT_FOUND"},
	{err: common.ErrEmailNotVerified, code: "EMAIL_NOT_VERIFIED"},
	{err: common.ErrPrimaryEmailDelete, code: "PRIMARY_EMAIL_DELETE"},
	{err: common.ErrEmailVerificationInvalid, code: "EMAIL_VERIFICATION_INVALID"},
	{err: common.ErrEmailVerificationExpired, code: "EMAIL_VERIFICATION_EXPIRED"},
	{err: common.ErrEmailVerificationResend, code: "EMAIL_VERIFICATION_RESEND"},
	{err: common.ErrLastLoginMethod, code: "LAST_LOGIN_METHOD"},
	{err: common.ErrInvalidCredentials, code: "INVALID_CREDENTIALS"},
	{err: common.ErrCurrentPasswordInvalid, code: "CURRENT_PASSWORD_INVALID"},
	{err: common.ErrPasswordNotSet, code: "PASSWORD_NOT_SET"},
	{err: common.ErrPasswordLengthInvalid, code: "PASSWORD_LENGTH_INVALID"},
	{err: common.ErrPasswordLetterRequired, code: "PASSWORD_LETTER_REQUIRED"},
	{err: common.ErrPasswordDigitRequired, code: "PASSWORD_DIGIT_REQUIRED"},
	{err: common.ErrInvalidOTP, code: "INVALID_OTP"},
	{err: common.ErrOTPExpired, code: "OTP_EXPIRED"},
	{err: common.ErrOTPAttemptsExceeded, code: "OTP_ATTEMPTS_EXCEEDED"},
	{err: common.ErrChallengeInvalid, code: "CHALLENGE_INVALID"},
	{err: common.ErrInvalidCaptcha, code: "INVALID_CAPTCHA"},
	{err: common.ErrRateLimited, code: "RATE_LIMITED"},
	{err: common.ErrCaptchaRequired, code: "CAPTCHA_REQUIRED"},
	{err: common.ErrSessionRevoked, code: "SESSION_REVOKED"},
	{err: common.ErrDeviceNotFound, code: "DEVICE_NOT_FOUND"},
	{err: common.ErrCurrentDevice, code: "CURRENT_DEVICE"},
	{err: common.ErrRefreshTokenInvalid, code: "REFRESH_TOKEN_INVALID"},
	{err: common.ErrAccountLocked, code: "ACCOUNT_LOCKED"},
	{err: common.ErrMessageNotSent, code: "MESSAGE_NOT_SENT"},
	{err: common.ErrInvalidRedirect, code: "INVALID_REDIRECT"},
	{err: common.ErrPasskeyRequired, code: "PASSKEY_REQUIRED"},
	{err: common.ErrEmailRequiredForPasskey, code: "EMAIL_REQUIRED_FOR_PASSKEY"},
	{err: common.ErrWebAuthnCeremonyInvalid, code: "WEBAUTHN_CEREMONY_INVALID"},
	{err: common.ErrReauthRequired, code: "REAUTH_REQUIRED"},
	{err: common.ErrReauthTokenInvalid, code: "REAUTH_TOKEN_INVALID"},
	{err: common.ErrReauthMethodUnavailable, code: "REAUTH_METHOD_UNAVAILABLE"},
	{err: common.ErrPasskeyNotFound, code: "PASSKEY_NOT_FOUND"},
	{err: common.ErrPasskeyNameInvalid, code: "PASSKEY_NAME_INVALID"},
	{err: common.ErrPasskeyCloneWarning, code: "PASSKEY_CLONE_WARNING"},
	{err: common.ErrInvalidProvider, code: "INVALID_PROVIDER"},
	{err: common.ErrProviderAuthFailed, code: "PROVIDER_AUTH_FAILED"},
	{err: common.ErrThirdPartyAlreadyBound, code: "THIRD_PARTY_ALREADY_BOUND"},
	{err: common.ErrThirdPartyBoundToAnother, code: "THIRD_PARTY_BOUND_TO_ANOTHER"},
	{err: common.ErrThirdPartyNotBound, code: "THIRD_PARTY_NOT_BOUND"},
	{err: common.ErrThirdPartyBindingNotFound, code: "THIRD_PARTY_BINDING_NOT_FOUND"},
	{err: common.ErrOAuthClientExists, code: "OAUTH_CLIENT_EXISTS"},
	{err: common.ErrOAuthClientNotFound, code: "OAUTH_CLIENT_NOT_FOUND"},
	{err: common.ErrInvalidOAuthClient, code: "INVALID_OAUTH_CLIENT"},
	{err: common.ErrLogoStorageUnavailable, code: "LOGO_STORAGE_UNAVAILABLE"},
	{err: common.ErrQRCodeExpired, code: "QR_CODE_EXPIRED"},
	{err: common.ErrQRCodeInvalidStatus, code: "QR_CODE_INVALID_STATUS"},
	{err: common.ErrQRCodeInvalidUser, code: "QR_CODE_INVALID_USER"},
	{err: common.ErrQRCodeInvalidTicket, code: "QR_CODE_INVALID_TICKET"},
}

// Error records only a known reason code; raw errors may contain credentials or SQL.
func Error(c *gin.Context, err error) {
	if err == nil || current(c) == nil {
		return
	}
	for _, item := range safeErrors {
		if errors.Is(err, item.err) {
			Failure(c, item.code)
			return
		}
	}
	Failure(c, "OPERATION_FAILED")
}
