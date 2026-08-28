package passkey

import (
	"context"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/conf"
	"sso-server/dal/kv"
	"sso-server/model"
	serviceauth "sso-server/service/auth"
	"sso-server/service/reauth"
	"sso-server/util/useragent"
)

func TestService_BeginRegistrationRequiresCurrentEmailOTPAndCannotReplay(t *testing.T) {
	t.Setenv("ENV", "local")
	database := newPasskeyTestDB(t)
	email := "owner@example.com"
	user := model.User{ID: uuid.NewString(), Email: &email, IsActive: true}
	require.NoError(t, database.Create(&user).Error)
	store := kv.NewMemoryStore()
	config := &conf.Config{
		Auth: conf.AuthConfig{OTPSecret: "test-secret", OTPExpire: time.Minute, OTPMaxAttempts: 5},
		Dev:  conf.DevConfig{SkipSendMessage: true, FixedEmailOTP: "123456"},
		Passkey: conf.PasskeyConfig{
			RPID: "example.com", RPOrigins: []string{"https://example.com"}, RPDisplayName: "Example", CeremonyTTL: time.Minute, ReauthTokenTTL: time.Minute,
		},
	}
	authService := serviceauth.NewAuthService(config, database, store, nil, nil)
	service := NewService(config, database, store, authService, reauth.NewService(config, store))
	require.NoError(t, store.Set(t.Context(), kv.KeyCaptcha("captcha-1"), "4321", time.Minute))

	challenge, err := service.SendRegistrationEmail(t.Context(), user.ID, "captcha-1", "4321", serviceauth.OTPRequestContext{DeviceID: "device-1", IP: "127.0.0.1"})
	require.NoError(t, err)
	result, err := service.BeginRegistration(t.Context(), user.ID, "session-1", "https://example.com", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)", challenge.ChallengeID, "123456", "device-1")
	require.NoError(t, err)
	require.NotEmpty(t, result.CeremonyID)
	require.Equal(t, "required", string(result.Options.Response.AuthenticatorSelection.ResidentKey))
	require.Equal(t, "required", string(result.Options.Response.AuthenticatorSelection.UserVerification))
	require.Equal(t, "none", string(result.Options.Response.Attestation))

	_, err = service.BeginRegistration(t.Context(), user.ID, "session-1", "https://example.com", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)", challenge.ChallengeID, "123456", "device-1")
	require.Error(t, err)
}

func TestService_SendRegistrationEmailRejectsAccountWithoutEmail(t *testing.T) {
	database := newPasskeyTestDB(t)
	user := model.User{ID: uuid.NewString(), IsActive: true}
	require.NoError(t, database.Create(&user).Error)
	store := kv.NewMemoryStore()
	config := &conf.Config{Passkey: conf.PasskeyConfig{RPID: "example.com"}}
	service := NewService(config, database, store, serviceauth.NewAuthService(config, database, store, nil, nil), reauth.NewService(config, store))

	_, err := service.SendRegistrationEmail(t.Context(), user.ID, "captcha", "answer", serviceauth.OTPRequestContext{})
	require.ErrorIs(t, err, common.ErrEmailRequiredForPasskey)
}

func TestService_CeremonyIsSingleUseAndBoundToContext(t *testing.T) {
	service := &Service{cfg: &conf.Config{Passkey: conf.PasskeyConfig{CeremonyTTL: time.Minute}}, store: kv.NewMemoryStore()}
	id, err := service.saveCeremony(context.Background(), ceremony{
		Kind: ceremonyReauth, UserID: "user-1", SessionID: "session-1", Origin: "https://sso.example.com",
		SessionData: webauthn.SessionData{Challenge: "challenge"},
	})
	require.NoError(t, err)

	state, err := service.takeCeremony(context.Background(), id, ceremonyReauth, "user-1", "session-1", "https://sso.example.com")
	require.NoError(t, err)
	require.Equal(t, "challenge", state.SessionData.Challenge)
	_, err = service.takeCeremony(context.Background(), id, ceremonyReauth, "user-1", "session-1", "https://sso.example.com")
	require.ErrorIs(t, err, common.ErrWebAuthnCeremonyInvalid)
}

