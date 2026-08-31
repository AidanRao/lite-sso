package db

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	"sso-server/model"
)

// AuditRepository persists audit snapshots without logging SQL parameters.
type AuditRepository struct{ database *gorm.DB }

// NewAuditRepository shares the application's pool but disables SQL logging for audits.
func NewAuditRepository(database *gorm.DB) *AuditRepository {
	return &AuditRepository{database: database.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})}
}

// InsertBatch ignores only duplicate event IDs, making uncertain retries safe.
func (r *AuditRepository) InsertBatch(ctx context.Context, records []model.AuditLog) error {
	if len(records) == 0 {
		return nil
	}
	return r.database.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "id"}}, DoNothing: true}).Create(&records).Error
}

// DeleteExpired removes a bounded batch while skipping rows claimed by other instances.
func (r *AuditRepository) DeleteExpired(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	result := r.database.WithContext(ctx).Exec(`WITH expired AS (
		SELECT id FROM audit_logs WHERE occurred_at < ? ORDER BY occurred_at, id
		LIMIT ? FOR UPDATE SKIP LOCKED
	) DELETE FROM audit_logs USING expired WHERE audit_logs.id = expired.id`, cutoff, limit)
	return result.RowsAffected, result.Error
}
