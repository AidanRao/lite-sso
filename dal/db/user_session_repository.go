package db

import (
	"context"
	"time"

	"gorm.io/gorm"

	"sso-server/model"
)

// UserSessionRepository persists authenticated user sessions.
type UserSessionRepository struct {
	db *gorm.DB
}

func NewUserSessionRepository(database *gorm.DB) *UserSessionRepository {
	return &UserSessionRepository{db: database}
}

func (r *UserSessionRepository) Create(ctx context.Context, session *model.UserSession) error {
	return r.db.WithContext(ctx).Create(session).Error
}

func (r *UserSessionRepository) FindActive(ctx context.Context, sessionID string, now time.Time) (*model.UserSession, error) {
	var session model.UserSession
	err := r.db.WithContext(ctx).
		Where("id = ? AND revoked_at IS NULL AND expires_at > ?", sessionID, now).
		First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *UserSessionRepository) FindByID(ctx context.Context, sessionID string) (*model.UserSession, error) {
	var session model.UserSession
	if err := r.db.WithContext(ctx).Where("id = ?", sessionID).First(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *UserSessionRepository) FindByRefreshHash(ctx context.Context, tokenHash string) (*model.UserSession, error) {
	var session model.UserSession
	if err := r.db.WithContext(ctx).Where("refresh_token_hash = ?", tokenHash).First(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *UserSessionRepository) Rotate(ctx context.Context, sessionID string, oldHash string, newHash string, now time.Time) error {
	result := r.db.WithContext(ctx).Model(&model.UserSession{}).
		Where("id = ? AND refresh_token_hash = ? AND revoked_at IS NULL AND expires_at > ?", sessionID, oldHash, now).
		Updates(map[string]any{"refresh_token_hash": newHash, "last_seen_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *UserSessionRepository) Revoke(ctx context.Context, sessionID string, reason string, now time.Time) error {
	result := r.db.WithContext(ctx).Model(&model.UserSession{}).
		Where("id = ? AND revoked_at IS NULL", sessionID).
		Updates(map[string]any{"revoked_at": now, "revoke_reason": reason})
	return result.Error
}

func (r *UserSessionRepository) RevokeOthers(ctx context.Context, userID string, keepSessionID string, reason string, now time.Time) error {
	return r.db.WithContext(ctx).Model(&model.UserSession{}).
		Where("user_id = ? AND id <> ? AND revoked_at IS NULL", userID, keepSessionID).
		Updates(map[string]any{"revoked_at": now, "revoke_reason": reason}).Error
}

func (r *UserSessionRepository) RevokeAll(ctx context.Context, userID string, reason string, now time.Time) error {
	return r.db.WithContext(ctx).Model(&model.UserSession{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Updates(map[string]any{"revoked_at": now, "revoke_reason": reason}).Error
}
