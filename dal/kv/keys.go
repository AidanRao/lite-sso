package kv

import "fmt"

func KeyCaptcha(captchaID string) string {
	return fmt.Sprintf("captcha:%s", captchaID)
}

func KeyOTP(email string) string {
	return fmt.Sprintf("otp:%s", email)
}

func KeyChallenge(challengeID string) string {
	return fmt.Sprintf("auth:challenge:%s", challengeID)
}

func KeyAuthRateLimit(scope string, value string) string {
	return fmt.Sprintf("auth:rate:%s:%s", scope, value)
}

func KeyAuthFailure(scope string, value string) string {
	return fmt.Sprintf("auth:fail:%s:%s", scope, value)
}

func KeyAuthDistinctAccounts(scope string, value string) string {
	return fmt.Sprintf("auth:accounts:%s:%s", scope, value)
}

func KeyAuthCooldown(scope string, value string) string {
	return fmt.Sprintf("auth:cooldown:%s:%s", scope, value)
}

func KeyRateLimitEmail(email string) string {
	return fmt.Sprintf("ratelimit:email:%s", email)
}

func KeyPasswordLoginFailures(email string) string {
	return fmt.Sprintf("password:failures:%s", email)
}

func KeyPasswordLoginLock(email string) string {
	return fmt.Sprintf("password:lock:%s", email)
}

func KeyQR(uuid string) string {
	return fmt.Sprintf("qr:%s", uuid)
}

func KeySession(sessionID string) string {
	return fmt.Sprintf("session:%s", sessionID)
}

func KeyOAuthState(state string) string {
	return fmt.Sprintf("oauth:state:%s", state)
}

func KeyOAuthPendingBinding(bindingID string) string {
	return fmt.Sprintf("oauth:pending-binding:%s", bindingID)
}
