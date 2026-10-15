package oauth

import (
	"context"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"
)

func TestRefreshStore_AtomicRotationAndRevocation(t *testing.T) {
	store := NewMemoryRefreshStore()
	grant := RefreshGrant{UserID: "usr_1", ClientID: "android-app", Audiences: []string{"classhopper-api"}, Scopes: []string{"courses:read"}, AuthorizedAt: time.Now().UnixMicro(), ExpiresAt: time.Now().Add(time.Hour)}
	if err := store.Issue(context.Background(), "old", grant); err != nil {
		t.Fatal(err)
	}
	var successes int
	var mu sync.Mutex
	var workers sync.WaitGroup
	for i := 0; i < 20; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if err := store.Rotate(context.Background(), "old", "new", grant); err == nil {
				mu.Lock()
				successes++
				mu.Unlock()
			}
		}()
	}
	workers.Wait()
	if successes != 1 {
		t.Fatalf("expected one successful rotation, got %d", successes)
	}
	if _, err := store.Load(context.Background(), "old"); err == nil {
		t.Fatal("old refresh token remained valid")
	}
	if err := store.RevokeUser(context.Background(), "usr_1"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(context.Background(), "new"); err == nil {
		t.Fatal("refresh token survived logout")
	}
	grant.ExpiresAt = time.Now().Add(-time.Second)
	grant.AuthorizedAt = time.Now().UnixMicro() + 1
	if err := store.Issue(context.Background(), "expired", grant); err == nil {
		t.Fatal("expired refresh token accepted")
	}
}

func Test_RefreshStore_DeletionCutoffRetryPreservesNewAuthorizations(t *testing.T) {
	for _, backend := range []string{"memory", "redis"} {
		t.Run(backend, func(t *testing.T) {
			var store RefreshStore = NewMemoryRefreshStore()
			if backend == "redis" {
				server := miniredis.RunT(t)
				client := redis.NewClient(&redis.Options{Addr: server.Addr()})
				t.Cleanup(func() { _ = client.Close() })
				store = NewRedisRefreshStore(client)
			}
			cutoff := time.Now().Truncate(time.Microsecond)
			grant := RefreshGrant{UserID: "owner", ClientID: "app", AuthorizedAt: cutoff.Add(-time.Second).UnixMicro(), ExpiresAt: cutoff.Add(time.Hour)}
			require.NoError(t, store.Issue(t.Context(), "old", grant))
			require.NoError(t, store.RevokeUserBefore(t.Context(), "owner", cutoff))
			_, err := store.Load(t.Context(), "old")
			require.Error(t, err)
			grant.AuthorizedAt = cutoff.UnixMicro() + 1
			require.NoError(t, store.Issue(t.Context(), "new", grant))
			require.NoError(t, store.RevokeUserBefore(t.Context(), "owner", cutoff))
			require.NoError(t, store.RevokeUserBefore(t.Context(), "owner", cutoff.Add(-time.Hour)))
			_, err = store.Load(t.Context(), "new")
			require.NoError(t, err)
			grant.AuthorizedAt = cutoff.UnixMicro()
			require.ErrorIs(t, store.Issue(t.Context(), "stale", grant), ErrInvalidRefresh)
		})
	}
}
