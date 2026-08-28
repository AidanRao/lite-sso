package kv

import (
	"context"
	"strconv"
	"sync"
	"time"
)

type memoryItem struct {
	value     string
	expiresAt time.Time
	hasExpiry bool
}

type MemoryStore struct {
	mu    sync.Mutex
	items map[string]memoryItem
	sets  map[string]map[string]time.Time
	rates map[string]memoryRate
}

type memoryRate struct {
	startedAt time.Time
	count     int
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		items: make(map[string]memoryItem),
		sets:  make(map[string]map[string]time.Time),
		rates: make(map[string]memoryRate),
	}
}

func (s *MemoryStore) Get(ctx context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	it, ok := s.items[key]
	if !ok {
		return "", ErrNotFound
	}
	if it.hasExpiry && time.Now().After(it.expiresAt) {
		delete(s.items, key)
		return "", ErrNotFound
	}
	return it.value, nil
}

// Take atomically reads and removes a value.
func (s *MemoryStore) Take(ctx context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	it, ok := s.items[key]
	if !ok {
		return "", ErrNotFound
	}
	delete(s.items, key)
	if it.hasExpiry && time.Now().After(it.expiresAt) {
		return "", ErrNotFound
	}
	return it.value, nil
}

func (s *MemoryStore) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	it := memoryItem{value: value}
	if ttl > 0 {
		it.hasExpiry = true
		it.expiresAt = time.Now().Add(ttl)
	}
	s.items[key] = it
	return nil
}

func (s *MemoryStore) SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if it, ok := s.items[key]; ok {
		if it.hasExpiry && time.Now().After(it.expiresAt) {
			delete(s.items, key)
		} else {
			return false, nil
		}
	}

	it := memoryItem{value: value}
	if ttl > 0 {
		it.hasExpiry = true
		it.expiresAt = time.Now().Add(ttl)
	}
	s.items[key] = it
	return true, nil
}

func (s *MemoryStore) Increment(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var current int64
	if it, ok := s.items[key]; ok {
		if it.hasExpiry && time.Now().After(it.expiresAt) {
			delete(s.items, key)
		} else if parsed, err := strconv.ParseInt(it.value, 10, 64); err == nil {
			current = parsed
		}
	}

	current += 1
	it := memoryItem{value: strconv.FormatInt(current, 10)}
	if ttl > 0 {
		it.hasExpiry = true
		it.expiresAt = time.Now().Add(ttl)
	}
	s.items[key] = it
	return current, nil
}

func (s *MemoryStore) TTL(ctx context.Context, key string) (time.Duration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	it, ok := s.items[key]
	if !ok {
		return 0, ErrNotFound
	}
	if !it.hasExpiry {
		return 0, nil
	}
	remaining := time.Until(it.expiresAt)
	if remaining <= 0 {
		delete(s.items, key)
		return 0, ErrNotFound
	}
	return remaining, nil
}

func (s *MemoryStore) Del(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, key)
	delete(s.sets, key)
	delete(s.rates, key)
	return nil
}

func (s *MemoryStore) RateLimit(ctx context.Context, key string, rate int, burst int, period time.Duration) (bool, time.Duration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	state := s.rates[key]
	if state.startedAt.IsZero() || now.Sub(state.startedAt) >= period {
		state = memoryRate{startedAt: now}
	}
	if state.count >= burst {
		retryAfter := period - now.Sub(state.startedAt)
		if retryAfter <= 0 {
			retryAfter = time.Second
		}
		s.rates[key] = state
		return false, retryAfter, nil
	}
	state.count++
	s.rates[key] = state
	return true, 0, nil
}

func (s *MemoryStore) AddToSet(ctx context.Context, key string, value string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sets[key] == nil {
		s.sets[key] = make(map[string]time.Time)
	}
	expiresAt := time.Time{}
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}
	s.sets[key][value] = expiresAt
	return nil
}

func (s *MemoryStore) SetCardinality(ctx context.Context, key string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	values := s.sets[key]
	now := time.Now()
	for value, expiresAt := range values {
		if !expiresAt.IsZero() && !now.Before(expiresAt) {
			delete(values, value)
		}
	}
	return int64(len(values)), nil
}

func (s *MemoryStore) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	return nil, ErrScriptUnsupported
}
