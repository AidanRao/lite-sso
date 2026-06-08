package oauth2

import "testing"

func TestValidateRedirectURI_AllowsExactMatch(t *testing.T) {
	base := "https://app.example.com/callback"
	if err := ValidateRedirectURI(base, "https://app.example.com/callback"); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestValidateRedirectURI_RejectsDifferentPathOnSameHost(t *testing.T) {
	base := "https://app.example.com/callback"
	if err := ValidateRedirectURI(base, "https://app.example.com/other/path"); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestValidateRedirectURI_JSONList_AllowsExactItem(t *testing.T) {
	base := `["https://app.example.com/callback","https://admin.example.com/callback"]`
	if err := ValidateRedirectURI(base, "https://admin.example.com/callback"); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestValidateRedirectURI_JSONList_RejectsDifferentPathOnSameHost(t *testing.T) {
	base := `["https://app.example.com/callback"]`
	if err := ValidateRedirectURI(base, "https://app.example.com/other/path"); err == nil {
		t.Fatalf("expected error, got nil")
	}
}
