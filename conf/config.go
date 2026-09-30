package conf

import (
	"errors"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// GetEnvironmentName returns the name used to select configuration and isolate Redis keys.
func GetEnvironmentName() string {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("ENV")))
	if env == "" {
		return "local"
	}
	return env
}

type Config struct {
	Audit         AuditConfig           `mapstructure:"audit"`
	Server        ServerConfig          `mapstructure:"server"`
	Database      DatabaseConfig        `mapstructure:"database"`
	Cache         CacheConfig           `mapstructure:"cache"`
	Tokens        TokenConfig           `mapstructure:"tokens"`
	Auth          AuthConfig            `mapstructure:"auth"`
	MessageCenter MessageCenterConfig   `mapstructure:"message_center"`
	Dev           DevConfig             `mapstructure:"dev"`
	OAuth         ThirdPartyOAuthConfig `mapstructure:"oauth"`
	Admin         AdminConfig           `mapstructure:"admin"`
	OSS           OSSConfig             `mapstructure:"oss"`
	Passkey       PasskeyConfig         `mapstructure:"passkey"`
	Email         EmailConfig           `mapstructure:"email"`
}

// EmailConfig contains user email lifecycle limits and the public verification origin.
type EmailConfig struct {
	MaxAddresses        int    `mapstructure:"max_addresses"`
	VerificationBaseURL string `mapstructure:"verification_base_url"`
}

// ValidateEmail checks the multi-email account configuration.
func (c *Config) ValidateEmail() error {
	if c == nil {
		return errors.New("configuration is required")
	}
	if c.Email.MaxAddresses <= 0 {
		return errors.New("email.max_addresses must be positive")
	}
	baseURL := strings.TrimSpace(c.Email.VerificationBaseURL)
	if baseURL == "" {
		return errors.New("email.verification_base_url is required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("email.verification_base_url must be a valid HTTP origin")
	}
	return nil
}

// PasskeyConfig contains the relying-party and lifetime settings for WebAuthn.
type PasskeyConfig struct {
	RPID          string        `mapstructure:"rp_id"`
	RPOrigins     []string      `mapstructure:"rp_origins"`
	RPDisplayName string        `mapstructure:"rp_display_name"`
	CeremonyTTL   time.Duration `mapstructure:"ceremony_ttl"`
}

// ValidatePasskey checks the WebAuthn relying-party configuration.
func (c *Config) ValidatePasskey() error {
	if c == nil {
		return errors.New("configuration is required")
	}
	if strings.TrimSpace(c.Passkey.RPID) == "" {
		return errors.New("passkey.rp_id is required")
	}
	if len(c.Passkey.RPOrigins) == 0 {
		return errors.New("passkey.rp_origins is required")
	}
	for _, origin := range c.Passkey.RPOrigins {
		parsed, err := url.Parse(strings.TrimSpace(origin))
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
			return errors.New("passkey.rp_origins must contain valid origins")
		}
	}
	if strings.TrimSpace(c.Passkey.RPDisplayName) == "" {
		return errors.New("passkey.rp_display_name is required")
	}
	if c.Passkey.CeremonyTTL <= 0 {
		return errors.New("passkey.ceremony_ttl must be positive")
	}
	return nil
}

// ValidateReauth checks the short-lived authorization grant configuration.
func (c *Config) ValidateReauth() error {
	if c == nil {
		return errors.New("configuration is required")
	}
	if c.Tokens.Reauth.GrantTTL <= 0 {
		return errors.New("tokens.reauth.grant_ttl must be positive")
	}
	return nil
}

func (c *Config) IsAdminUser(userID string) bool {
	if c == nil || strings.TrimSpace(userID) == "" {
		return false
	}

	for _, adminUserID := range c.Admin.UserIDs {
		if strings.TrimSpace(adminUserID) == userID {
			return true
		}
	}
	return false
}

type AdminConfig struct {
	UserIDs []string `mapstructure:"user_ids"`
}

type ThirdPartyOAuthConfig struct {
	GitHub GitHubOAuthConfig `mapstructure:"github"`
	Feishu FeishuOAuthConfig `mapstructure:"feishu"`
}

type GitHubOAuthConfig struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	RedirectURI  string `mapstructure:"redirect_uri"`
}

// IsConfigured reports whether GitHub OAuth has the credentials required for use.
func (c GitHubOAuthConfig) IsConfigured() bool {
	return strings.TrimSpace(c.ClientID) != "" && strings.TrimSpace(c.ClientSecret) != ""
}

type FeishuOAuthConfig struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	RedirectURI  string `mapstructure:"redirect_uri"`
}

// IsConfigured reports whether Feishu OAuth has the credentials required for use.
func (c FeishuOAuthConfig) IsConfigured() bool {
	return strings.TrimSpace(c.ClientID) != "" && strings.TrimSpace(c.ClientSecret) != ""
}

