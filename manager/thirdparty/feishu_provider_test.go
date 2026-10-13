package thirdparty

import (
	"encoding/json"
	"testing"
)

func TestFeishuUserData_ToProfile(t *testing.T) {
	profile, err := (feishuUserData{
		OpenID:          "ou_x",
		UnionID:         "on_x",
		Name:            "Alice",
		EnterpriseEmail: "alice@example.com",
		AvatarBig:       "https://example.com/avatar.png",
	}).toProfile()
	if err != nil {
		t.Fatalf("to profile: %v", err)
	}

	if profile.Provider != Feishu {
		t.Fatalf("expected provider %q, got %q", Feishu, profile.Provider)
	}
	if profile.ProviderUID != "on_x" {
		t.Fatalf("expected union id, got %q", profile.ProviderUID)
	}
	if profile.Email != "alice@example.com" {
		t.Fatalf("expected enterprise email, got %q", profile.Email)
	}
	if profile.AvatarURL != "https://example.com/avatar.png" {
		t.Fatalf("expected avatar url, got %q", profile.AvatarURL)
	}
}

func TestFeishuUserData_ToProfileAllowsMissingEmail(t *testing.T) {
	profile, err := (feishuUserData{
		OpenID:  "ou_x",
		UnionID: "on_x",
		Name:    "Alice",
	}).toProfile()
	if err != nil {
		t.Fatalf("to profile: %v", err)
	}

	if profile.ProviderUID != "on_x" {
		t.Fatalf("expected union id, got %q", profile.ProviderUID)
	}
	if profile.Email != "" {
		t.Fatalf("expected empty email, got %q", profile.Email)
	}
}

func TestFeishuTokenResponse_TopLevelAccessToken(t *testing.T) {
	var token feishuTokenResponse
	err := json.Unmarshal([]byte(`{"code":0,"msg":"success","access_token":"u-token"}`), &token)
	if err != nil {
		t.Fatalf("unmarshal token: %v", err)
	}

	if token.AccessToken != "u-token" {
		t.Fatalf("expected top-level access token, got %q", token.AccessToken)
	}
}
