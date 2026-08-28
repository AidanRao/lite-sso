// Package passkey implements Passkey registration, assertion verification, and lifecycle management.
package passkey

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/conf"
	"sso-server/dal/db"
	"sso-server/dal/kv"
	"sso-server/model"
	serviceauth "sso-server/service/auth"
	"sso-server/service/reauth"
	"sso-server/util/useragent"
)

type ceremonyKind string

const (
	ceremonyRegistration ceremonyKind = "registration"
	ceremonyReauth       ceremonyKind = "reauth"
)

type ceremony struct {
	Kind        ceremonyKind         `json:"kind"`
	UserID      string               `json:"user_id"`
	SessionID   string               `json:"session_id"`
	Origin      string               `json:"origin"`
	DeviceLabel string               `json:"device_label,omitempty"`
	SessionData webauthn.SessionData `json:"session_data"`
	ExpiresAt   time.Time            `json:"expires_at"`
}

type webAuthnUser struct {
	id          []byte
	name        string
	displayName string
	credentials []webauthn.Credential
}

func (u webAuthnUser) WebAuthnID() []byte                         { return u.id }
func (u webAuthnUser) WebAuthnName() string                       { return u.name }
func (u webAuthnUser) WebAuthnDisplayName() string                { return u.displayName }
func (u webAuthnUser) WebAuthnCredentials() []webauthn.Credential { return u.credentials }

// Credential describes a Passkey without exposing key material.
type Credential struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Attachment     string     `json:"attachment"`
	BackupEligible bool       `json:"backup_eligible"`
	BackupState    bool       `json:"backup_state"`
	CreatedAt      time.Time  `json:"created_at"`
	LastUsedAt     *time.Time `json:"last_used_at"`
}

// RegistrationOptions contains browser creation options and the opaque ceremony identifier.
type RegistrationOptions struct {
	CeremonyID string                       `json:"ceremony_id"`
	Options    *protocol.CredentialCreation `json:"options"`
}

// AuthenticationOptions contains browser assertion options and the opaque ceremony identifier.
type AuthenticationOptions struct {
	CeremonyID string                        `json:"ceremony_id"`
	Options    *protocol.CredentialAssertion `json:"options"`
}

// Service coordinates WebAuthn ceremonies and credential persistence.
type Service struct {
	cfg      *conf.Config
	database *gorm.DB
	store    kv.Store
	auth     *serviceauth.AuthService
	reauth   *reauth.Service
	repo     *db.WebAuthnRepository
}

// NewService creates a Passkey service.
func NewService(cfg *conf.Config, database *gorm.DB, store kv.Store, authService *serviceauth.AuthService, reauthService *reauth.Service) *Service {
	return &Service{cfg: cfg, database: database, store: store, auth: authService, reauth: reauthService, repo: db.NewWebAuthnRepository(database)}
}

// List returns Passkeys belonging to the current user and relying party.
func (s *Service) List(ctx context.Context, userID string) ([]Credential, error) {
	records, err := s.repo.ListCredentials(ctx, s.cfg.Passkey.RPID, userID)
	if err != nil {
		return nil, err
	}
	result := make([]Credential, 0, len(records))
	for i := range records {
		flags := webauthn.NewCredentialFlags(protocol.AuthenticatorFlags(records[i].Flags))
		result = append(result, Credential{ID: records[i].ID, Name: records[i].Name, Attachment: records[i].Attachment, BackupEligible: flags.BackupEligible, BackupState: flags.BackupState, CreatedAt: records[i].CreatedAt, LastUsedAt: records[i].LastUsedAt})
	}
	return result, nil
}

// SendRegistrationEmail sends an OTP to the current account email.
func (s *Service) SendRegistrationEmail(ctx context.Context, userID string, captchaID string, captcha string, requestContext serviceauth.OTPRequestContext) (*serviceauth.ChallengeResult, error) {
	user, err := db.NewUserRepository(s.database).FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.Email == nil || strings.TrimSpace(*user.Email) == "" {
		return nil, common.ErrEmailRequiredForPasskey
	}
	return s.auth.SendEmailOTP(ctx, *user.Email, captchaID, captcha, requestContext, serviceauth.ChallengePurposePasskeyRegistration)
}

