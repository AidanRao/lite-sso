package kv

import (
	"context"
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
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{items: make(map[string]memoryItem)}
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

func (s *MemoryStore) Del(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, key)
	return nil
}
