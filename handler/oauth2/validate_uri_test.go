package oauth2

import "testing"

func TestValidateRedirectURI_JSONList_AllowsExactMatch(t *testing.T) {
	base := `["https://app.example.com/callback","https://app.example.com/callback2"]`
	if err := ValidateRedirectURI(base, "https://app.example.com/callback2"); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestValidateRedirectURI_JSONList_RejectsNonMember(t *testing.T) {
	base := `["https://app.example.com/callback"]`
	if err := ValidateRedirectURI(base, "https://evil.example.com/callback"); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestValidateRedirectURI_Single_AllowsExactMatch(t *testing.T) {
	base := "https://app.example.com/callback"
	if err := ValidateRedirectURI(base, "https://app.example.com/callback"); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}