type ServerConfig struct {
	Port              string `mapstructure:"port"`
	TrustProxyHeaders bool   `mapstructure:"trust_proxy_headers"`
	CookieSecure      bool   `mapstructure:"cookie_secure"`
}

type DatabaseConfig struct {
	Host         string `mapstructure:"host"`
	Port         string `mapstructure:"port"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	Name         string `mapstructure:"name"`
	SSLMode      string `mapstructure:"sslmode"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
}

type CacheConfig struct {
	URL string `mapstructure:"url"`
}

// TokenConfig contains lifetimes grouped by the token's purpose and issuer.
type TokenConfig struct {
	OAuth   OAuthTokenConfig   `mapstructure:"oauth"`
	Session SessionTokenConfig `mapstructure:"session"`
	Reauth  ReauthTokenConfig  `mapstructure:"reauth"`
}

// OAuthTokenConfig contains lifetimes for OAuth authorization grants and tokens.
type OAuthTokenConfig struct {
	AccessTokenTTL       time.Duration `mapstructure:"access_token_ttl"`
	AuthorizationCodeTTL time.Duration `mapstructure:"authorization_code_ttl"`
	RefreshTokenTTL      time.Duration `mapstructure:"refresh_token_ttl"`
	Issuer               string        `mapstructure:"issuer"`
	SigningKID           string        `mapstructure:"signing_kid"`
	SigningPrivateKeyPEM string        `mapstructure:"signing_private_key_pem"`
	PreviousPublicKeys   string        `mapstructure:"previous_public_keys"`
}

// ValidateOAuthTokenLifetimes checks the configured OAuth token lifetimes.
func (c *Config) ValidateOAuthTokenLifetimes() error {
	if c.Tokens.OAuth.AccessTokenTTL <= 0 || c.Tokens.OAuth.AuthorizationCodeTTL <= 0 || c.Tokens.OAuth.RefreshTokenTTL <= 0 {
		return errors.New("OAuth token lifetimes must be positive")
	}
	return nil
}

// SessionTokenConfig contains lifetimes for first-party login sessions.
type SessionTokenConfig struct {
	AccessTokenTTL  time.Duration `mapstructure:"access_token_ttl"`
	RefreshTokenTTL time.Duration `mapstructure:"refresh_token_ttl"`
}

// ValidateSessionTokenLifetimes checks the first-party session token lifetimes.
func (c *Config) ValidateSessionTokenLifetimes() error {
	if c.Tokens.Session.AccessTokenTTL <= 0 || c.Tokens.Session.RefreshTokenTTL <= 0 {
		return errors.New("session token lifetimes must be positive")
	}
	return nil
}

// ReauthTokenConfig contains lifetime settings for reauthentication grants.
type ReauthTokenConfig struct {
	GrantTTL time.Duration `mapstructure:"grant_ttl"`
}

type AuthConfig struct {
	OTPSecret                 string        `mapstructure:"otp_secret"`
	JWTSecret                 string        `mapstructure:"jwt_secret"`
	OTPExpire                 time.Duration `mapstructure:"otp_expire"`
	OTPMaxAttempts            int           `mapstructure:"otp_max_attempts"`
	PasswordAccountFailLimit  int           `mapstructure:"password_account_fail_limit"`
	PasswordDeviceFailLimit   int           `mapstructure:"password_device_fail_limit"`
	PasswordIPFailLimit       int           `mapstructure:"password_ip_fail_limit"`
	PasswordAccountFailWindow time.Duration `mapstructure:"password_account_fail_window"`
	PasswordDeviceFailWindow  time.Duration `mapstructure:"password_device_fail_window"`
	PasswordIPFailWindow      time.Duration `mapstructure:"password_ip_fail_window"`
}

func (c *Config) ValidateAuthSecrets() error {
	if c == nil {
		return errors.New("configuration is required")
	}
	if len(strings.TrimSpace(c.Auth.OTPSecret)) < 32 {
		return errors.New("auth.otp_secret must contain at least 32 characters")
	}
	if len(strings.TrimSpace(c.Auth.JWTSecret)) < 32 {
		return errors.New("auth.jwt_secret must contain at least 32 characters")
	}
	return nil
}

type MessageCenterConfig struct {
	URL       string `mapstructure:"url"`
	APIKey    string `mapstructure:"api_key"`
	SenderKey string `mapstructure:"sender_key"`
}

type DevConfig struct {
	FixedEmailOTP   string `mapstructure:"fixed_email_otp"`
	SkipSendMessage bool   `mapstructure:"skip_send_message"`
}

