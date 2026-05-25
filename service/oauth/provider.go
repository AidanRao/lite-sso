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
	Provider    string
	ProviderUID string
	Email       string
	Username    string
	AvatarURL   string
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
