package oauth2

import (
	"net/http/httptest"
	"testing"
)

func TestAuthorizationCodePolicy_PublicClientRequiresS256(t *testing.T) {
	policy := AuthorizationCodePolicy{}
	tests := []struct {
		name      string
		query     string
		wantError bool
	}{
		{name: "valid S256", query: "code_challenge=challenge-value-123456789012345678901234567890123456789&code_challenge_method=S256"},
		{name: "missing challenge", query: "code_challenge_method=S256", wantError: true},
		{name: "missing method", query: "code_challenge=challenge-value-123456789012345678901234567890123456789", wantError: true},
		{name: "plain method", query: "code_challenge=challenge-value-123456789012345678901234567890123456789&code_challenge_method=plain", wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest("GET", "/oauth/authorize?"+test.query, nil)
			err := policy.ValidateAuthorizationRequest(request, true)
			if (err != nil) != test.wantError {
				t.Fatalf("ValidateAuthorizationRequest() error = %v, wantError %t", err, test.wantError)
			}
		})
	}
}

func TestAuthorizationCodePolicy_ConfidentialClientAllowsOptionalS256(t *testing.T) {
	policy := AuthorizationCodePolicy{}
	for _, query := range []string{"", "code_challenge=challenge-value-123456789012345678901234567890123456789&code_challenge_method=S256"} {
		request := httptest.NewRequest("GET", "/oauth/authorize?"+query, nil)
		if err := policy.ValidateAuthorizationRequest(request, false); err != nil {
			t.Fatalf("expected confidential request to pass, got %v", err)
		}
	}
	request := httptest.NewRequest("GET", "/oauth/authorize?code_challenge_method=S256", nil)
	if err := policy.ValidateAuthorizationRequest(request, false); err == nil {
		t.Fatal("expected a PKCE method without a challenge to fail")
	}
}
