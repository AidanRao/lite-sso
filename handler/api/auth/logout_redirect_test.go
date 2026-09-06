package auth

import (
	"testing"

	"sso-server/model"
)

func Test_IsAllowedLogoutRedirect_RejectsUnsafeBrowserTargets(t *testing.T) {
	clients := []model.OAuthClient{{HomepageURL: "https://app.example.com"}}
	for _, target := range []string{"javascript://app.example.com/alert(1)", "//evil.example.com", "//app.example.com", "/\\evil.example.com", "https://user@app.example.com", "https://app.example.com.evil.com"} {
		t.Run(target, func(t *testing.T) {
			if isAllowedLogoutRedirect(clients, target) {
				t.Fatalf("accepted unsafe redirect %q", target)
			}
		})
	}
	for _, target := range []string{"/login", "https://app.example.com/home?from=logout"} {
		if !isAllowedLogoutRedirect(clients, target) {
			t.Fatalf("rejected valid redirect %q", target)
		}
	}
}
