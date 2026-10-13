package thirdparty

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"sso-server/conf"
)

type qqRoundTripFunc func(*http.Request) (*http.Response, error)

func (f qqRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func qqTestResponse(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}
}

func Test_QQProvider_AuthCodeURL_UsesRegisteredCallbackAndState(t *testing.T) {
	provider := newQQProvider(conf.QQOAuthConfig{
		ClientID:     "app-id",
		ClientSecret: "app-key",
		RedirectURI:  "https://sso.example.com/api/auth/third/qq/callback",
	})
	parsed, err := url.Parse(provider.AuthCodeURL("csrf-state"))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Host != "graph.qq.com" || parsed.Path != "/oauth2.0/authorize" {
		t.Fatalf("unexpected authorization endpoint: %s", parsed.String())
	}
	query := parsed.Query()
	if query.Get("response_type") != "code" || query.Get("client_id") != "app-id" || query.Get("redirect_uri") != provider.cfg.RedirectURI || query.Get("state") != "csrf-state" || query.Get("scope") != "get_user_info" {
		t.Fatalf("unexpected authorization parameters: %v", query)
	}
}

func Test_QQProvider_FetchProfile_UsesOpenIDWithoutEmail(t *testing.T) {
	requests := 0
	client := &http.Client{Transport: qqRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		query := r.URL.Query()
		switch r.URL.Path {
		case "/oauth2.0/token":
			if query.Get("grant_type") != "authorization_code" || query.Get("client_id") != "app-id" || query.Get("client_secret") != "app-key" || query.Get("redirect_uri") != "https://sso.example.com/api/auth/third/qq/callback" || query.Get("code") != "auth-code" || query.Get("fmt") != "json" {
				t.Errorf("unexpected token parameters: %v", query)
			}
			return qqTestResponse(`{"access_token":"token"}`), nil
		case "/oauth2.0/me":
			if query.Get("access_token") != "token" || query.Get("fmt") != "json" {
				t.Errorf("unexpected openid parameters: %v", query)
			}
			return qqTestResponse(`{"client_id":"app-id","openid":"user-openid"}`), nil
		case "/user/get_user_info":
			if query.Get("access_token") != "token" || query.Get("oauth_consumer_key") != "app-id" || query.Get("openid") != "user-openid" {
				t.Errorf("unexpected user parameters: %v", query)
			}
			return qqTestResponse(`{"ret":0,"nickname":"QQ 用户","figureurl_qq_1":"https://example.com/avatar.png"}`), nil
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(""))}, nil
		}
	})}

	provider := newQQProvider(conf.QQOAuthConfig{ClientID: "app-id", ClientSecret: "app-key", RedirectURI: "https://sso.example.com/api/auth/third/qq/callback"})
	provider.client = client
	profile, err := provider.FetchProfile(context.Background(), "auth-code")
	if err != nil {
		t.Fatal(err)
	}
	if requests != 3 || profile.Provider != QQ || profile.ProviderUID != "app-id:user-openid" || profile.Username != "QQ 用户" || profile.Email != "" || profile.AvatarURL != "https://example.com/avatar.png" {
		t.Fatalf("unexpected QQ profile: %#v, requests=%d", profile, requests)
	}
}

func Test_QQProvider_FetchProfile_RejectsMismatchedAppIDAndUserError(t *testing.T) {
	for _, scenario := range []struct {
		name       string
		tokenBody  string
		openidBody string
		userBody   string
		calls      int
	}{
		{name: "token error", tokenBody: `{"error":100013,"error_description":"invalid code"}`, calls: 1},
		{name: "mismatched app", openidBody: `{"client_id":"other-app","openid":"user"}`, calls: 2},
		{name: "missing openid", openidBody: `{"client_id":"app-id"}`, calls: 2},
		{name: "userinfo error", openidBody: `{"client_id":"app-id","openid":"user"}`, userBody: `{"ret":1002,"msg":"not logged in"}`, calls: 3},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: qqRoundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				switch r.URL.Path {
				case "/oauth2.0/token":
					if scenario.tokenBody != "" {
						return qqTestResponse(scenario.tokenBody), nil
					}
					return qqTestResponse(`{"access_token":"token"}`), nil
				case "/oauth2.0/me":
					return qqTestResponse(scenario.openidBody), nil
				case "/user/get_user_info":
					return qqTestResponse(scenario.userBody), nil
				}
				return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(""))}, nil
			})}
			provider := newQQProvider(conf.QQOAuthConfig{ClientID: "app-id", ClientSecret: "app-key", RedirectURI: "https://sso.example.com/callback"})
			provider.client = client
			if _, err := provider.FetchProfile(context.Background(), "auth-code"); err == nil {
				t.Fatal("expected QQ response to be rejected")
			}
			if calls != scenario.calls {
				t.Fatalf("expected %d calls, got %d", scenario.calls, calls)
			}
		})
	}
}
