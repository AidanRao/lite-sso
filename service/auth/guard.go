package auth

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"sso-server/common"
	"sso-server/dal/kv"
)

type loginGuard struct {
	store  kv.SecurityStore
	secret string
	config guardConfig
}

type guardConfig struct {
	PasswordAccountFailLimit int
	PasswordDeviceFailLimit  int
	PasswordIPFailLimit      int
	PasswordAccountWindow    time.Duration
	PasswordDeviceWindow     time.Duration
	PasswordIPWindow         time.Duration
}

const passwordDistinctAccountCaptchaThreshold = 10

func newLoginGuard(store kv.SecurityStore, service *AuthService) *loginGuard {
	config := guardConfig{
		PasswordAccountFailLimit: 5,
		PasswordDeviceFailLimit:  20,
		PasswordIPFailLimit:      100,
		PasswordAccountWindow:    10 * time.Minute,
		PasswordDeviceWindow:     10 * time.Minute,
		PasswordIPWindow:         time.Hour,
	}
	if service.cfg != nil {
		if service.cfg.Auth.PasswordAccountFailLimit > 0 {
			config.PasswordAccountFailLimit = service.cfg.Auth.PasswordAccountFailLimit
		}
		if service.cfg.Auth.PasswordDeviceFailLimit > 0 {
			config.PasswordDeviceFailLimit = service.cfg.Auth.PasswordDeviceFailLimit
		}
		if service.cfg.Auth.PasswordIPFailLimit > 0 {
			config.PasswordIPFailLimit = service.cfg.Auth.PasswordIPFailLimit
		}
		if service.cfg.Auth.PasswordAccountFailWindow > 0 {
			config.PasswordAccountWindow = service.cfg.Auth.PasswordAccountFailWindow
		}
		if service.cfg.Auth.PasswordDeviceFailWindow > 0 {
			config.PasswordDeviceWindow = service.cfg.Auth.PasswordDeviceFailWindow
		}
		if service.cfg.Auth.PasswordIPFailWindow > 0 {
			config.PasswordIPWindow = service.cfg.Auth.PasswordIPFailWindow
		}
	}
	return &loginGuard{store: store, secret: service.authSecret(), config: config}
}

func (g *loginGuard) allowOTPSend(ctx context.Context, email string, deviceID string, ip string) error {
	checks := []struct {
		key    string
		rate   int
		burst  int
		period time.Duration
	}{
		{kv.KeyAuthRateLimit("otp:send:email:minute", identifierHash(g.secret, email)), 1, 1, time.Minute},
		{kv.KeyAuthRateLimit("otp:send:email:hour", identifierHash(g.secret, email)), 5, 5, time.Hour},
		{kv.KeyAuthRateLimit("otp:send:email:day", identifierHash(g.secret, email)), 20, 20, 24 * time.Hour},
		{kv.KeyAuthRateLimit("otp:send:device:hour", deviceID), 15, 15, time.Hour},
		{kv.KeyAuthRateLimit("otp:send:ip:hour", identifierHash(g.secret, ip)), 30, 30, time.Hour},
		{kv.KeyAuthRateLimit("otp:send:ip:day", identifierHash(g.secret, ip)), 100, 100, 24 * time.Hour},
	}
	return g.allowAll(ctx, checks)
}

func (g *loginGuard) allowOTPVerify(ctx context.Context, email string, deviceID string, ip string) error {
	checks := []struct {
		key    string
		rate   int
		burst  int
		period time.Duration
	}{
		{kv.KeyAuthRateLimit("otp:verify:device", deviceID), 20, 20, 10 * time.Minute},
		{kv.KeyAuthRateLimit("otp:verify:email", identifierHash(g.secret, email)), 20, 20, 30 * time.Minute},
		{kv.KeyAuthRateLimit("otp:verify:ip:10m", identifierHash(g.secret, ip)), 50, 50, 10 * time.Minute},
		{kv.KeyAuthRateLimit("otp:verify:ip:hour", identifierHash(g.secret, ip)), 200, 200, time.Hour},
	}
	return g.allowAll(ctx, checks)
}

