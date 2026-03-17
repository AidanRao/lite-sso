package conf

// loadProdConfig 加载生产环境配置
func loadProdConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: getEnv("PORT", "8080"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", ""),
			Name:     getEnv("DB_NAME", "sso-server"),
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
			SMTPHost: getEnv("SMTP_HOST", "smtp.163.com"),
			SMTPPort: parseInt(getEnv("SMTP_PORT", "465"), 465),
			SMTPUser: getEnv("SMTP_USER", ""),
			SMTPPass: getEnv("SMTP_PASS", ""),
			SMTPFrom: getEnv("SMTP_FROM", ""),
		},
		Dev: DevConfig{
			UserID:  getEnv("DEV_USER_ID", ""),
			EchoOTP: parseBool(getEnv("DEV_ECHO_OTP", "false")), // 生产环境默认关闭EchoOTP
		},
	}
}
