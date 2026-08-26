package kv

import (
	"context"
	"errors"
	"time"

	redis_rate "github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	client  *redis.Client
	limiter *redis_rate.Limiter
}

func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client, limiter: redis_rate.NewLimiter(client)}
}

func (s *RedisStore) Get(ctx context.Context, key string) (string, error) {
	val, err := s.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrNotFound
	}
	return val, err
}

func (s *RedisStore) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return s.client.Set(ctx, key, value, ttl).Err()
}

func (s *RedisStore) SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	return s.client.SetNX(ctx, key, value, ttl).Result()
}

func (s *RedisStore) Increment(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	pipe := s.client.TxPipeline()
	count := pipe.Incr(ctx, key)
	if ttl > 0 {
		pipe.Expire(ctx, key, ttl)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return count.Val(), nil
}

func (s *RedisStore) TTL(ctx context.Context, key string) (time.Duration, error) {
	ttl, err := s.client.TTL(ctx, key).Result()
	if errors.Is(err, redis.Nil) || ttl == -2*time.Second {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	if ttl < 0 {
		return 0, nil
	}
	return ttl, nil
}

func (s *RedisStore) Del(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}

func (s *RedisStore) RateLimit(ctx context.Context, key string, rate int, burst int, period time.Duration) (bool, time.Duration, error) {
	result, err := s.limiter.Allow(ctx, key, redis_rate.Limit{Rate: rate, Burst: burst, Period: period})
	if err != nil {
		return false, 0, err
	}
	return result.Allowed == 1, result.RetryAfter, nil
}

func (s *RedisStore) AddToSet(ctx context.Context, key string, value string, ttl time.Duration) error {
	pipe := s.client.TxPipeline()
	pipe.SAdd(ctx, key, value)
	if ttl > 0 {
		pipe.Expire(ctx, key, ttl)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (s *RedisStore) SetCardinality(ctx context.Context, key string) (int64, error) {
	return s.client.SCard(ctx, key).Result()
}

func (s *RedisStore) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	return s.client.Eval(ctx, script, keys, args...).Result()
}
