package model

import "time"

// UserEmail stores one normalized email address owned by a user.
type UserEmail struct {
	ID                    string     `gorm:"type:varchar(36);primaryKey"`
	UserID                string     `gorm:"type:varchar(36);not null;index;uniqueIndex:idx_user_email_owner,priority:1;uniqueIndex:idx_user_email_primary,where:is_primary = true"`
	Email                 string     `gorm:"type:varchar(100);not null;uniqueIndex:idx_user_email_owner,priority:2;uniqueIndex:idx_user_email_verified,where:verified_at IS NOT NULL"`
	VerifiedAt            *time.Time `gorm:"index"`
	IsPrimary             bool       `gorm:"not null;default:false"`
	VerificationTokenHash *string    `gorm:"type:char(64);uniqueIndex:idx_user_email_verification_token,where:verification_token_hash IS NOT NULL"`
	VerificationExpiresAt *time.Time
	VerificationSentAt    *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

func (UserEmail) TableName() string {
	return "user_emails"
}

// UserEmailSource links a trusted email address to the provider binding that supplied it.
type UserEmailSource struct {
	ID               uint           `gorm:"primaryKey;autoIncrement"`
	UserEmailID      string         `gorm:"type:varchar(36);not null;index;uniqueIndex:idx_user_email_source,priority:1"`
	UserThirdPartyID uint           `gorm:"not null;index;uniqueIndex:idx_user_email_source,priority:2"`
	UserEmail        UserEmail      `gorm:"foreignKey:UserEmailID;references:ID;constraint:OnDelete:CASCADE"`
	UserThirdParty   UserThirdParty `gorm:"foreignKey:UserThirdPartyID;references:ID;constraint:OnDelete:CASCADE"`
}

func (UserEmailSource) TableName() string {
	return "user_email_sources"
}