// BeginRegistration verifies the email OTP before creating a single-use WebAuthn ceremony.
func (s *Service) BeginRegistration(ctx context.Context, userID string, sessionID string, origin string, userAgent string, challengeID string, code string, deviceID string) (*RegistrationOptions, error) {
	if err := s.validateOrigin(origin); err != nil {
		return nil, err
	}
	user, err := db.NewUserRepository(s.database).FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.Email == nil || strings.TrimSpace(*user.Email) == "" {
		return nil, common.ErrEmailRequiredForPasskey
	}
	email, err := s.auth.VerifyChallengeForPurpose(ctx, challengeID, code, deviceID, serviceauth.ChallengePurposePasskeyRegistration)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(strings.TrimSpace(email), strings.TrimSpace(*user.Email)) {
		return nil, common.ErrWebAuthnCeremonyInvalid
	}

	webUser, err := s.loadUser(ctx, user, true)
	if err != nil {
		return nil, err
	}
	instance, err := s.instance(origin)
	if err != nil {
		return nil, err
	}
	exclusions := make([]protocol.CredentialDescriptor, 0, len(webUser.credentials))
	for i := range webUser.credentials {
		exclusions = append(exclusions, webUser.credentials[i].Descriptor())
	}
	creation, sessionData, err := instance.BeginRegistration(webUser,
		webauthn.WithExclusions(exclusions),
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{ResidentKey: protocol.ResidentKeyRequirementRequired, RequireResidentKey: protocol.ResidentKeyRequired(), UserVerification: protocol.VerificationRequired}),
		webauthn.WithConveyancePreference(protocol.PreferNoAttestation),
	)
	if err != nil {
		return nil, err
	}
	ceremonyID, err := s.saveCeremony(ctx, ceremony{
		Kind:        ceremonyRegistration,
		UserID:      userID,
		SessionID:   sessionID,
		Origin:      origin,
		DeviceLabel: useragent.Parse(userAgent).Label,
		SessionData: *sessionData,
	})
	if err != nil {
		return nil, err
	}
	return &RegistrationOptions{CeremonyID: ceremonyID, Options: creation}, nil
}

// FinishRegistration verifies a registration response and stores the credential.
func (s *Service) FinishRegistration(ctx context.Context, userID string, sessionID string, origin string, ceremonyID string, response json.RawMessage) (*Credential, error) {
	state, err := s.takeCeremony(ctx, ceremonyID, ceremonyRegistration, userID, sessionID, origin)
	if err != nil {
		return nil, err
	}
	user, err := db.NewUserRepository(s.database).FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	webUser, err := s.loadUser(ctx, user, false)
	if err != nil {
		return nil, err
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes(response)
	if err != nil {
		return nil, common.ErrWebAuthnCeremonyInvalid
	}
	instance, err := s.instance(origin)
	if err != nil {
		return nil, err
	}
	verified, err := instance.CreateCredential(webUser, state.SessionData, parsed)
	if err != nil {
		return nil, common.ErrWebAuthnCeremonyInvalid
	}
	extensions, err := json.Marshal(parsed.ClientExtensionResults)
	if err != nil {
		return nil, err
	}
	record, err := s.repo.CreateCredential(ctx, s.cfg.Passkey.RPID, userID, defaultCredentialName(verified, state.DeviceLabel), string(extensions), verified)
	if err != nil {
		return nil, err
	}
	return recordView(record), nil
}

// BeginReauth starts a Passkey assertion for the current user and session.
func (s *Service) BeginReauth(ctx context.Context, userID string, sessionID string, origin string) (*AuthenticationOptions, error) {
	if err := s.validateOrigin(origin); err != nil {
		return nil, err
	}
	user, err := db.NewUserRepository(s.database).FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	webUser, err := s.loadUser(ctx, user, false)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, common.ErrPasskeyRequired
		}
		return nil, err
	}
	if len(webUser.credentials) == 0 {
		return nil, common.ErrPasskeyRequired
	}
	instance, err := s.instance(origin)
	if err != nil {
		return nil, err
	}
	assertion, sessionData, err := instance.BeginLogin(webUser)
	if err != nil {
		return nil, err
	}
	ceremonyID, err := s.saveCeremony(ctx, ceremony{Kind: ceremonyReauth, UserID: userID, SessionID: sessionID, Origin: origin, SessionData: *sessionData})
	if err != nil {
		return nil, err
	}
	return &AuthenticationOptions{CeremonyID: ceremonyID, Options: assertion}, nil
}

