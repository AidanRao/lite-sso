package dto

import (
	"time"

	"sso-server/model"
)

// AuditLogQuery contains only filters; the authenticated user is supplied separately.
type AuditLogQuery struct {
	Q       string `form:"q"`
	Actions string `form:"actions"`
	Outcome string `form:"outcome"`
	From    string `form:"from"`
	To      string `form:"to"`
	Cursor  string `form:"cursor"`
	Limit   *int   `form:"limit"`
}

// AuditLogResponse exposes facts only. Presentation belongs to the frontend.
type AuditLogResponse struct {
	ID          string                    `json:"id"`
	OccurredAt  time.Time                 `json:"occurred_at"`
	Action      string                    `json:"action"`
	Outcome     string                    `json:"outcome"`
	ReasonCode  string                    `json:"reason_code"`
	TargetType  string                    `json:"target_type"`
	TargetID    string                    `json:"target_id"`
	ClientID    *string                   `json:"client_id"`
	Application *AuditApplicationResponse `json:"application"`
	Method      string                    `json:"method"`
	Route       string                    `json:"route"`
	HTTPStatus  int                       `json:"http_status"`
	DurationMS  int64                     `json:"duration_ms"`
	IP          string                    `json:"ip"`
	DeviceLabel string                    `json:"device_label"`
	Details     model.AuditDetails        `json:"details"`
}

// AuditApplicationResponse contains current public application metadata, not a historical snapshot.
type AuditApplicationResponse struct {
	ClientID    string  `json:"client_id"`
	Name        string  `json:"name"`
	LogoURL     *string `json:"logo_url"`
	HomepageURL string  `json:"homepage_url"`
}

// AuditLogPage uses keyset pagination and intentionally omits a total count.
type AuditLogPage struct {
	Items      []AuditLogResponse `json:"items"`
	NextCursor string             `json:"next_cursor"`
}
