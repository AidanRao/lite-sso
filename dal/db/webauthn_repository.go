package db

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"sso-server/model"
)

// WebAuthnRepository persists WebAuthn users and credential records.
type WebAuthnRepository struct {
	db *gorm.DB
}

// NewWebAuthnRepository creates a WebAuthn repository.
func NewWebAuthnRepository(database *gorm.DB) *WebAuthnRepository {
	return &WebAuthnRepository{db: database}
}

// GetOrCreateUser returns the stable WebAuthn user handle for a relying party.
func (r *WebAuthnRepository) GetOrCreateUser(ctx context.Context, rpID string, userID string) (*model.WebAuthnUser, error) {
	var record model.WebAuthnUser
	err := r.db.WithContext(ctx).First(&record, "rp_id = ? AND user_id = ?", rpID, userID).Error
	if err == nil {
		return &record, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	handle := make([]byte, 64)
	if _, err := rand.Read(handle); err != nil {
		return nil, err
	}
	record = model.WebAuthnUser{ID: uuid.NewString(), RPID: rpID, UserID: userID, Handle: handle}
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		if lookupErr := r.db.WithContext(ctx).First(&record, "rp_id = ? AND user_id = ?", rpID, userID).Error; lookupErr == nil {
			return &record, nil
		}
		return nil, err
	}
	return &record, nil
}

// FindUser gets an existing WebAuthn user record.
func (r *WebAuthnRepository) FindUser(ctx context.Context, rpID string, userID string) (*model.WebAuthnUser, error) {
	var record model.WebAuthnUser
	if err := r.db.WithContext(ctx).First(&record, "rp_id = ? AND user_id = ?", rpID, userID).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

// ListCredentials returns all Passkeys for the user.
func (r *WebAuthnRepository) ListCredentials(ctx context.Context, rpID string, userID string) ([]model.WebAuthnCredential, error) {
	var records []model.WebAuthnCredential
	if err := r.db.WithContext(ctx).Where("rp_id = ? AND user_id = ?", rpID, userID).Order("created_at ASC").Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

// HasCredentials reports whether the user has at least one Passkey for the relying party.
func (r *WebAuthnRepository) HasCredentials(ctx context.Context, rpID string, userID string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.WebAuthnCredential{}).Where("rp_id = ? AND user_id = ?", rpID, userID).Limit(1).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// CreateCredential persists a newly verified Passkey.
func (r *WebAuthnRepository) CreateCredential(ctx context.Context, rpID string, userID string, name string, extensionsJSON string, credential *webauthn.Credential) (*model.WebAuthnCredential, error) {
	record, err := credentialToRecord(rpID, userID, name, extensionsJSON, credential)
	if err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

// UpdateCredentialName changes only user-facing credential metadata.
func (r *WebAuthnRepository) UpdateCredentialName(ctx context.Context, rpID string, userID string, id string, name string) error {
	result := r.db.WithContext(ctx).Model(&model.WebAuthnCredential{}).
		Where("id = ? AND rp_id = ? AND user_id = ?", id, rpID, userID).
		Update("name", name)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteCredential deletes one credential owned by the user.
func (r *WebAuthnRepository) DeleteCredential(ctx context.Context, rpID string, userID string, id string) error {
	result := r.db.WithContext(ctx).Where("id = ? AND rp_id = ? AND user_id = ?", id, rpID, userID).Delete(&model.WebAuthnCredential{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ValidateAndUpdateCredential locks credentials, invokes verification, and atomically stores mutable state.
func (r *WebAuthnRepository) ValidateAndUpdateCredential(ctx context.Context, rpID string, userID string, validate func([]webauthn.Credential) (*webauthn.Credential, error)) (*model.WebAuthnCredential, error) {
	var updated *model.WebAuthnCredential
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var records []model.WebAuthnCredential
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("rp_id = ? AND user_id = ?", rpID, userID).Find(&records).Error; err != nil {
			return err
		}
		credentials := make([]webauthn.Credential, 0, len(records))
		for i := range records {
			credential, err := RecordToCredential(&records[i])
			if err != nil {
				return err
			}
			credentials = append(credentials, credential)
		}
		verified, err := validate(credentials)
		if err != nil {
			return err
		}
		for i := range records {
			if string(records[i].CredentialID) != string(verified.ID) {
				continue
			}
			now := time.Now()
			values := map[string]interface{}{
				"flags": uint8(verified.Flags.ProtocolValue()), "sign_count": verified.Authenticator.SignCount,
				"clone_warning": verified.Authenticator.CloneWarning, "attachment": string(verified.Authenticator.Attachment),
				"last_used_at": now, "updated_at": now,
			}
			if err := tx.Model(&model.WebAuthnCredential{}).Where("id = ?", records[i].ID).Updates(values).Error; err != nil {
				return err
			}
			records[i].Flags = uint8(verified.Flags.ProtocolValue())
			records[i].SignCount = verified.Authenticator.SignCount
			records[i].CloneWarning = verified.Authenticator.CloneWarning
			records[i].Attachment = string(verified.Authenticator.Attachment)
			records[i].LastUsedAt = &now
			updated = &records[i]
			return nil
		}
		return gorm.ErrRecordNotFound
	})
	return updated, err
}

// RecordToCredential reconstructs the complete go-webauthn credential record.
func RecordToCredential(record *model.WebAuthnCredential) (webauthn.Credential, error) {
	var transports []protocol.AuthenticatorTransport
	if err := json.Unmarshal([]byte(record.TransportsJSON), &transports); err != nil {
		return webauthn.Credential{}, err
	}
	return webauthn.Credential{
		ID: record.CredentialID, PublicKey: record.PublicKey, AttestationType: record.AttestationType,
		AttestationFormat: record.AttestationFormat, Transport: transports,
		Flags:         webauthn.NewCredentialFlags(protocol.AuthenticatorFlags(record.Flags)),
		Authenticator: webauthn.Authenticator{AAGUID: record.AAGUID, SignCount: record.SignCount, CloneWarning: record.CloneWarning, Attachment: protocol.AuthenticatorAttachment(record.Attachment)},
		Attestation:   webauthn.CredentialAttestation{ClientDataJSON: record.AttestationClientData, ClientDataHash: record.AttestationClientHash, AuthenticatorData: record.AttestationAuthData, PublicKeyAlgorithm: record.PublicKeyAlgorithm, Object: record.AttestationObject},
	}, nil
}

func credentialToRecord(rpID string, userID string, name string, extensionsJSON string, credential *webauthn.Credential) (*model.WebAuthnCredential, error) {
	transports, err := json.Marshal(credential.Transport)
	if err != nil {
		return nil, err
	}
	return &model.WebAuthnCredential{
		ID: uuid.NewString(), RPID: rpID, UserID: userID, CredentialID: credential.ID, PublicKey: credential.PublicKey,
		AAGUID: credential.Authenticator.AAGUID, AttestationType: credential.AttestationType, AttestationFormat: credential.AttestationFormat,
		TransportsJSON: string(transports), Attachment: string(credential.Authenticator.Attachment), Flags: uint8(credential.Flags.ProtocolValue()),
		SignCount: credential.Authenticator.SignCount, CloneWarning: credential.Authenticator.CloneWarning,
		AttestationClientData: credential.Attestation.ClientDataJSON, AttestationClientHash: credential.Attestation.ClientDataHash,
		AttestationAuthData: credential.Attestation.AuthenticatorData, PublicKeyAlgorithm: credential.Attestation.PublicKeyAlgorithm,
		AttestationObject: credential.Attestation.Object, ExtensionsJSON: extensionsJSON, Name: name,
	}, nil
}
