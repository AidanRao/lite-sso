package oauth2

import "testing"

func TestResolveRedirectURI_DefaultsToFirstAllowed(t *testing.T) {
	allowed := `["https://app.example.com/callback","https://app.example.com/callback2"]`
	got, err := ResolveRedirectURI(allowed, "")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got != "https://app.example.com/callback" {
		t.Fatalf("expected first redirect uri, got %q", got)
	}
}

func TestResolveRedirectURI_AllowsSameHost(t *testing.T) {
	allowed := `["https://app.example.com/callback"]`
	got, err := ResolveRedirectURI(allowed, "https://app.example.com/other/path")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got != "https://app.example.com/other/path" {
		t.Fatalf("expected requested uri, got %q", got)
	}
}

func TestResolveRedirectURI_RejectsDifferentHost(t *testing.T) {
	allowed := `["https://app.example.com/callback"]`
	_, err := ResolveRedirectURI(allowed, "https://evil.example.com/callback")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}
