package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	gxoauth2 "golang.org/x/oauth2"
	"golang.org/x/oauth2/github"

	"sso-server/conf"
)

type githubOAuthProvider struct {
	cfg conf.GitHubOAuthConfig
}

func newGitHubProvider(cfg conf.GitHubOAuthConfig) *githubOAuthProvider {
	return &githubOAuthProvider{cfg: cfg}
}

func (p *githubOAuthProvider) Configured() bool {
	return p.cfg.ClientID != "" && p.cfg.ClientSecret != ""
}

func (p *githubOAuthProvider) AuthCodeURL(state string) string {
	return p.oauthConfig().AuthCodeURL(state)
}

func (p *githubOAuthProvider) FetchProfile(ctx context.Context, code string) (*thirdPartyProfile, error) {
	token, err := p.oauthConfig().Exchange(ctx, code)
	if err != nil {
		return nil, err
	}

	user, err := p.getUser(ctx, token.AccessToken)
	if err != nil {
		return nil, err
	}

	return user.toProfile(), nil
}

func (p *githubOAuthProvider) oauthConfig() *gxoauth2.Config {
	return &gxoauth2.Config{
		ClientID:     p.cfg.ClientID,
		ClientSecret: p.cfg.ClientSecret,
		RedirectURL:  p.cfg.RedirectURI,
		Scopes:       []string{"user:email", "read:user"},
		Endpoint:     github.Endpoint,
	}
}

func (p *githubOAuthProvider) getUser(ctx context.Context, accessToken string) (*githubUserResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api error: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var user githubUserResponse
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, err
	}

	if user.Email == nil || *user.Email == "" {
		user.Email = p.getPrimaryEmail(ctx, accessToken)
	}

	return &user, nil
}

func (p *githubOAuthProvider) getPrimaryEmail(ctx context.Context, accessToken string) *string {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user/emails", nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	var emails []githubEmailResponse
	if err := json.Unmarshal(body, &emails); err != nil {
		return nil
	}

	for _, email := range emails {
		if email.Primary && email.Verified {
			return &email.Email
		}
	}

	return nil
}

type githubUserResponse struct {
	ID        int     `json:"id"`
	Login     string  `json:"login"`
	Email     *string `json:"email"`
	AvatarURL string  `json:"avatar_url"`
	Name      string  `json:"name"`
}

func (r *githubUserResponse) toProfile() *thirdPartyProfile {
	email := ""
	if r.Email != nil {
		email = *r.Email
	}

	username := r.Name
	if username == "" {
		username = r.Login
	}

	return &thirdPartyProfile{
		Provider:    githubProvider,
		ProviderUID: fmt.Sprintf("%d", r.ID),
		Email:       email,
		Username:    username,
		AvatarURL:   r.AvatarURL,
	}
}

type githubEmailResponse struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}
