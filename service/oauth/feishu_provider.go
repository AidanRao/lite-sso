package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkauthen "github.com/larksuite/oapi-sdk-go/v3/service/authen/v1"
	gxoauth2 "golang.org/x/oauth2"

	"sso-server/conf"
)

type feishuOAuthProvider struct {
	cfg conf.FeishuOAuthConfig
}

func newFeishuProvider(cfg conf.FeishuOAuthConfig) *feishuOAuthProvider {
	return &feishuOAuthProvider{cfg: cfg}
}

func (p *feishuOAuthProvider) Configured() bool {
	return p.cfg.ClientID != "" && p.cfg.ClientSecret != ""
}

func (p *feishuOAuthProvider) AuthCodeURL(state string) string {
	return p.oauthConfig().AuthCodeURL(state)
}

func (p *feishuOAuthProvider) FetchProfile(ctx context.Context, code string) (*thirdPartyProfile, error) {
	accessToken, err := p.exchangeToken(ctx, code)
	if err != nil {
		log.Printf("OAuthService: feishu token exchange failed, err=%v", err)
		return nil, err
	}

	profile, err := p.getUser(ctx, accessToken)
	if err != nil {
		log.Printf("OAuthService: feishu user info failed, err=%v", err)
		return nil, err
	}

	return profile, nil
}

func (p *feishuOAuthProvider) oauthConfig() *gxoauth2.Config {
	return &gxoauth2.Config{
		ClientID:     p.cfg.ClientID,
		ClientSecret: p.cfg.ClientSecret,
		RedirectURL:  p.cfg.RedirectURI,
		Scopes:       []string{"contact:user.email:readonly"},
		Endpoint: gxoauth2.Endpoint{
			AuthURL:  "https://accounts.feishu.cn/open-apis/authen/v1/authorize",
			TokenURL: "https://open.feishu.cn/open-apis/authen/v2/oauth/token",
		},
	}
}

func (p *feishuOAuthProvider) client() *lark.Client {
	return lark.NewClient(p.cfg.ClientID, p.cfg.ClientSecret)
}

