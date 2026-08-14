package kv

import "fmt"

func KeyCaptcha(captchaID string) string {
	return fmt.Sprintf("captcha:%s", captchaID)
}

func KeyOTP(email string) string {
	return fmt.Sprintf("otp:%s", email)
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