func (g *loginGuard) allowPasswordLogin(ctx context.Context, email string, deviceID string, ip string, captchaValid bool) error {
	for _, item := range []struct {
		key string
	}{
		{kv.KeyAuthDistinctAccounts("password:device", deviceID)},
		{kv.KeyAuthDistinctAccounts("password:ip", identifierHash(g.secret, ip))},
	} {
		cardinality, err := g.store.SetCardinality(ctx, item.key)
		if err != nil && !errors.Is(err, kv.ErrScriptUnsupported) {
			return err
		}
		if cardinality >= passwordDistinctAccountCaptchaThreshold && !captchaValid {
			return common.CaptchaRequiredError{Reason: "distinct_accounts"}
		}
	}

	for _, item := range []struct {
		key string
		max int
	}{
		{kv.KeyAuthFailure("password:account", identifierHash(g.secret, email)), g.config.PasswordAccountFailLimit},
		{kv.KeyAuthFailure("password:device", deviceID), g.config.PasswordDeviceFailLimit},
		{kv.KeyAuthFailure("password:ip", identifierHash(g.secret, ip)), g.config.PasswordIPFailLimit},
	} {
		value, err := g.store.Get(ctx, item.key)
		if err != nil && !errors.Is(err, kv.ErrNotFound) {
			return err
		}
		count := 0
		if value != "" {
			count, _ = strconv.Atoi(value)
		}
		if count >= item.max && !captchaValid {
			return common.CaptchaRequiredError{Reason: "login_risk"}
		}
	}
	return nil
}

func (g *loginGuard) recordPasswordFailure(ctx context.Context, email string, deviceID string, ip string) error {
	if _, err := g.store.Increment(ctx, kv.KeyAuthFailure("password:account", identifierHash(g.secret, email)), g.config.PasswordAccountWindow); err != nil {
		return err
	}
	if _, err := g.store.Increment(ctx, kv.KeyAuthFailure("password:device", deviceID), g.config.PasswordDeviceWindow); err != nil {
		return err
	}
	if _, err := g.store.Increment(ctx, kv.KeyAuthFailure("password:ip", identifierHash(g.secret, ip)), g.config.PasswordIPWindow); err != nil {
		return err
	}
	if err := g.store.AddToSet(ctx, kv.KeyAuthDistinctAccounts("password:device", deviceID), identifierHash(g.secret, email), g.config.PasswordDeviceWindow); err != nil && !errors.Is(err, kv.ErrScriptUnsupported) {
		return err
	}
	if err := g.store.AddToSet(ctx, kv.KeyAuthDistinctAccounts("password:ip", identifierHash(g.secret, ip)), identifierHash(g.secret, email), g.config.PasswordIPWindow); err != nil && !errors.Is(err, kv.ErrScriptUnsupported) {
		return err
	}
	return nil
}

func (g *loginGuard) clearPasswordAccountFailure(ctx context.Context, email string) error {
	return g.store.Del(ctx, kv.KeyAuthFailure("password:account", identifierHash(g.secret, email)))
}

func (g *loginGuard) recordOTPFailure(ctx context.Context, email string, deviceID string, ip string) error {
	if _, err := g.store.Increment(ctx, kv.KeyAuthFailure("otp:email", identifierHash(g.secret, email)), 30*time.Minute); err != nil {
		return err
	}
	if _, err := g.store.Increment(ctx, kv.KeyAuthFailure("otp:device", deviceID), 10*time.Minute); err != nil {
		return err
	}
	_, err := g.store.Increment(ctx, kv.KeyAuthFailure("otp:ip", identifierHash(g.secret, ip)), 10*time.Minute)
	return err
}

func (g *loginGuard) allowAll(ctx context.Context, checks []struct {
	key    string
	rate   int
	burst  int
	period time.Duration
}) error {
	for _, check := range checks {
		allowed, retryAfter, err := g.store.RateLimit(ctx, check.key, check.rate, check.burst, check.period)
		if err != nil {
			return err
		}
		if !allowed {
			seconds := int(retryAfter / time.Second)
			if seconds < 1 {
				seconds = 1
			}
			return common.RateLimitedError{RetryAfterSeconds: seconds, Reason: check.key}
		}
	}
	return nil
}

func (g *loginGuard) requiresCAPTCHA(ctx context.Context, scope string, value string) bool {
	key := kv.KeyAuthFailure(scope, strings.TrimSpace(value))
	_, err := g.store.Get(ctx, key)
	return err == nil
}
