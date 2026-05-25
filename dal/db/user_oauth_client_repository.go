package db

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"sso-server/model"
)

type UserOAuthClientRepository struct {
	db *gorm.DB
}

type UserOAuthClientView struct {
	ClientID    string
	Name        string
	LastLoginAt time.Time
}

func NewUserOAuthClientRepository(db *gorm.DB) *UserOAuthClientRepository {
	return &UserOAuthClientRepository{db: db}
}

func (r *UserOAuthClientRepository) RecordLogin(ctx context.Context, userID string, clientID string, at time.Time) error {
	if userID == "" || clientID == "" {
		return nil
	}

	record := &model.UserOAuthClient{
		UserID:      userID,
		ClientID:    clientID,
		LastLoginAt: at,
	}

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"},
			{Name: "client_id"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"last_login_at": at,
			"updated_at":    at,
		}),
	}).Create(record).Error
}

func (r *UserOAuthClientRepository) ListByUserID(ctx context.Context, userID string) ([]UserOAuthClientView, error) {
	var apps []UserOAuthClientView
	err := r.db.WithContext(ctx).
		Table("user_oauth_clients AS uoc").
		Select("uoc.client_id, oc.name, uoc.last_login_at").
		Joins("JOIN oauth_clients AS oc ON oc.client_id = uoc.client_id").
		Where("uoc.user_id = ?", userID).
		Order("uoc.last_login_at DESC").
		Scan(&apps).Error
	if err != nil {
		return nil, err
	}
	return apps, nil
}
