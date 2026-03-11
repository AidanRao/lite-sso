package conf

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Cache    CacheConfig
	Security SecurityConfig
	Email    EmailConfig
	Dev      DevConfig
}

type ServerConfig struct {
	Port string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
}

type CacheConfig struct {
	URL      string
	Password string
}

type SecurityConfig struct {
	AccessTokenExpire time.Duration
	MaxLoginAttempts  int
	LockoutDuration   time.Duration
}

type EmailConfig struct {
	SMTPAddr string
	SMTPUser string
	SMTPPass string
	SMTPFrom string
}

type DevConfig struct {
	UserID  string
	EchoOTP bool
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port: getEnv("PORT", "8080"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", ""),
			Name:     getEnv("DB_NAME", "sso"),
		},
		Cache: CacheConfig{
			URL:      getEnv("REDIS_URL", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
		},
		Security: SecurityConfig{
			AccessTokenExpire: parseDuration(getEnv("ACCESS_TOKEN_EXPIRE", "12h")),
			MaxLoginAttempts:  parseInt(getEnv("MAX_LOGIN_ATTEMPTS", "5"), 5),
			LockoutDuration:   parseDuration(getEnv("LOCKOUT_DURATION", "30m")),
		},
		Email: EmailConfig{
			SMTPAddr: getEnv("SMTP_ADDR", ""),
			SMTPUser: getEnv("SMTP_USER", ""),
			SMTPPass: getEnv("SMTP_PASS", ""),
			SMTPFrom: getEnv("SMTP_FROM", ""),
		},
		Dev: DevConfig{
			UserID:  getEnv("DEV_USER_ID", "u1"),
			EchoOTP: parseBool(getEnv("DEV_ECHO_OTP", "false")),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0
	}
	return d
}

func parseInt(s string, defaultValue int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return defaultValue
	}
	return n
}

func parseBool(s string) bool {
	switch s {
	case "1", "true", "TRUE", "True", "yes", "YES", "Yes":
		return true
	default:
		return false
	}
}