// FinishReauth verifies an assertion and issues a generic Passkey authorization grant.
func (s *Service) FinishReauth(ctx context.Context, userID string, sessionID string, origin string, ceremonyID string, response json.RawMessage) (*reauth.Result, error) {
	state, err := s.takeCeremony(ctx, ceremonyID, ceremonyReauth, userID, sessionID, origin)
	if err != nil {
		return nil, err
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(response)
	if err != nil {
		return nil, common.ErrWebAuthnCeremonyInvalid
	}
	user, err := db.NewUserRepository(s.database).FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	webUserRecord, err := s.repo.FindUser(ctx, s.cfg.Passkey.RPID, userID)
	if err != nil {
		return nil, common.ErrWebAuthnCeremonyInvalid
	}
	instance, err := s.instance(origin)
	if err != nil {
		return nil, err
	}
	record, err := s.repo.ValidateAndUpdateCredential(ctx, s.cfg.Passkey.RPID, userID, func(credentials []webauthn.Credential) (*webauthn.Credential, error) {
		webUser := makeWebAuthnUser(user, webUserRecord.Handle, credentials)
		return instance.ValidateLogin(webUser, state.SessionData, parsed)
	})
	if err != nil {
		return nil, common.ErrWebAuthnCeremonyInvalid
	}
	if record.CloneWarning {
		return nil, common.ErrPasskeyCloneWarning
	}
	return s.reauth.Issue(ctx, userID, sessionID, record.ID)
}

// Rename updates one Passkey display name.
func (s *Service) Rename(ctx context.Context, userID string, id string, name string) error {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 64 {
		return common.ErrPasskeyNameInvalid
	}
	if err := s.repo.UpdateCredentialName(ctx, s.cfg.Passkey.RPID, userID, id, name); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return common.ErrPasskeyNotFound
		}
		return err
	}
	return nil
}

// Delete removes one Passkey. Deleting the final credential is intentionally allowed.
func (s *Service) Delete(ctx context.Context, userID string, id string) error {
	if err := s.repo.DeleteCredential(ctx, s.cfg.Passkey.RPID, userID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return common.ErrPasskeyNotFound
		}
		return err
	}
	return nil
}

func (s *Service) loadUser(ctx context.Context, user *model.User, create bool) (webAuthnUser, error) {
	var webUser *model.WebAuthnUser
	var err error
	if create {
		webUser, err = s.repo.GetOrCreateUser(ctx, s.cfg.Passkey.RPID, user.ID)
	} else {
		webUser, err = s.repo.FindUser(ctx, s.cfg.Passkey.RPID, user.ID)
	}
	if err != nil {
		return webAuthnUser{}, err
	}
	records, err := s.repo.ListCredentials(ctx, s.cfg.Passkey.RPID, user.ID)
	if err != nil {
		return webAuthnUser{}, err
	}
	credentials := make([]webauthn.Credential, 0, len(records))
	for i := range records {
		credential, err := db.RecordToCredential(&records[i])
		if err != nil {
			return webAuthnUser{}, err
		}
		credentials = append(credentials, credential)
	}
	return makeWebAuthnUser(user, webUser.Handle, credentials), nil
}

func makeWebAuthnUser(user *model.User, handle []byte, credentials []webauthn.Credential) webAuthnUser {
	name := user.ID
	if user.Email != nil && strings.TrimSpace(*user.Email) != "" {
		name = strings.TrimSpace(*user.Email)
	}
	displayName := name
	if user.Username != nil && strings.TrimSpace(*user.Username) != "" {
		displayName = strings.TrimSpace(*user.Username)
	}
	return webAuthnUser{id: handle, name: name, displayName: displayName, credentials: credentials}
}

