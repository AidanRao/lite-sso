package oauth2

import "testing"

func TestValidateRedirectURI_JSONList_AllowsSameHost(t *testing.T) {
	base := `["https://app.example.com/callback","https://app.example.com/callback2"]`
	if err := ValidateRedirectURI(base, "https://app.example.com/any/path"); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestValidateRedirectURI_JSONList_RejectsDifferentHost(t *testing.T) {
	base := `["https://app.example.com/callback"]`
	if err := ValidateRedirectURI(base, "https://evil.example.com/callback"); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestValidateRedirectURI_Single_AllowsSameHost(t *testing.T) {
	base := "https://app.example.com/callback"
	if err := ValidateRedirectURI(base, "https://app.example.com/other/path"); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestValidateRedirectURI_Single_RejectsDifferentHost(t *testing.T) {
	base := "https://app.example.com/callback"
	if err := ValidateRedirectURI(base, "https://evil.example.com/callback"); err == nil {
		t.Fatalf("expected error, got nil")
	}
}
