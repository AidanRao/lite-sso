package model

import (
	"time"
)

// AuditLog is an immutable snapshot of one audited HTTP operation.
// There are deliberately no foreign keys or credentials in this record.
type AuditLog struct {
	ID          string    `json:"id" gorm:"type:uuid;primaryKey"`
	OccurredAt  time.Time `json:"occurred_at" gorm:"not null"`
	CreatedAt   time.Time `json:"created_at" gorm:"not null"`
	UserID      *string   `json:"user_id" gorm:"type:varchar(36)"`
	ActorType   string    `json:"actor_type" gorm:"type:varchar(16);not null"`
	Action      string    `json:"action" gorm:"type:varchar(64);not null"`
	TargetType  string    `json:"target_type" gorm:"type:varchar(32);not null"`
	TargetID    string    `json:"target_id" gorm:"type:varchar(128);not null"`
	ClientID    *string   `json:"client_id" gorm:"type:varchar(64)"`
	Method      string    `json:"method" gorm:"type:varchar(16);not null"`
	Route       string    `json:"route" gorm:"type:varchar(160);not null"`
	HTTPStatus  int       `json:"http_status" gorm:"not null"`
	Outcome     string    `json:"outcome" gorm:"type:varchar(16);not null"`
	ReasonCode  string    `json:"reason_code" gorm:"type:varchar(64);not null"`
	DurationMS  int64     `json:"duration_ms" gorm:"not null"`
	IP          string    `json:"ip" gorm:"type:varchar(64);not null"`
	DeviceID    string    `json:"device_id" gorm:"type:varchar(64);not null"`
	SessionHash string    `json:"session_hash" gorm:"type:char(64);not null"`
	DeviceLabel string    `json:"device_label" gorm:"type:varchar(32);not null"`
	Details     string    `json:"details" gorm:"type:jsonb;not null"`
}

// AuditDetails is the complete allowlist of operation-specific audit metadata.
type AuditDetails struct {
	ChangedFields  []string `json:"changed_fields,omitempty"`
	AuthMethod     string   `json:"auth_method,omitempty"`
	Provider       string   `json:"provider,omitempty"`
	EmailMasked    string   `json:"email_masked,omitempty"`
	CompletedSteps []string `json:"completed_steps,omitempty"`
}

// TableName returns the append-only audit table (except retention deletion).
func (AuditLog) TableName() string { return "audit_logs" }