func defaultCredentialName(credential *webauthn.Credential, deviceLabel string) string {
	if credential.Authenticator.Attachment == protocol.Platform {
		return platformCredentialName(deviceLabel)
	}

	hasHybrid := false
	hasInternal := false
	for _, transport := range credential.Transport {
		switch transport {
		case protocol.USB, protocol.NFC, protocol.BLE, protocol.SmartCard:
			return "安全密钥"
		case protocol.Hybrid:
			hasHybrid = true
		case protocol.Internal:
			hasInternal = true
		}
	}
	if hasHybrid {
		return "跨设备 Passkey"
	}
	if hasInternal {
		return platformCredentialName(deviceLabel)
	}
	if credential.Authenticator.Attachment == protocol.CrossPlatform {
		return "外部 Passkey"
	}
	return "Passkey"
}

func platformCredentialName(deviceLabel string) string {
	if deviceLabel == "" {
		return "设备 Passkey"
	}
	return deviceLabel + " Passkey"
}

func (s *Service) instance(origin string) (*webauthn.WebAuthn, error) {
	return webauthn.New(&webauthn.Config{
		RPID: s.cfg.Passkey.RPID, RPDisplayName: s.cfg.Passkey.RPDisplayName, RPOrigins: []string{origin}, RPAllowCrossOrigin: false,
		AttestationPreference:  protocol.PreferNoAttestation,
		AuthenticatorSelection: protocol.AuthenticatorSelection{ResidentKey: protocol.ResidentKeyRequirementRequired, RequireResidentKey: protocol.ResidentKeyRequired(), UserVerification: protocol.VerificationRequired},
		Timeouts: webauthn.TimeoutsConfig{
			Login:        webauthn.TimeoutConfig{Enforce: true, Timeout: s.ceremonyTTL(), TimeoutUVD: s.ceremonyTTL()},
			Registration: webauthn.TimeoutConfig{Enforce: true, Timeout: s.ceremonyTTL(), TimeoutUVD: s.ceremonyTTL()},
		},
	})
}

func (s *Service) validateOrigin(origin string) error {
	origin = strings.TrimSpace(origin)
	for _, allowed := range s.cfg.Passkey.RPOrigins {
		if origin == strings.TrimSpace(allowed) {
			return nil
		}
	}
	return common.ErrWebAuthnCeremonyInvalid
}

func (s *Service) saveCeremony(ctx context.Context, state ceremony) (string, error) {
	ceremonyID := "wac_" + uuid.NewString()
	state.ExpiresAt = time.Now().Add(s.ceremonyTTL())
	data, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	if err := s.store.Set(ctx, kv.KeyWebAuthnCeremony(ceremonyID), string(data), s.ceremonyTTL()); err != nil {
		return "", err
	}
	return ceremonyID, nil
}

func (s *Service) takeCeremony(ctx context.Context, id string, kind ceremonyKind, userID string, sessionID string, origin string) (*ceremony, error) {
	data, err := s.store.Take(ctx, kv.KeyWebAuthnCeremony(id))
	if err != nil {
		return nil, common.ErrWebAuthnCeremonyInvalid
	}
	var state ceremony
	if err := json.Unmarshal([]byte(data), &state); err != nil {
		return nil, common.ErrWebAuthnCeremonyInvalid
	}
	if state.Kind != kind || state.UserID != userID || state.SessionID != sessionID || state.Origin != origin || !time.Now().Before(state.ExpiresAt) {
		return nil, common.ErrWebAuthnCeremonyInvalid
	}
	return &state, nil
}

func (s *Service) ceremonyTTL() time.Duration {
	if s.cfg != nil && s.cfg.Passkey.CeremonyTTL > 0 {
		return s.cfg.Passkey.CeremonyTTL
	}
	return 5 * time.Minute
}

func recordView(record *model.WebAuthnCredential) *Credential {
	flags := webauthn.NewCredentialFlags(protocol.AuthenticatorFlags(record.Flags))
	return &Credential{ID: record.ID, Name: record.Name, Attachment: record.Attachment, BackupEligible: flags.BackupEligible, BackupState: flags.BackupState, CreatedAt: record.CreatedAt, LastUsedAt: record.LastUsedAt}
}
