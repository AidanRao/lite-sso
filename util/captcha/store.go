package captcha

import (
	"context"
	"strings"
	"time"

	"github.com/mojocn/base64Captcha"

	"sso-server/dal/kv"
)

type Store struct {
	kv  kv.Store
	ttl time.Duration
}

func NewStore(kvStore kv.Store, ttl time.Duration) base64Captcha.Store {
	return &Store{kv: kvStore, ttl: ttl}
}

func (s *Store) Set(id string, value string) error {
	return s.kv.Set(context.Background(), kv.KeyCaptcha(id), strings.ToLower(value), s.ttl)
}

func (s *Store) Get(id string, clear bool) string {
	val, err := s.kv.Get(context.Background(), kv.KeyCaptcha(id))
	if err != nil {
		return ""
	}
	if clear {
		_ = s.kv.Del(context.Background(), kv.KeyCaptcha(id))
	}
	return val
}

func (s *Store) Verify(id string, answer string, clear bool) bool {
	val := s.Get(id, false)
	if val == "" {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(answer), strings.TrimSpace(val)) {
		return false
	}
	if clear {
		_ = s.kv.Del(context.Background(), kv.KeyCaptcha(id))
	}
	return true
}
