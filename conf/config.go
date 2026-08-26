package conf

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Environment string

const (
	EnvLocal Environment = "local"
	EnvTest  Environment = "test"
	EnvProd  Environment = "prod"
)

func GetEnv() Environment {
	name := GetEnvironmentName()
	switch name {
	case string(EnvProd):
		return EnvProd
	case string(EnvTest):
		return EnvTest
	default:
		return EnvLocal
	}
}

// GetEnvironmentName returns the normalized environment name used for Redis key isolation.
func GetEnvironmentName() string {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("ENV")))
	if env == "" {
		return string(EnvLocal)
	}
	return env
}

type Config struct {
	Server        ServerConfig          `mapstructure:"server"`
	Database      DatabaseConfig        `mapstructure:"database"`
	Cache         CacheConfig           `mapstructure:"cache"`
	Security      SecurityConfig        `mapstructure:"security"`
	Auth          AuthConfig            `mapstructure:"auth"`
	MessageCenter MessageCenterConfig   `mapstructure:"message_center"`
	Dev           DevConfig             `mapstructure:"dev"`
	OAuth         ThirdPartyOAuthConfig `mapstructure:"oauth"`
	Admin         AdminConfig           `mapstructure:"admin"`
	OSS           OSSConfig             `mapstructure:"oss"`
}

// OSSConfig contains the Alibaba Cloud OSS settings used for user avatars.
type OSSConfig struct {
	Region          string `mapstructure:"region"`
	Endpoint        string `mapstructure:"endpoint"`
	Bucket          string `mapstructure:"bucket"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	AccessKeySecret string `mapstructure:"access_key_secret"`
	AvatarPrefix    string `mapstructure:"avatar_prefix"`
	PublicBaseURL   string `mapstructure:"public_base_url"`
}

// IsConfigured reports whether any OSS setting has been provided.
func (c OSSConfig) IsConfigured() bool {
	return strings.TrimSpace(c.Region) != "" ||
		strings.TrimSpace(c.Endpoint) != "" ||
		strings.TrimSpace(c.Bucket) != "" ||
		strings.TrimSpace(c.AccessKeyID) != "" ||
		strings.TrimSpace(c.AccessKeySecret) != "" ||
		strings.TrimSpace(c.AvatarPrefix) != "" ||
		strings.TrimSpace(c.PublicBaseURL) != ""
}

// ValidateOSS checks that all required OSS settings are supplied together.
func (c *Config) ValidateOSS() error {
	if c == nil {
		return errors.New("configuration is required")
	}
	if !c.OSS.IsConfigured() {
		if GetEnv() == EnvProd {
			return errors.New("oss configuration is required in production")
		}
		return nil
	}

	values := map[string]string{
		"oss.region":            c.OSS.Region,
		"oss.bucket":            c.OSS.Bucket,
		"oss.access_key_id":     c.OSS.AccessKeyID,
		"oss.access_key_secret": c.OSS.AccessKeySecret,
		"oss.avatar_prefix":     c.OSS.AvatarPrefix,
		"oss.public_base_url":   c.OSS.PublicBaseURL,
	}
	for name, value := range values {
		if strings.TrimSpace(value) == "" {
			return errors.New(name + " is required when OSS is configured")
		}
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

type FeishuOAuthConfig struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	RedirectURI  string `mapstructure:"redirect_uri"`
}

type ServerConfig struct {
	Port              string `mapstructure:"port"`
	TrustProxyHeaders bool   `mapstructure:"trust_proxy_headers"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Name     string `mapstructure:"name"`
	SSLMode  string `mapstructure:"sslmode"`
}

type CacheConfig struct {
	URL string `mapstructure:"url"`
}

type SecurityConfig struct {
	AccessTokenExpire time.Duration `mapstructure:"access_token_expire"`
}

type AuthConfig struct {
	OTPSecret                 string        `mapstructure:"otp_secret"`
	JWTSecret                 string        `mapstructure:"jwt_secret"`
	OTPExpire                 time.Duration `mapstructure:"otp_expire"`
	OTPMaxAttempts            int           `mapstructure:"otp_max_attempts"`
	AccessTokenTTL            time.Duration `mapstructure:"access_token_ttl"`
	RefreshTokenTTL           time.Duration `mapstructure:"refresh_token_ttl"`
	PasswordMinLength         int           `mapstructure:"password_min_length"`
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
	if GetEnv() != EnvProd {
		return nil
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
	env := GetEnv()
	v := viper.New()

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	bindEnvs(v)

	if configFile := os.Getenv("CONFIG_FILE"); configFile != "" {
		v.SetConfigFile(configFile)
	} else {
		v.SetConfigName(string(env))
		v.AddConfigPath("conf")
		v.AddConfigPath(".")
	}

	setDefaults(v, env)

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

	return &cfg, nil
}

func bindEnvs(v *viper.Viper) {
	envKeys := []string{
		"database.host",
		"database.port",
		"database.user",
		"database.password",
		"database.name",
		"security.access_token_expire",
		"auth.otp_secret",
		"auth.jwt_secret",
		"auth.otp_expire",
		"auth.otp_max_attempts",
		"auth.access_token_ttl",
		"auth.refresh_token_ttl",
		"auth.password_min_length",
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
		"oss.region",
		"oss.endpoint",
		"oss.bucket",
		"oss.access_key_id",
		"oss.access_key_secret",
		"oss.avatar_prefix",
		"oss.public_base_url",
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

func setDefaults(v *viper.Viper, env Environment) {
	if env != EnvProd {
		return
	}

	defaults := map[string]any{
		"server.port":                       "8080",
		"server.trust_proxy_headers":        false,
		"security.access_token_expire":      "12h",
		"auth.otp_expire":                   "5m",
		"auth.otp_max_attempts":             5,
		"auth.access_token_ttl":             "15m",
		"auth.refresh_token_ttl":            "720h",
		"auth.password_min_length":          12,
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
