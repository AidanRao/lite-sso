package kv

import "fmt"

const keyPrefix = "lite-sso"

func KeyCaptcha(captchaID string) string {
	return fmt.Sprintf("%s:captcha:%s", keyPrefix, captchaID)
}

func KeyOTP(email string) string {
	return fmt.Sprintf("%s:otp:%s", keyPrefix, email)
}

func KeyRateLimitEmail(email string) string {
	return fmt.Sprintf("%s:ratelimit:email:%s", keyPrefix, email)
}

func KeyQR(uuid string) string {
	return fmt.Sprintf("%s:qr:%s", keyPrefix, uuid)
}

func KeySession(sessionID string) string {
	return fmt.Sprintf("%s:session:%s", keyPrefix, sessionID)
}
