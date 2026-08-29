package reauth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/conf"
	"sso-server/dal/kv"
	"sso-server/model"
	serviceauth "sso-server/service/auth"
)

func TestService_IssuedGrantIsReusableAndSessionBound(t *testing.T) {
	store := kv.NewMemoryStore()
	config := &conf.Config{Auth: conf.AuthConfig{ReauthTokenTTL: time.Minute}}
	service := NewService(Deps{Config: config, Store: store})
	result, err := service.Issue(context.Background(), "user-1", "session-1", MethodPasskey, "credential-1")
	require.NoError(t, err)
	require.NotEmpty(t, result.Token)
	require.Equal(t, "Reauth", result.TokenType)

	first, err := service.Authorize(context.Background(), result.Token, "user-1", "session-1")
	require.NoError(t, err)
	second, err := service.Authorize(context.Background(), result.Token, "user-1", "session-1")
	require.NoError(t, err)
	require.Equal(t, first.ProofID, second.ProofID)
	require.Equal(t, MethodPasskey, first.Method)
	sessionGrant, err := service.AuthorizeSession(context.Background(), "user-1", "session-1")
	require.NoError(t, err)
	require.Equal(t, first, sessionGrant)
	reloadedService := NewService(Deps{Config: config, Store: store})
	reloadedGrant, err := reloadedService.AuthorizeSession(context.Background(), "user-1", "session-1")
	require.NoError(t, err)
	require.Equal(t, first, reloadedGrant)

	_, err = service.Authorize(context.Background(), result.Token, "user-1", "session-2")
	require.ErrorIs(t, err, common.ErrReauthTokenInvalid)
	_, err = service.Authorize(context.Background(), result.Token, "user-2", "session-1")
	require.ErrorIs(t, err, common.ErrReauthTokenInvalid)
	_, err = service.AuthorizeSession(context.Background(), "user-1", "session-2")
	require.ErrorIs(t, err, common.ErrReauthRequired)
	_, err = service.AuthorizeSession(context.Background(), "user-2", "session-1")
	require.ErrorIs(t, err, common.ErrReauthRequired)
}

func TestService_DescribeOrdersAvailableMethodsAndMasksEmail(t *testing.T) {
	database := newReauthTestDB(t)
	email := "owner@example.com"
	require.NoError(t, database.Create(&model.User{ID: "user-1", Email: &email, IsActive: true}).Error)
	require.NoError(t, database.Create(&model.WebAuthnCredential{
		ID: "credential-1", RPID: "example.com", UserID: "user-1", CredentialID: []byte("credential"), PublicKey: []byte("key"),
		AttestationType: "none", AttestationFormat: "none", TransportsJSON: "[]", Attachment: "platform", ExtensionsJSON: "{}", Name: "Passkey",
	}).Error)
	service := NewService(Deps{
		Config: &conf.Config{Auth: conf.AuthConfig{ReauthTokenTTL: 5 * time.Minute}, Passkey: conf.PasskeyConfig{RPID: "example.com"}},
		DB:     database, Store: kv.NewMemoryStore(),
	})

	descriptor, err := service.Describe(t.Context(), "user-1")
	require.NoError(t, err)
	require.Equal(t, []string{MethodPasskey, MethodEmail}, descriptor.Methods)
	require.Equal(t, 300, descriptor.MaxAge)
	require.Equal(t, "o***@example.com", descriptor.EmailHint)
}

func TestService_EmailChallengeIsSessionBoundSingleUseAndIssuesGrant(t *testing.T) {
	t.Setenv("ENV", "local")
	database := newReauthTestDB(t)
	email := "owner@example.com"
	require.NoError(t, database.Create(&model.User{ID: "user-1", Email: &email, IsActive: true}).Error)
	store := kv.NewMemoryStore()
	config := &conf.Config{
		Auth: conf.AuthConfig{OTPSecret: "test-secret", OTPExpire: time.Minute, OTPMaxAttempts: 5, ReauthTokenTTL: 5 * time.Minute},
		Dev:  conf.DevConfig{SkipSendMessage: true, FixedEmailOTP: "123456"},
	}
	authService := serviceauth.NewAuthService(config, database, store, nil, nil)
	service := NewService(Deps{Config: config, DB: database, Store: store, Auth: authService})
	require.NoError(t, store.Set(t.Context(), kv.KeyCaptcha("captcha-1"), "4321", time.Minute))

	challenge, err := service.BeginEmail(t.Context(), "user-1", "session-1", "device-1", "captcha-1", "4321", serviceauth.OTPRequestContext{DeviceID: "device-1", IP: "192.0.2.1"})
	require.NoError(t, err)

	_, err = service.FinishEmail(t.Context(), "user-1", "session-2", "device-1", "192.0.2.1", challenge.ChallengeID, "123456")
	require.ErrorIs(t, err, common.ErrChallengeInvalid)

	grant, err := service.FinishEmail(t.Context(), "user-1", "session-1", "device-1", "192.0.2.1", challenge.ChallengeID, "123456")
	require.NoError(t, err)
	require.Equal(t, "Reauth", grant.TokenType)
	authorized, err := service.Authorize(t.Context(), grant.Token, "user-1", "session-1")
	require.NoError(t, err)
	require.Equal(t, MethodEmail, authorized.Method)

	_, err = service.FinishEmail(t.Context(), "user-1", "session-1", "device-1", "192.0.2.1", challenge.ChallengeID, "123456")
	require.ErrorIs(t, err, common.ErrChallengeInvalid)
}

func TestService_BeginEmailRequiresValidCaptcha(t *testing.T) {
	t.Setenv("ENV", "local")
	database := newReauthTestDB(t)
	email := "owner@example.com"
	require.NoError(t, database.Create(&model.User{ID: "user-1", Email: &email, IsActive: true}).Error)
	store := kv.NewMemoryStore()
	config := &conf.Config{Auth: conf.AuthConfig{OTPSecret: "test-secret", OTPExpire: time.Minute, OTPMaxAttempts: 5}, Dev: conf.DevConfig{SkipSendMessage: true, FixedEmailOTP: "123456"}}
	authService := serviceauth.NewAuthService(config, database, store, nil, nil)
	service := NewService(Deps{Config: config, DB: database, Store: store, Auth: authService})

	_, err := service.BeginEmail(t.Context(), "user-1", "session-1", "device-1", "missing", "4321", serviceauth.OTPRequestContext{DeviceID: "device-1", IP: "192.0.2.1"})
	require.ErrorIs(t, err, common.ErrInvalidCaptcha)
}

func newReauthTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.User{}, &model.WebAuthnCredential{}))
	return database
}

func TestService_MissingAndExpiredToken(t *testing.T) {
	service := NewService(Deps{Config: &conf.Config{Auth: conf.AuthConfig{ReauthTokenTTL: time.Millisecond}}, Store: kv.NewMemoryStore()})
	_, err := service.Authorize(context.Background(), "", "user-1", "session-1")
	require.ErrorIs(t, err, common.ErrReauthRequired)

	result, err := service.Issue(context.Background(), "user-1", "session-1", MethodPasskey, "credential-1")
	require.NoError(t, err)
	time.Sleep(5 * time.Millisecond)
	_, err = service.Authorize(context.Background(), result.Token, "user-1", "session-1")
	require.ErrorIs(t, err, common.ErrReauthTokenInvalid)
	_, err = service.AuthorizeSession(context.Background(), "user-1", "session-1")
	require.ErrorIs(t, err, common.ErrReauthRequired)
}
