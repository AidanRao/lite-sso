package db

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"sso-server/conf"
)

func Test_BuildDSN_SSLModeConfigured(t *testing.T) {
	cfg := &conf.Config{
		Database: conf.DatabaseConfig{
			Host:     "db.example.com",
			Port:     "5432",
			User:     "sso",
			Password: "secret",
			Name:     "sso",
			SSLMode:  "require",
		},
	}

	dsn := buildDSN(cfg)

	assert.Equal(t, "host=db.example.com user=sso password=secret dbname=sso port=5432 sslmode=require TimeZone=Asia/Shanghai", dsn)
}

func Test_BuildDSN_SSLModeDefaultsToDisable(t *testing.T) {
	cfg := &conf.Config{
		Database: conf.DatabaseConfig{
			Host: "localhost",
			Port: "5432",
			User: "postgres",
			Name: "sso",
		},
	}

	dsn := buildDSN(cfg)

	assert.Contains(t, dsn, "sslmode=disable")
}

func Test_BuildDSN_SSLModeTrimsWhitespace(t *testing.T) {
	cfg := &conf.Config{
		Database: conf.DatabaseConfig{
			SSLMode: "  verify-full  ",
		},
	}

	dsn := buildDSN(cfg)

	assert.Contains(t, dsn, "sslmode=verify-full")
}
