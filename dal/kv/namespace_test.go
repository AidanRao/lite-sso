package kv

import (
	"context"
	"testing"
	"time"
)

func TestNamespacePrefix_NormalizesEnvironment(t *testing.T) {
	if got := NamespacePrefix("  PROD "); got != "prod:sso:" {
		t.Fatalf("expected prod:sso: prefix, got %q", got)
	}
	if got := NamespacePrefix(""); got != "local:sso:" {
		t.Fatalf("expected local:sso: prefix, got %q", got)
	}
}

func TestNamespacedStore_IsolatesEnvironments(t *testing.T) {
	ctx := context.Background()
	base := NewMemoryStore()
	localStore := NewNamespacedStore(base, "local")
	prodStore := NewNamespacedStore(base, "prod")

	if err := localStore.Set(ctx, KeySession("same-session"), "local-user", time.Hour); err != nil {
		t.Fatalf("set local session: %v", err)
	}
	if err := prodStore.Set(ctx, KeySession("same-session"), "prod-user", time.Hour); err != nil {
		t.Fatalf("set prod session: %v", err)
	}

	localValue, err := localStore.Get(ctx, KeySession("same-session"))
	if err != nil {
		t.Fatalf("get local session: %v", err)
	}
	if localValue != "local-user" {
		t.Fatalf("expected local-user, got %q", localValue)
	}

	prodValue, err := prodStore.Get(ctx, KeySession("same-session"))
	if err != nil {
		t.Fatalf("get prod session: %v", err)
	}
	if prodValue != "prod-user" {
		t.Fatalf("expected prod-user, got %q", prodValue)
	}

	storedValue, err := base.Get(ctx, "prod:sso:session:same-session")
	if err != nil {
		t.Fatalf("get namespaced base key: %v", err)
	}
	if storedValue != "prod-user" {
		t.Fatalf("expected prod-user at namespaced key, got %q", storedValue)
	}

	if _, err := base.Get(ctx, KeySession("same-session")); err != ErrNotFound {
		t.Fatalf("expected legacy unprefixed key to be absent, got %v", err)
	}
}
