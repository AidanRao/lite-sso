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
	MessageCenter MessageCenterConfig   `mapstructure:"message_center"`
	Dev           DevConfig             `mapstructure:"dev"`
	OAuth         ThirdPartyOAuthConfig `mapstructure:"oauth"`
	Admin         AdminConfig           `mapstructure:"admin"`
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
	Port string `mapstructure:"port"`
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
	MaxLoginAttempts  int           `mapstructure:"max_login_attempts"`
	LockoutDuration   time.Duration `mapstructure:"lockout_duration"`
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
		"security.max_login_attempts",
		"security.lockout_duration",
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
		"server.port":                  "8080",
		"security.access_token_expire": "12h",
		"security.max_login_attempts":  5,
		"security.lockout_duration":    "30m",
		"dev.skip_send_message":        false,
		"dev.fixed_email_otp":          "",
		"admin.user_ids":               []string{},
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
