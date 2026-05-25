package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/conf"
	"sso-server/dal/db"
	"sso-server/dal/kv"
	"sso-server/model"
)

type fakeThirdPartyProvider struct {
	profile *thirdPartyProfile
}

func (p *fakeThirdPartyProvider) Configured() bool {
	return true
}

func (p *fakeThirdPartyProvider) AuthCodeURL(state string) string {
	return "https://provider.example/oauth?state=" + url.QueryEscape(state)
}

func (p *fakeThirdPartyProvider) FetchProfile(ctx context.Context, code string) (*thirdPartyProfile, error) {
	return p.profile, nil
}

func TestOAuthService_HandleThirdPartyLogin_FeishuBuildsAuthURL(t *testing.T) {
	cfg := &conf.Config{
		OAuth: conf.ThirdPartyOAuthConfig{
			Feishu: conf.FeishuOAuthConfig{
				ClientID:     "cli_feishu",
				ClientSecret: "secret",
				RedirectURI:  "http://localhost:8080/api/auth/third/feishu/callback",
			},
		},
	}
	service := NewOAuthService(cfg, nil, kv.NewMemoryStore(), nil)

	authURL, err := service.HandleThirdPartyLogin(context.Background(), feishuProvider, "/profile")
	if err != nil {
		t.Fatalf("handle feishu login: %v", err)
	}

	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse auth url: %v", err)
	}

	if parsed.Scheme != "https" || parsed.Host != "accounts.feishu.cn" || parsed.Path != "/open-apis/authen/v1/authorize" {
		t.Fatalf("unexpected auth url: %s", authURL)
	}
	query := parsed.Query()
	if query.Get("client_id") != cfg.OAuth.Feishu.ClientID {
		t.Fatalf("expected client_id %q, got %q", cfg.OAuth.Feishu.ClientID, query.Get("client_id"))
	}
	if query.Get("redirect_uri") != cfg.OAuth.Feishu.RedirectURI {
		t.Fatalf("expected redirect_uri %q, got %q", cfg.OAuth.Feishu.RedirectURI, query.Get("redirect_uri"))
	}
	if query.Get("response_type") != "code" {
		t.Fatalf("expected response_type code, got %q", query.Get("response_type"))
	}
	if query.Get("state") == "" {
		t.Fatal("expected state")
	}
}

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

	if profile.Provider != feishuProvider {
		t.Fatalf("expected provider %q, got %q", feishuProvider, profile.Provider)
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

func TestOAuthService_ThirdPartyBind_BindsCurrentUser(t *testing.T) {
	ctx := context.Background()
	gormDB := newOAuthTestDB(t)
	if err := gormDB.Create(&model.User{ID: "u1", IsActive: true}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	kvStore := kv.NewMemoryStore()
	service := NewOAuthService(&conf.Config{}, gormDB, kvStore, db.NewUserRepository(gormDB))
	service.providers[githubProvider] = &fakeThirdPartyProvider{
		profile: &thirdPartyProfile{
			Provider:    githubProvider,
			ProviderUID: "gh_1",
			Username:    "alice",
		},
	}

	authURL, err := service.HandleThirdPartyBind(ctx, "u1", githubProvider, "/profile")
	if err != nil {
		t.Fatalf("handle bind: %v", err)
	}

	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse auth url: %v", err)
	}

	result, err := service.HandleThirdPartyCallbackWithState(ctx, githubProvider, "code", parsed.Query().Get("state"))
	if err != nil {
		t.Fatalf("handle bind callback: %v", err)
	}

	if result.Action != ThirdPartyActionBind || result.User.ID != "u1" {
		t.Fatalf("unexpected bind result: %#v", result)
	}

	var binding model.UserThirdParty
	if err := gormDB.First(&binding, "user_id = ? AND provider = ?", "u1", githubProvider).Error; err != nil {
		t.Fatalf("expected binding: %v", err)
	}
	if binding.ProviderUID != "gh_1" {
		t.Fatalf("expected provider uid gh_1, got %q", binding.ProviderUID)
	}
}

func TestOAuthService_ThirdPartyBind_RejectsAccountBoundToAnotherUser(t *testing.T) {
	ctx := context.Background()
	gormDB := newOAuthTestDB(t)
	if err := gormDB.Create(&model.User{ID: "u1", IsActive: true}).Error; err != nil {
		t.Fatalf("create user u1: %v", err)
	}
	if err := gormDB.Create(&model.User{ID: "u2", IsActive: true}).Error; err != nil {
		t.Fatalf("create user u2: %v", err)
	}
	if err := gormDB.Create(&model.UserThirdParty{
		UserID:      "u2",
		Provider:    githubProvider,
		ProviderUID: "gh_1",
	}).Error; err != nil {
		t.Fatalf("create binding: %v", err)
	}

	kvStore := kv.NewMemoryStore()
	service := NewOAuthService(&conf.Config{}, gormDB, kvStore, db.NewUserRepository(gormDB))
	service.providers[githubProvider] = &fakeThirdPartyProvider{
		profile: &thirdPartyProfile{
			Provider:    githubProvider,
			ProviderUID: "gh_1",
		},
	}

	authURL, err := service.HandleThirdPartyBind(ctx, "u1", githubProvider, "/profile")
	if err != nil {
		t.Fatalf("handle bind: %v", err)
	}
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse auth url: %v", err)
	}

	_, err = service.HandleThirdPartyCallbackWithState(ctx, githubProvider, "code", parsed.Query().Get("state"))
	if !errors.Is(err, common.ErrThirdPartyBoundToAnother) {
		t.Fatalf("expected ErrThirdPartyBoundToAnother, got %v", err)
	}
}

func newOAuthTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := "file:" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()) + "?mode=memory&cache=shared"
	gormDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := gormDB.AutoMigrate(&model.User{}, &model.OAuthClient{}, &model.UserThirdParty{}, &model.UserOAuthClient{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return gormDB
}
