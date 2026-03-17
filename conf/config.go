package conf

import (
	"os"
	"strconv"
	"time"
)

// Environment 环境类型
type Environment string

const (
	// EnvLocal 本地环境
	EnvLocal Environment = "local"
	// EnvProd 生产环境
	EnvProd Environment = "prod"
)

// GetEnv 获取当前环境
func GetEnv() Environment {
	env := os.Getenv("ENV")
	switch env {
	case "prod":
		return EnvProd
	default:
		return EnvLocal
	}
}

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
	SMTPHost string
	SMTPPort int
	SMTPUser string
	SMTPPass string
	SMTPFrom string
}

type DevConfig struct {
	UserID  string
	EchoOTP bool
}

// Load 根据环境加载配置
func Load() *Config {
	env := GetEnv()

	switch env {
	case EnvProd:
		return loadProdConfig()
	default:
		return loadLocalConfig()
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
