package model

import (
	"time"
)

const (
	UserStatusActive              = "active"
	UserStatusDisabled            = "disabled"
	UserStatusPendingDeletion     = "pending_deletion"
	SessionPurposeNormal          = "normal"
	SessionPurposeAccountDeletion = "account_deletion"
)

// UserDeletion records a cooling period without retaining identity fields.
type UserDeletion struct {
	ID          string    `gorm:"type:varchar(36);primaryKey"`
	UserID      string    `gorm:"type:varchar(36);not null;index"`
	RequestedAt time.Time `gorm:"not null"`
	ScheduledAt time.Time `gorm:"not null;index"`
	Status      string    `gorm:"type:varchar(16);not null"`
	FinishedAt  *time.Time
}

// UserCleanupTask persists external cleanup independently of the user's lifetime.
type UserCleanupTask struct {
	ID            string    `gorm:"type:varchar(36);primaryKey"`
	UserID        string    `gorm:"type:varchar(36);not null"`
	Kind          string    `gorm:"type:varchar(32);not null"`
	ObjectKey     string    `gorm:"type:varchar(512);not null;default:''"`
	Cutoff        time.Time `gorm:"not null"`
	Attempts      int       `gorm:"not null;default:0"`
	NextAttemptAt time.Time `gorm:"not null;index"`
}

// Due reports whether this pending request has reached its irreversible deadline.
func (d *UserDeletion) Due(now time.Time) bool {
	return d.Status == "pending" && !now.Before(d.ScheduledAt)
}
