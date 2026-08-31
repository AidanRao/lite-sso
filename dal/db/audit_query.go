package db

import (
	"context"
	"errors"
	"strings"
	"time"

	"sso-server/model"
)

// AuditLogQuery is a validated, user-scoped query with an exclusive upper bound.
type AuditLogQuery struct {
	UserID     string
	From       time.Time
	To         time.Time
	Search     string
	Actions    []string
	Outcome    string
	CursorTime time.Time
	CursorID   string
	Limit      int
}

// ListForUser always applies ownership before search and cursor predicates.
func (r *AuditRepository) ListForUser(ctx context.Context, filter AuditLogQuery) ([]model.AuditLog, error) {
	if filter.UserID == "" || filter.Limit < 1 || filter.Limit > 101 {
		return nil, errors.New("invalid audit query scope")
	}
	query := r.database.WithContext(ctx).Model(&model.AuditLog{}).
		Select("id", "occurred_at", "action", "outcome", "reason_code", "target_type", "target_id", "client_id", "method", "route", "http_status", "duration_ms", "ip", "device_label", "details").
		Where("user_id = ? AND occurred_at >= ? AND occurred_at < ?", filter.UserID, filter.From, filter.To)
	if filter.Outcome != "" {
		query = query.Where("outcome = ?", filter.Outcome)
	}
	var predicates []string
	var arguments []any
	if filter.Search != "" {
		pattern := "%" + strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(strings.ToLower(filter.Search)) + "%"
		for _, column := range []string{"action", "COALESCE(details->>'provider', '')", "ip", "target_id", "COALESCE(client_id, '')"} {
			predicates = append(predicates, "LOWER("+column+") LIKE ? ESCAPE '!' ")
			arguments = append(arguments, pattern)
		}
	}
	if len(filter.Actions) > 0 {
		predicates = append(predicates, "action IN ?")
		arguments = append(arguments, filter.Actions)
	}
	if len(predicates) > 0 {
		query = query.Where("("+strings.Join(predicates, " OR ")+")", arguments...)
	}
	if filter.CursorID != "" {
		query = query.Where("(occurred_at < ? OR (occurred_at = ? AND id < ?))", filter.CursorTime, filter.CursorTime, filter.CursorID)
	}
	var records []model.AuditLog
	err := query.Order("occurred_at DESC, id DESC").Limit(filter.Limit).Find(&records).Error
	return records, err
}
