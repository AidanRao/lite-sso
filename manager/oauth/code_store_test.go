package oauth

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	gooauth2 "github.com/go-oauth2/oauth2/v4"
	"github.com/go-oauth2/oauth2/v4/models"
	"github.com/stretchr/testify/require"

	"sso-server/dal/kv"
)

func TestAuthorizationCodeStore_Lifecycle(t *testing.T) {
	base := kv.NewMemoryStore()
	store := NewAuthorizationCodeStore(kv.NewNamespacedStore(base, "test"))
	info := models.NewToken()
	info.SetCode("secret-code")
	info.SetClientID("client")
	info.SetUserID("user")
	info.SetCodeCreateAt(time.Now())
	info.SetCodeExpiresIn(time.Minute)
	info.GetExtension().Set("session_id", "session")

	require.NoError(t, store.Create(t.Context(), info))
	key := kv.NamespacePrefix("test") + kv.KeyOAuthAuthorizationCode("secret-code")
	require.NotContains(t, key, "secret-code")
	require.True(t, strings.HasPrefix(key, "test:sso:oauth:authorization-code:"))
	require.Positive(t, mustTTL(t, base, key))

	loaded, err := store.GetByCode(t.Context(), "secret-code")
	require.NoError(t, err)
	require.Equal(t, info.GetCode(), loaded.GetCode())
	require.Equal(t, info.GetClientID(), loaded.GetClientID())
	require.Equal(t, "session", loaded.(gooauth2.ExtendableTokenInfo).GetExtension().Get("session_id"))

	loaded, err = store.GetByCode(t.Context(), "secret-code")
	require.NoError(t, err)
	require.Nil(t, loaded)
	require.NoError(t, store.RemoveByCode(t.Context(), "secret-code"))
}

func TestAuthorizationCodeStore_SingleUse(t *testing.T) {
	store := NewAuthorizationCodeStore(kv.NewMemoryStore())
	info := models.NewToken()
	info.SetCode("one-use")
	info.SetCodeCreateAt(time.Now())
	info.SetCodeExpiresIn(time.Minute)
	require.NoError(t, store.Create(t.Context(), info))

	var successful atomic.Int32
	var workers sync.WaitGroup
	for range 20 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			loaded, err := store.GetByCode(t.Context(), "one-use")
			if err == nil && loaded != nil {
				successful.Add(1)
			}
		}()
	}
	workers.Wait()
	require.EqualValues(t, 1, successful.Load())
}

func TestAuthorizationCodeStore_StatelessAccessToken(t *testing.T) {
	base := &writeCountingStore{Store: kv.NewMemoryStore()}
	store := NewAuthorizationCodeStore(base)
	info := models.NewToken()
	info.SetAccess("signed.jwt.token")
	require.NoError(t, store.Create(t.Context(), info))
	require.Zero(t, base.writes)
}

func TestAuthorizationCodeStore_ExpiredCode(t *testing.T) {
	store := NewAuthorizationCodeStore(kv.NewMemoryStore())
	info := models.NewToken()
	info.SetCode("expired")
	info.SetCodeCreateAt(time.Now().Add(-time.Hour))
	info.SetCodeExpiresIn(time.Minute)
	require.ErrorIs(t, store.Create(t.Context(), info), ErrInvalidAuthorizationCodeToken)
}

func mustTTL(t *testing.T, store kv.Store, key string) time.Duration {
	t.Helper()
	ttl, err := store.TTL(t.Context(), key)
	require.NoError(t, err)
	return ttl
}

type writeCountingStore struct {
	kv.Store
	writes int
}

func (s *writeCountingStore) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	s.writes++
	return s.Store.Set(ctx, key, value, ttl)
}
