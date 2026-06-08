package oauth2

import "testing"

func TestResolveRedirectURI_AllowsExactMatch(t *testing.T) {
	allowed := "https://app.example.com/callback"
	got, err := ResolveRedirectURI(allowed, "https://app.example.com/callback")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got != "https://app.example.com/callback" {
		t.Fatalf("expected requested uri, got %q", got)
	}
}

func TestResolveRedirectURI_RejectsDifferentPathOnSameHost(t *testing.T) {
	allowed := "https://app.example.com/callback"
	_, err := ResolveRedirectURI(allowed, "https://app.example.com/other/path")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestResolveRedirectURI_RejectsMissingRequest(t *testing.T) {
	allowed := "https://app.example.com/callback"
	_, err := ResolveRedirectURI(allowed, "")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}
