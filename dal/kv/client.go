package kv

import (
	"context"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"

	"sso-server/conf"
)

var Client *redis.Client

func Init(cfg *conf.Config) error {
	opt, err := toRedisOptions(cfg.Cache.URL)
	if err != nil {
		return err
	}

	Client = redis.NewClient(opt)
	return Client.Ping(context.Background()).Err()
}

func toRedisOptions(raw string) (*redis.Options, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("redis url is empty")
	}

	return redis.ParseURL(raw)
}
