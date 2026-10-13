package thirdparty

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"sso-server/conf"
)

const qqAPIBaseURL = "https://graph.qq.com"

type qqOAuthProvider struct {
	cfg     conf.QQOAuthConfig
	client  *http.Client
	baseURL string
}

type qqTokenResponse struct {
	AccessToken      string `json:"access_token"`
	Error            any    `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type qqOpenIDResponse struct {
	ClientID         string `json:"client_id"`
	OpenID           string `json:"openid"`
	Error            any    `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type qqUserResponse struct {
	Ret          int    `json:"ret"`
	Msg          string `json:"msg"`
	Nickname     string `json:"nickname"`
	FigureURLQQ2 string `json:"figureurl_qq_2"`
	FigureURLQQ1 string `json:"figureurl_qq_1"`
}

func newQQProvider(cfg conf.QQOAuthConfig) *qqOAuthProvider {
	return &qqOAuthProvider{
		cfg:     cfg,
		client:  &http.Client{Timeout: 20 * time.Second},
		baseURL: qqAPIBaseURL,
	}
}

func (p *qqOAuthProvider) Configured() bool {
	return p.cfg.IsConfigured()
}

func (p *qqOAuthProvider) AuthCodeURL(state string) string {
	query := url.Values{
		"response_type": {"code"},
		"client_id":     {p.cfg.ClientID},
		"redirect_uri":  {p.cfg.RedirectURI},
		"state":         {state},
		"scope":         {"get_user_info"},
	}
	return p.baseURL + "/oauth2.0/authorize?" + query.Encode()
}

func (p *qqOAuthProvider) FetchProfile(ctx context.Context, code string) (*Profile, error) {
	tokenQuery := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {p.cfg.ClientID},
		"client_secret": {p.cfg.ClientSecret},
		"code":          {code},
		"redirect_uri":  {p.cfg.RedirectURI},
		"fmt":           {"json"},
	}
	var token qqTokenResponse
	if err := p.getJSON(ctx, "/oauth2.0/token", tokenQuery, &token); err != nil {
		return nil, fmt.Errorf("qq token exchange: %w", err)
	}
	if token.Error != nil {
		return nil, fmt.Errorf("qq token exchange failed: error=%v description=%q", token.Error, truncateQQErrorMessage(token.ErrorDescription))
	}
	if token.AccessToken == "" {
		return nil, fmt.Errorf("qq token exchange returned empty access_token")
	}

	openIDQuery := url.Values{"access_token": {token.AccessToken}, "fmt": {"json"}}
	var identity qqOpenIDResponse
	if err := p.getJSON(ctx, "/oauth2.0/me", openIDQuery, &identity); err != nil {
		return nil, fmt.Errorf("qq openid lookup: %w", err)
	}
	if identity.Error != nil || identity.OpenID == "" || identity.ClientID != p.cfg.ClientID {
		return nil, fmt.Errorf("qq openid lookup failed: error=%v description=%q has_openid=%t client_id_match=%t", identity.Error, truncateQQErrorMessage(identity.ErrorDescription), identity.OpenID != "", identity.ClientID == p.cfg.ClientID)
	}

	userQuery := url.Values{
		"access_token":       {token.AccessToken},
		"oauth_consumer_key": {p.cfg.ClientID},
		"openid":             {identity.OpenID},
	}
	var user qqUserResponse
	if err := p.getJSON(ctx, "/user/get_user_info", userQuery, &user); err != nil {
		return nil, fmt.Errorf("qq user info: %w", err)
	}
	if user.Ret != 0 {
		return nil, fmt.Errorf("qq user info failed: ret=%d msg=%q", user.Ret, truncateQQErrorMessage(user.Msg))
	}

	avatarURL := user.FigureURLQQ2
	if avatarURL == "" {
		avatarURL = user.FigureURLQQ1
	}
	return &Profile{
		Provider:    QQ,
		ProviderUID: p.cfg.ClientID + ":" + identity.OpenID,
		Username:    user.Nickname,
		AvatarURL:   avatarURL,
	}, nil
}

func truncateQQErrorMessage(message string) string {
	const maxLength = 256
	runes := []rune(message)
	if len(runes) > maxLength {
		return string(runes[:maxLength])
	}
	return message
}

func (p *qqOAuthProvider) getJSON(ctx context.Context, path string, query url.Values, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+path+"?"+query.Encode(), nil)
	if err != nil {
		return fmt.Errorf("invalid qq request")
	}
	resp, err := p.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var requestErr *url.Error
		if errors.As(err, &requestErr) {
			return fmt.Errorf("qq request failed: %w", requestErr.Err)
		}
		return fmt.Errorf("qq request failed: error_type=%T", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("qq returned http status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024+1))
	if err != nil {
		return err
	}
	if len(body) > 64*1024 {
		return fmt.Errorf("qq response too large")
	}
	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("invalid qq response: %w", err)
	}
	return nil
}
