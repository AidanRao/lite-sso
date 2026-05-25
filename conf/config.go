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
	EnvProd  Environment = "prod"
)

func GetEnv() Environment {
	env := os.Getenv("ENV")
	if env == "prod" {
		return EnvProd
	}
	return EnvLocal
}

type Config struct {
	Server   ServerConfig          `mapstructure:"server"`
	Database DatabaseConfig        `mapstructure:"database"`
	Cache    CacheConfig           `mapstructure:"cache"`
	Security SecurityConfig        `mapstructure:"security"`
	Email    EmailConfig           `mapstructure:"email"`
	Dev      DevConfig             `mapstructure:"dev"`
	OAuth    ThirdPartyOAuthConfig `mapstructure:"oauth"`
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
}

type CacheConfig struct {
	URL      string `mapstructure:"url"`
	Password string `mapstructure:"password"`
}

type SecurityConfig struct {
	AccessTokenExpire time.Duration `mapstructure:"access_token_expire"`
	MaxLoginAttempts  int           `mapstructure:"max_login_attempts"`
	LockoutDuration   time.Duration `mapstructure:"lockout_duration"`
}

type EmailConfig struct {
	SMTPHost string `mapstructure:"smtp_host"`
	SMTPPort int    `mapstructure:"smtp_port"`
	SMTPUser string `mapstructure:"smtp_user"`
	SMTPPass string `mapstructure:"smtp_pass"`
	SMTPFrom string `mapstructure:"smtp_from"`
}

type DevConfig struct {
	FixedEmailOTP string `mapstructure:"fixed_email_otp"`
	SkipSendEmail bool   `mapstructure:"skip_send_email"`
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

	return &cfg, nil
}

func bindEnvs(v *viper.Viper) {
	envKeys := []string{
		"server.port",
		"database.host",
		"database.port",
		"database.user",
		"database.password",
		"database.name",
		"cache.url",
		"cache.password",
		"security.access_token_expire",
		"security.max_login_attempts",
		"security.lockout_duration",
		"email.smtp_host",
		"email.smtp_port",
		"email.smtp_user",
		"email.smtp_pass",
		"email.smtp_from",
		"dev.fixed_email_otp",
		"dev.skip_send_email",
		"oauth.github.client_id",
		"oauth.github.client_secret",
		"oauth.github.redirect_uri",
		"oauth.feishu.client_id",
		"oauth.feishu.client_secret",
		"oauth.feishu.redirect_uri",
	}

	for _, key := range envKeys {
		if err := v.BindEnv(key); err != nil {
			panic(err)
		}
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
		"dev.skip_send_email":          false,
		"dev.fixed_email_otp":          "",
	}

	for key, value := range defaults {
		v.SetDefault(key, value)
	}
}