func (p *feishuOAuthProvider) exchangeToken(ctx context.Context, code string) (string, error) {
	reqBody := feishuTokenRequest{
		GrantType:    "authorization_code",
		ClientID:     p.cfg.ClientID,
		ClientSecret: p.cfg.ClientSecret,
		Code:         code,
		RedirectURI:  p.cfg.RedirectURI,
	}

	resp, err := p.client().Post(ctx, "/open-apis/authen/v2/oauth/token", reqBody, larkcore.AccessTokenTypeNone)
	if err != nil {
		return "", formatFeishuSDKError("feishu token api", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("feishu token api http_status=%d request_id=%s body=%s", resp.StatusCode, resp.RequestId(), truncateForLog(resp.RawBody, 512))
	}

	var token feishuTokenResponse
	if err := json.Unmarshal(resp.RawBody, &token); err != nil {
		return "", err
	}
	if token.Code != 0 {
		return "", fmt.Errorf("feishu token api code=%d msg=%s", token.Code, token.Msg)
	}
	accessToken := token.AccessToken
	if accessToken == "" {
		accessToken = token.Data.AccessToken
	}
	if accessToken == "" {
		return "", fmt.Errorf("feishu token api returned empty access_token")
	}

	return accessToken, nil
}

func (p *feishuOAuthProvider) getUser(ctx context.Context, accessToken string) (*thirdPartyProfile, error) {
	resp, err := p.client().Authen.UserInfo.Get(ctx, larkcore.WithUserAccessToken(accessToken))
	if err != nil {
		return nil, formatFeishuSDKError("feishu user api", err)
	}

	if resp == nil {
		return nil, fmt.Errorf("feishu user api returned nil response")
	}

	if !resp.Success() {
		requestID := ""
		body := ""
		if resp.ApiResp != nil {
			requestID = resp.RequestId()
			body = truncateForLog(resp.RawBody, 512)
		}
		return nil, fmt.Errorf("feishu user api code=%d msg=%s request_id=%s body=%s", resp.Code, resp.Msg, requestID, body)
	}

	if resp.Data != nil {
		log.Printf(
			"OAuthService: feishu user info returned data, has_union_id=%t, has_open_id=%t, has_user_id=%t, has_email=%t, has_enterprise_email=%t",
			stringValue(resp.Data.UnionId) != "",
			stringValue(resp.Data.OpenId) != "",
			stringValue(resp.Data.UserId) != "",
			stringValue(resp.Data.Email) != "",
			stringValue(resp.Data.EnterpriseEmail) != "",
		)
	}

	return feishuRespDataToProfile(resp.Data)
}

type feishuTokenRequest struct {
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Code         string `json:"code"`
	RedirectURI  string `json:"redirect_uri"`
}

type feishuTokenResponse struct {
	Code        int             `json:"code"`
	Msg         string          `json:"msg"`
	AccessToken string          `json:"access_token"`
	Data        feishuTokenData `json:"data"`
}

type feishuTokenData struct {
	AccessToken string `json:"access_token"`
}

type feishuUserData struct {
	OpenID          string `json:"open_id"`
	UnionID         string `json:"union_id"`
	UserID          string `json:"user_id"`
	Name            string `json:"name"`
	Email           string `json:"email"`
	EnterpriseEmail string `json:"enterprise_email"`
	AvatarURL       string `json:"avatar_url"`
	AvatarBig       string `json:"avatar_big"`
	AvatarMiddle    string `json:"avatar_middle"`
}

func feishuRespDataToProfile(data *larkauthen.GetUserInfoRespData) (*thirdPartyProfile, error) {
	if data == nil {
		return nil, fmt.Errorf("feishu user info returned empty data")
	}

	return feishuUserData{
		OpenID:          stringValue(data.OpenId),
		UnionID:         stringValue(data.UnionId),
		UserID:          stringValue(data.UserId),
		Name:            stringValue(data.Name),
		Email:           stringValue(data.Email),
		EnterpriseEmail: stringValue(data.EnterpriseEmail),
		AvatarURL:       stringValue(data.AvatarUrl),
		AvatarBig:       stringValue(data.AvatarBig),
		AvatarMiddle:    stringValue(data.AvatarMiddle),
	}.toProfile()
}

func (d feishuUserData) toProfile() (*thirdPartyProfile, error) {
	providerUID := d.UnionID
	if providerUID == "" {
		providerUID = d.OpenID
	}
	if providerUID == "" {
		providerUID = d.UserID
	}

	email := d.Email
	if email == "" {
		email = d.EnterpriseEmail
	}

	avatarURL := d.AvatarURL
	if avatarURL == "" {
		avatarURL = d.AvatarBig
	}
	if avatarURL == "" {
		avatarURL = d.AvatarMiddle
	}

	if providerUID == "" {
		return nil, fmt.Errorf(
			"feishu user info missing required fields: has_union_id=%t has_open_id=%t has_user_id=%t has_email=%t has_enterprise_email=%t name_present=%t",
			d.UnionID != "",
			d.OpenID != "",
			d.UserID != "",
			d.Email != "",
			d.EnterpriseEmail != "",
			d.Name != "",
		)
	}

	return &thirdPartyProfile{
		Provider:    feishuProvider,
		ProviderUID: providerUID,
		Email:       email,
		Username:    d.Name,
		AvatarURL:   avatarURL,
	}, nil
}

func truncateForLog(body []byte, max int) string {
	if len(body) <= max {
		return string(body)
	}
	return string(body[:max]) + "...(truncated)"
}

func formatFeishuSDKError(prefix string, err error) error {
	if codeError, ok := errors.AsType[*larkcore.CodeError](err); ok {
		return fmt.Errorf("%s code=%d msg=%s detail=%s", prefix, codeError.Code, codeError.Msg, codeError.ErrorResp())
	}
	return err
}
