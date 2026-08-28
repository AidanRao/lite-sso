package kv

import (
	"context"
	"errors"
	"time"
)

type Store interface {
	Get(ctx context.Context, key string) (string, error)
	Take(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
	Increment(ctx context.Context, key string, ttl time.Duration) (int64, error)
	TTL(ctx context.Context, key string) (time.Duration, error)
	Del(ctx context.Context, key string) error
}

// SecurityStore extends Store with the atomic and aggregate operations used by
// authentication risk controls.
type SecurityStore interface {
	Store
	RateLimit(ctx context.Context, key string, rate int, burst int, period time.Duration) (bool, time.Duration, error)
	AddToSet(ctx context.Context, key string, value string, ttl time.Duration) error
	SetCardinality(ctx context.Context, key string) (int64, error)
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error)
}

var ErrScriptUnsupported = errors.New("redis script execution is unsupported")

type storeSecurityAdapter struct {
	Store
}

func NewSecurityStore(store Store) SecurityStore {
	if securityStore, ok := store.(SecurityStore); ok {
		return securityStore
	}
	return &storeSecurityAdapter{Store: store}
}

func (s *storeSecurityAdapter) RateLimit(ctx context.Context, key string, rate int, burst int, period time.Duration) (bool, time.Duration, error) {
	return false, 0, ErrScriptUnsupported
}

func (s *storeSecurityAdapter) AddToSet(ctx context.Context, key string, value string, ttl time.Duration) error {
	return ErrScriptUnsupported
}

func (s *storeSecurityAdapter) SetCardinality(ctx context.Context, key string) (int64, error) {
	return 0, ErrScriptUnsupported
}

func (s *storeSecurityAdapter) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	return nil, ErrScriptUnsupported
}
