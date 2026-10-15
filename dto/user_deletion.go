package dto

import (
	"time"
)

// AccountDeletionResponse describes the server-controlled cooling period.
type AccountDeletionResponse struct {
	Status      string     `json:"status"`
	RequestedAt *time.Time `json:"deletion_requested_at,omitempty"`
	ScheduledAt *time.Time `json:"deletion_scheduled_at,omitempty"`
	CanCancel   bool       `json:"can_cancel"`
}
