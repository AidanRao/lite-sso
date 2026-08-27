package oauth

import "context"

const (
	githubProvider = "github"
	feishuProvider = "feishu"
)

type thirdPartyProvider interface {
	Configured() bool
	AuthCodeURL(state string) string
	FetchProfile(ctx context.Context, code string) (*thirdPartyProfile, error)
}

type thirdPartyProfile struct {
	Provider    string `json:"provider"`
	ProviderUID string `json:"provider_uid"`
	Email       string `json:"email"`
	Username    string `json:"username"`
	AvatarURL   string `json:"avatar_url"`
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
