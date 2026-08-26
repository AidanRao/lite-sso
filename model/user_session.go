package model

import "time"

// UserSession stores the server-side state for one authenticated device.
type UserSession struct {
	ID               string     `gorm:"type:varchar(64);primaryKey"`
	UserID           string     `gorm:"type:varchar(36);not null;index"`
	DeviceID         string     `gorm:"type:varchar(64);not null;index"`
	AuthMethod       string     `gorm:"type:varchar(32);not null"`
	RefreshTokenHash string     `gorm:"type:char(64);not null"`
	IP               string     `gorm:"type:varchar(64);not null"`
	UserAgent        string     `gorm:"type:varchar(512);not null"`
	CreatedAt        time.Time  `gorm:"not null"`
	LastSeenAt       time.Time  `gorm:"not null"`
	ExpiresAt        time.Time  `gorm:"not null;index"`
	RevokedAt        *time.Time `gorm:"index"`
	RevokeReason     *string    `gorm:"type:varchar(64)"`
}

func (UserSession) TableName() string {
	return "user_session"
}
