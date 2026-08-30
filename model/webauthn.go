package model

import "time"

// WebAuthnUser stores the stable opaque user handle scoped to one relying party.
type WebAuthnUser struct {
	ID        string    `gorm:"type:varchar(36);primaryKey"`
	RPID      string    `gorm:"type:varchar(255);not null;uniqueIndex:idx_webauthn_user_rp_user;uniqueIndex:idx_webauthn_user_rp_handle"`
	UserID    string    `gorm:"type:varchar(36);not null;uniqueIndex:idx_webauthn_user_rp_user"`
	Handle    []byte    `gorm:"type:bytea;not null;uniqueIndex:idx_webauthn_user_rp_handle"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (WebAuthnUser) TableName() string {
	return "webauthn_users"
}

// WebAuthnCredential stores one Passkey and the mutable authenticator state.
type WebAuthnCredential struct {
	ID                    string     `gorm:"type:varchar(36);primaryKey"`
	RPID                  string     `gorm:"type:varchar(255);not null;uniqueIndex:idx_webauthn_credential_rp_id;index:idx_webauthn_credential_rp_user"`
	UserID                string     `gorm:"type:varchar(36);not null;index:idx_webauthn_credential_rp_user"`
	CredentialID          []byte     `gorm:"type:bytea;not null;uniqueIndex:idx_webauthn_credential_rp_id"`
	PublicKey             []byte     `gorm:"type:bytea;not null"`
	AAGUID                []byte     `gorm:"column:aaguid;type:bytea"`
	AttestationType       string     `gorm:"type:varchar(64);not null"`
	AttestationFormat     string     `gorm:"type:varchar(64);not null"`
	TransportsJSON        string     `gorm:"type:text;not null"`
	Attachment            string     `gorm:"type:varchar(32);not null"`
	Flags                 uint8      `gorm:"not null"`
	SignCount             uint32     `gorm:"not null"`
	CloneWarning          bool       `gorm:"not null"`
	AttestationClientData []byte     `gorm:"type:bytea"`
	AttestationClientHash []byte     `gorm:"type:bytea"`
	AttestationAuthData   []byte     `gorm:"type:bytea"`
	PublicKeyAlgorithm    int64      `gorm:"not null"`
	AttestationObject     []byte     `gorm:"type:bytea"`
	ExtensionsJSON        string     `gorm:"type:text;not null"`
	Name                  string     `gorm:"type:varchar(64);not null"`
	CreatedAt             time.Time  `gorm:"not null"`
	UpdatedAt             time.Time  `gorm:"not null"`
	LastUsedAt            *time.Time `gorm:"index"`
}

func (WebAuthnCredential) TableName() string {
	return "webauthn_credentials"
}
