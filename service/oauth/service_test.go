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

func Test_OAuthService_ThirdPartyBind_PreviewsBeforeConfirmation(t *testing.T) {
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
			Email:       "alice@example.com",
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

	state := parsed.Query().Get("state")
	if !service.IsThirdPartyBindingState(ctx, githubProvider, state) {
		t.Fatal("expected binding state to be recognized before callback")
	}

	result, err := service.HandleThirdPartyCallbackWithState(ctx, githubProvider, "code", state)
	if err != nil {
		t.Fatalf("handle bind callback: %v", err)
	}
	if service.IsThirdPartyBindingState(ctx, githubProvider, state) {
		t.Fatal("expected callback to consume binding state")
	}

	if result.Action != ThirdPartyActionBind || result.User.ID != "u1" || result.PendingBindingID == "" {
		t.Fatalf("unexpected bind result: %#v", result)
	}

	var binding model.UserThirdParty
	if err := gormDB.First(&binding, "user_id = ? AND provider = ?", "u1", githubProvider).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected no binding before confirmation, got %v", err)
	}

	preview, err := service.GetThirdPartyBindingPreview(ctx, "u1", result.PendingBindingID)
	if err != nil {
		t.Fatalf("get binding preview: %v", err)
	}
	if preview.Provider != githubProvider || preview.Username != "alice" {
		t.Fatalf("unexpected preview: %#v", preview)
	}

	if _, err := service.ConfirmThirdPartyBinding(ctx, "u1", result.PendingBindingID); err != nil {
		t.Fatalf("confirm binding: %v", err)
	}
	if err := gormDB.First(&binding, "user_id = ? AND provider = ?", "u1", githubProvider).Error; err != nil {
		t.Fatalf("expected confirmed binding: %v", err)
	}
	if binding.ProviderUID != "gh_1" {
		t.Fatalf("expected provider uid gh_1, got %q", binding.ProviderUID)
	}
	var imported model.UserEmail
	if err := gormDB.First(&imported, "user_id = ? AND email = ?", "u1", "alice@example.com").Error; err != nil {
		t.Fatalf("expected imported email: %v", err)
	}
	if imported.VerifiedAt == nil || !imported.IsPrimary {
		t.Fatalf("expected trusted primary email: %#v", imported)
	}
	var sourceCount int64
	if err := gormDB.Model(&model.UserEmailSource{}).Where("user_email_id = ? AND user_third_party_id = ?", imported.ID, binding.ID).Count(&sourceCount).Error; err != nil || sourceCount != 1 {
		t.Fatalf("expected provider source, count=%d err=%v", sourceCount, err)
	}
	if _, err := service.GetThirdPartyBindingPreview(ctx, "u1", result.PendingBindingID); !errors.Is(err, common.ErrThirdPartyBindingNotFound) {
		t.Fatalf("expected consumed preview, got %v", err)
	}
}

func Test_OAuthService_ThirdPartyBind_RejectsPreviewOwnedByAnotherUser(t *testing.T) {
	ctx := context.Background()
	gormDB := newOAuthTestDB(t)
	for _, userID := range []string{"u1", "u2"} {
		if err := gormDB.Create(&model.User{ID: userID, IsActive: true}).Error; err != nil {
			t.Fatalf("create user %s: %v", userID, err)
		}
	}

	kvStore := kv.NewMemoryStore()
	service := NewOAuthService(&conf.Config{}, gormDB, kvStore, db.NewUserRepository(gormDB))
	service.providers[githubProvider] = &fakeThirdPartyProvider{
		profile: &thirdPartyProfile{Provider: githubProvider, ProviderUID: "gh_1"},
	}
	authURL, err := service.HandleThirdPartyBind(ctx, "u1", githubProvider, "/profile")
	if err != nil {
		t.Fatalf("start third party bind: %v", err)
	}
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse auth url: %v", err)
	}
	result, err := service.HandleThirdPartyCallbackWithState(ctx, githubProvider, "code", parsed.Query().Get("state"))
	if err != nil {
		t.Fatalf("handle callback: %v", err)
	}

	if _, err := service.GetThirdPartyBindingPreview(ctx, "u2", result.PendingBindingID); !errors.Is(err, common.ErrThirdPartyBindingNotFound) {
		t.Fatalf("expected inaccessible preview, got %v", err)
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

func Test_FindOrCreateUser_AllowsNewProviderAccountWithoutEmail(t *testing.T) {
	gormDB := newOAuthTestDB(t)
	service := NewOAuthService(&conf.Config{}, gormDB, kv.NewMemoryStore(), db.NewUserRepository(gormDB))

	user, err := service.findOrCreateUser(context.Background(), &thirdPartyProfile{Provider: githubProvider, ProviderUID: "gh-new"})
	if err != nil {
		t.Fatalf("create provider-only account: %v", err)
	}
	if user.Email != nil {
		t.Fatalf("expected account without email, got %#v", user.Email)
	}
	var emailCount int64
	if err := gormDB.Model(&model.UserEmail{}).Where("user_id = ?", user.ID).Count(&emailCount).Error; err != nil {
		t.Fatalf("count emails: %v", err)
	}
	if emailCount != 0 {
		t.Fatalf("expected zero emails, got %d", emailCount)
	}
}

func Test_FindOrCreateUser_DoesNotResyncEmailForEstablishedBinding(t *testing.T) {
	gormDB := newOAuthTestDB(t)
	primary := "primary@example.com"
	if err := gormDB.Create(&model.User{ID: "u1", Email: &primary, IsActive: true}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := gormDB.Create(&model.UserThirdParty{UserID: "u1", Provider: githubProvider, ProviderUID: "gh-1"}).Error; err != nil {
		t.Fatalf("create binding: %v", err)
	}
	service := NewOAuthService(&conf.Config{}, gormDB, kv.NewMemoryStore(), db.NewUserRepository(gormDB))

	if _, err := service.findOrCreateUser(context.Background(), &thirdPartyProfile{Provider: githubProvider, ProviderUID: "gh-1", Email: "changed@example.com"}); err != nil {
		t.Fatalf("find established user: %v", err)
	}
	var count int64
	if err := gormDB.Model(&model.UserEmail{}).Where("user_id = ?", "u1").Count(&count).Error; err != nil {
		t.Fatalf("count emails: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected no login-time resync, got %d emails", count)
	}
}

func newOAuthTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := "file:" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()) + "?mode=memory&cache=shared"
	gormDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := gormDB.AutoMigrate(&model.User{}, &model.UserEmail{}, &model.OAuthClient{}, &model.UserThirdParty{}, &model.UserEmailSource{}, &model.UserOAuthClient{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return gormDB
}