func TestService_CeremonyRejectsWrongSessionAndOrigin(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		sessionID string
		origin    string
	}{
		{name: "session", sessionID: "session-2", origin: "https://sso.example.com"},
		{name: "origin", sessionID: "session-1", origin: "https://evil.example.com"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			service := &Service{cfg: &conf.Config{Passkey: conf.PasskeyConfig{CeremonyTTL: time.Minute}}, store: kv.NewMemoryStore()}
			id, err := service.saveCeremony(context.Background(), ceremony{Kind: ceremonyRegistration, UserID: "user-1", SessionID: "session-1", Origin: "https://sso.example.com"})
			require.NoError(t, err)
			_, err = service.takeCeremony(context.Background(), id, ceremonyRegistration, "user-1", testCase.sessionID, testCase.origin)
			require.ErrorIs(t, err, common.ErrWebAuthnCeremonyInvalid)
		})
	}
}

func TestService_ValidateOriginRequiresExactConfiguredOrigin(t *testing.T) {
	service := &Service{cfg: &conf.Config{Passkey: conf.PasskeyConfig{RPOrigins: []string{"https://sso.example.com"}}}}
	require.NoError(t, service.validateOrigin("https://sso.example.com"))
	require.ErrorIs(t, service.validateOrigin("https://sso.example.com:443"), common.ErrWebAuthnCeremonyInvalid)
	require.ErrorIs(t, service.validateOrigin("https://evil.example.com"), common.ErrWebAuthnCeremonyInvalid)
}

func TestDefaultCredentialName_UsesAuthenticatorCharacteristics(t *testing.T) {
	testCases := []struct {
		name       string
		credential webauthn.Credential
		expected   string
	}{
		{
			name:       "platform",
			credential: webauthn.Credential{Authenticator: webauthn.Authenticator{Attachment: protocol.Platform}, Transport: []protocol.AuthenticatorTransport{protocol.Hybrid, protocol.Internal}},
			expected:   "设备 Passkey",
		},
		{
			name:       "security key",
			credential: webauthn.Credential{Authenticator: webauthn.Authenticator{Attachment: protocol.CrossPlatform}, Transport: []protocol.AuthenticatorTransport{protocol.USB}},
			expected:   "安全密钥",
		},
		{
			name:       "hybrid",
			credential: webauthn.Credential{Authenticator: webauthn.Authenticator{Attachment: protocol.CrossPlatform}, Transport: []protocol.AuthenticatorTransport{protocol.Hybrid}},
			expected:   "跨设备 Passkey",
		},
		{
			name:       "unknown",
			credential: webauthn.Credential{},
			expected:   "Passkey",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(t, testCase.expected, defaultCredentialName(&testCase.credential, ""))
		})
	}
}

func TestDefaultCredentialName_UsesUserAgentHintForPlatformCredential(t *testing.T) {
	credential := webauthn.Credential{Authenticator: webauthn.Authenticator{Attachment: protocol.Platform}}
	require.Equal(t, "Mac Passkey", defaultCredentialName(&credential, useragent.Parse("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)").Label))
	require.Equal(t, "Windows Passkey", defaultCredentialName(&credential, useragent.Parse("Mozilla/5.0 (Windows NT 10.0; Win64; x64)").Label))
	require.Equal(t, "iPhone Passkey", defaultCredentialName(&credential, useragent.Parse("Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X)").Label))
}

func TestDefaultCredentialName_DoesNotMislabelExternalAuthenticator(t *testing.T) {
	credential := webauthn.Credential{
		Authenticator: webauthn.Authenticator{Attachment: protocol.CrossPlatform},
		Transport:     []protocol.AuthenticatorTransport{protocol.USB},
	}
	require.Equal(t, "安全密钥", defaultCredentialName(&credential, "Mac"))
}

func newPasskeyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.User{}, &model.WebAuthnUser{}, &model.WebAuthnCredential{}))
	return database
}
