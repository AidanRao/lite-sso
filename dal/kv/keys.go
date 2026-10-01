// Package kv provides Redis operations and the shared SSO key layout.
package kv

import (
	"crypto/sha256"
	"encoding/hex"
)

func KeyCaptcha(captchaID string) string {
	return "captcha:" + captchaID
}

func KeyChallenge(challengeID string) string {
	return "auth:challenge:" + challengeID
}

func KeyAuthRateLimit(scope string, value string) string {
	return "auth:rate:" + scope + ":" + value
}

func KeyAuthFailure(scope string, value string) string {
	return "auth:fail:" + scope + ":" + value
}

func KeyAuthDistinctAccounts(scope string, value string) string {
	return "auth:accounts:" + scope + ":" + value
}

func KeyQR(uuid string) string {
	return "qr:" + uuid
}

func KeySession(sessionID string) string {
	return "session:" + sessionID
}

func KeyOAuthState(state string) string {
	return "oauth:state:" + state
}

func KeyOAuthPendingBinding(bindingID string) string {
	return "oauth:pending-binding:" + bindingID
}

// KeyOAuthAuthorizationCode keeps the bearer code out of Redis key names.
func KeyOAuthAuthorizationCode(code string) string {
	digest := sha256.Sum256([]byte(code))
	return "oauth:authorization-code:" + hex.EncodeToString(digest[:])
}

func KeyOAuthRefreshToken(tokenDigest string) string {
	return "oauth:refresh:token:" + tokenDigest
}

func KeyOAuthRefreshUser(userID string) string {
	return "oauth:refresh:user:" + userID
}

func KeyOAuthRefreshRevoked(userID string) string {
	return "oauth:refresh:revoked:" + userID
}

func KeyWebAuthnCeremony(ceremonyID string) string {
	return "webauthn:ceremony:" + ceremonyID
}

func KeyReauthGrant(tokenHash string) string {
	return "reauth:grant:" + tokenHash
}

func KeyReauthSession(sessionID string) string {
	return "reauth:session:" + sessionID
}

func KeyReauthEmailChallenge(challengeID string) string {
	return "reauth:email:" + challengeID
}
