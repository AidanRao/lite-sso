package db

import (
	"context"
	"time"

	"github.com/google/uuid"
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
	HomepageURL string
	LogoURL     *string
	LastLoginAt time.Time
}

// OAuthClientUserRecord contains display fields for one association, including missing accounts.
type OAuthClientUserRecord struct {
	UserID      string
	Username    *string
	Email       *string
	AvatarURL   *string
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
		UserID:          userID,
		ClientID:        clientID,
		LastLoginAt:     at,
		AuthorizationID: uuid.NewString(),
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := NewOAuthClientRepository(tx).LockByClientID(ctx, clientID); err != nil {
			return err
		}
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "user_id"},
				{Name: "client_id"},
			},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"last_login_at": at,
				"updated_at":    at,
			}),
		}).Create(record).Error
	})
}

func (r *UserOAuthClientRepository) ListByUserID(ctx context.Context, userID string) ([]UserOAuthClientView, error) {
	var apps []UserOAuthClientView
	err := r.db.WithContext(ctx).
		Table("user_oauth_clients AS uoc").
		Select("uoc.client_id, oc.name, oc.homepage_url, oc.logo_url, uoc.last_login_at").
		Joins("JOIN oauth_clients AS oc ON oc.client_id = uoc.client_id").
		Where("uoc.user_id = ?", userID).
		Order("uoc.last_login_at DESC").
		Scan(&apps).Error
	if err != nil {
		return nil, err
	}
	return apps, nil
}

// FindAssociation returns the current authorization identity for a user and client.
func (r *UserOAuthClientRepository) FindAssociation(ctx context.Context, userID, clientID string) (*model.UserOAuthClient, error) {
	var association model.UserOAuthClient
	err := r.db.WithContext(ctx).First(&association, "user_id = ? AND client_id = ?", userID, clientID).Error
	return &association, err
}

// CountByClientID counts associations even when a referenced user no longer exists.
func (r *UserOAuthClientRepository) CountByClientID(ctx context.Context, clientID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.UserOAuthClient{}).Where("client_id = ?", clientID).Count(&count).Error
	return count, err
}

// PageByClientID returns user display fields without fetching every system user.
func (r *UserOAuthClientRepository) PageByClientID(ctx context.Context, clientID string, page, pageSize int) ([]OAuthClientUserRecord, error) {
	var rows []OAuthClientUserRecord
	err := r.db.WithContext(ctx).Table("user_oauth_clients AS uoc").
		Select("uoc.user_id, users.username, users.avatar_url, uoc.last_login_at, (SELECT email FROM user_emails WHERE user_id = uoc.user_id AND is_primary = true AND verified_at IS NOT NULL LIMIT 1) AS email").
		Joins("LEFT JOIN users ON users.id = uoc.user_id").Where("uoc.client_id = ?", clientID).
		Order("uoc.last_login_at DESC, uoc.user_id ASC").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&rows).Error
	return rows, err
}
