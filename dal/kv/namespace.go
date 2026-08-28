package kv

import (
	"context"
	"strings"
	"time"
)

const defaultEnvironmentName = "local"

// NamespacePrefix returns the Redis key prefix for an environment.
func NamespacePrefix(environment string) string {
	environment = strings.ToLower(strings.TrimSpace(environment))
	if environment == "" {
		environment = defaultEnvironmentName
	}
	return environment + ":sso:"
}

// NamespacedStore adds an environment namespace to every key operation.
type NamespacedStore struct {
	store  Store
	prefix string
}

// NewNamespacedStore creates a store that prefixes every key with the environment namespace.
func NewNamespacedStore(store Store, environment string) *NamespacedStore {
	return &NamespacedStore{
		store:  store,
		prefix: NamespacePrefix(environment),
	}
}

func (s *NamespacedStore) namespacedKey(key string) string {
	return s.prefix + key
}

func (s *NamespacedStore) Get(ctx context.Context, key string) (string, error) {
	return s.store.Get(ctx, s.namespacedKey(key))
}

func (s *NamespacedStore) Take(ctx context.Context, key string) (string, error) {
	return s.store.Take(ctx, s.namespacedKey(key))
}

func (s *NamespacedStore) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return s.store.Set(ctx, s.namespacedKey(key), value, ttl)
}

func (s *NamespacedStore) SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	return s.store.SetNX(ctx, s.namespacedKey(key), value, ttl)
}

func (s *NamespacedStore) Increment(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	return s.store.Increment(ctx, s.namespacedKey(key), ttl)
}

func (s *NamespacedStore) TTL(ctx context.Context, key string) (time.Duration, error) {
	return s.store.TTL(ctx, s.namespacedKey(key))
}

func (s *NamespacedStore) Del(ctx context.Context, key string) error {
	return s.store.Del(ctx, s.namespacedKey(key))
}

func (s *NamespacedStore) RateLimit(ctx context.Context, key string, rate int, burst int, period time.Duration) (bool, time.Duration, error) {
	securityStore, ok := s.store.(SecurityStore)
	if !ok {
		return false, 0, ErrScriptUnsupported
	}
	return securityStore.RateLimit(ctx, s.namespacedKey(key), rate, burst, period)
}

func (s *NamespacedStore) AddToSet(ctx context.Context, key string, value string, ttl time.Duration) error {
	securityStore, ok := s.store.(SecurityStore)
	if !ok {
		return ErrScriptUnsupported
	}
	return securityStore.AddToSet(ctx, s.namespacedKey(key), value, ttl)
}

func (s *NamespacedStore) SetCardinality(ctx context.Context, key string) (int64, error) {
	securityStore, ok := s.store.(SecurityStore)
	if !ok {
		return 0, ErrScriptUnsupported
	}
	return securityStore.SetCardinality(ctx, s.namespacedKey(key))
}

func (s *NamespacedStore) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	securityStore, ok := s.store.(SecurityStore)
	if !ok {
		return nil, ErrScriptUnsupported
	}
	namespacedKeys := make([]string, len(keys))
	for i, key := range keys {
		namespacedKeys[i] = s.namespacedKey(key)
	}
	return securityStore.Eval(ctx, script, namespacedKeys, args...)
}
