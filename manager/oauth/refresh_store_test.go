package oauth

import (
	"context"
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