func Load() (*Config, error) {
	v := viper.New()

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	bindEnvs(v)

	if configFile := os.Getenv("CONFIG_FILE"); configFile != "" {
		v.SetConfigFile(configFile)
	} else {
		v.SetConfigName(GetEnvironmentName())
		v.AddConfigPath("conf")
		v.AddConfigPath(".")
	}

	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := errors.AsType[viper.ConfigFileNotFoundError](err); !ok {
			return nil, err
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	cfg.Admin.UserIDs = readStringSlice(v, "admin.user_ids", cfg.Admin.UserIDs)
	cfg.Passkey.RPOrigins = readStringSlice(v, "passkey.rp_origins", cfg.Passkey.RPOrigins)

	return &cfg, nil
}

func bindEnvs(v *viper.Viper) {
	envKeys := []string{
		"audit.queue_capacity", "audit.batch_size", "audit.flush_interval", "audit.write_timeout",
		"database.host",
		"database.port",
		"database.user",
		"database.password",
		"database.name",
		"database.max_open_conns",
		"database.max_idle_conns",
		"tokens.oauth.access_token_ttl",
		"tokens.oauth.authorization_code_ttl",
		"tokens.oauth.refresh_token_ttl",
		"tokens.oauth.issuer",
		"tokens.oauth.signing_kid",
		"tokens.oauth.signing_private_key_pem",
		"tokens.oauth.previous_public_keys",
		"tokens.session.access_token_ttl",
		"tokens.session.refresh_token_ttl",
		"tokens.reauth.grant_ttl",
		"auth.otp_secret",
		"auth.jwt_secret",
		"auth.otp_expire",
		"auth.otp_max_attempts",
		"auth.password_account_fail_limit",
		"auth.password_device_fail_limit",
		"auth.password_ip_fail_limit",
		"auth.password_account_fail_window",
		"auth.password_device_fail_window",
		"auth.password_ip_fail_window",
		"message_center.url",
		"message_center.api_key",
		"message_center.sender_key",
		"dev.fixed_email_otp",
		"dev.skip_send_message",
		"oauth.github.client_id",
		"oauth.github.client_secret",
		"oauth.github.redirect_uri",
		"oauth.feishu.client_id",
		"oauth.feishu.client_secret",
		"oauth.feishu.redirect_uri",
		"admin.user_ids",
		"server.trust_proxy_headers",
		"server.cookie_secure",
		"oss.region",
		"oss.endpoint",
		"oss.bucket",
		"oss.access_key_id",
		"oss.access_key_secret",
		"oss.avatar_prefix",
		"oss.public_base_url",
		"passkey.rp_id",
		"passkey.rp_origins",
		"passkey.rp_display_name",
		"passkey.ceremony_ttl",
		"email.max_addresses",
		"email.verification_base_url",
	}

	for _, key := range envKeys {
		if err := v.BindEnv(key); err != nil {
			panic(err)
		}
	}
	if err := v.BindEnv("server.port", "PORT", "SERVER_PORT"); err != nil {
		panic(err)
	}
	if err := v.BindEnv("database.sslmode", "DB_SSLMODE"); err != nil {
		panic(err)
	}
	if err := v.BindEnv("cache.url", "REDIS_URL"); err != nil {
		panic(err)
	}
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("database.max_open_conns", 2)
	v.SetDefault("database.max_idle_conns", 1)
	v.SetDefault("audit.queue_capacity", 1024)
	v.SetDefault("audit.batch_size", 50)
	v.SetDefault("audit.flush_interval", "1s")
	v.SetDefault("audit.write_timeout", "2s")
	v.SetDefault("passkey.ceremony_ttl", "5m")
	v.SetDefault("tokens.oauth.access_token_ttl", "30m")
	v.SetDefault("tokens.oauth.authorization_code_ttl", "5m")
	v.SetDefault("tokens.oauth.refresh_token_ttl", "720h")
	v.SetDefault("tokens.session.access_token_ttl", "15m")
	v.SetDefault("tokens.session.refresh_token_ttl", "720h")
	v.SetDefault("tokens.reauth.grant_ttl", "5m")
	v.SetDefault("email.max_addresses", 3)
	defaults := map[string]any{
		"server.port":                       "8080",
		"server.trust_proxy_headers":        false,
		"auth.otp_expire":                   "5m",
		"auth.otp_max_attempts":             5,
		"auth.password_account_fail_limit":  5,
		"auth.password_device_fail_limit":   20,
		"auth.password_ip_fail_limit":       100,
		"auth.password_account_fail_window": "10m",
		"auth.password_device_fail_window":  "10m",
		"auth.password_ip_fail_window":      "1h",
		"dev.skip_send_message":             false,
		"dev.fixed_email_otp":               "",
		"admin.user_ids":                    []string{},
	}

	for key, value := range defaults {
		v.SetDefault(key, value)
	}
}

func readStringSlice(v *viper.Viper, key string, fallback []string) []string {
	values := v.GetStringSlice(key)
	if len(values) == 0 {
		values = fallback
	}
	if len(values) == 1 {
		values = strings.Split(values[0], ",")
	}

	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
