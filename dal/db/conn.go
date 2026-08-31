package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"sso-server/conf"
)

const (
	defaultSSLMode  = "disable"
	connMaxIdleTime = 30 * time.Second
)

var DB *gorm.DB

func Init(cfg *conf.Config) error {
	var err error
	DB, err = Open(cfg)
	return err
}

// Open creates and verifies a database connection using the application configuration.
func Open(cfg *conf.Config) (*gorm.DB, error) {
	dsn := buildDSN(cfg)

	database, err := gorm.Open(postgres.New(postgres.Config{
		DSN: dsn,
	}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	sqlDB, err := database.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}
	configureConnectionPool(sqlDB, cfg.Database)

	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return database, nil
}

func configureConnectionPool(sqlDB *sql.DB, cfg conf.DatabaseConfig) {
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxIdleTime(connMaxIdleTime)
}

func buildDSN(cfg *conf.Config) string {
	sslMode := strings.TrimSpace(cfg.Database.SSLMode)
	if sslMode == "" {
		sslMode = defaultSSLMode
	}

	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=Asia/Shanghai",
		cfg.Database.Host,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Name,
		cfg.Database.Port,
		sslMode,
	)
}
