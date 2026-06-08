package db

import (
	"context"

	"gorm.io/gorm"

	"sso-server/model"
)

type OAuthClientRepository struct {
	db *gorm.DB
}

func NewOAuthClientRepository(db *gorm.DB) *OAuthClientRepository {
	return &OAuthClientRepository{db: db}
}

func (r *OAuthClientRepository) FindByClientID(ctx context.Context, clientID string) (*model.OAuthClient, error) {
	var client model.OAuthClient
	if err := r.db.WithContext(ctx).First(&client, "client_id = ?", clientID).Error; err != nil {
		return nil, err
	}
	return &client, nil
}

func (r *OAuthClientRepository) FindByID(ctx context.Context, id uint) (*model.OAuthClient, error) {
	var client model.OAuthClient
	if err := r.db.WithContext(ctx).First(&client, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &client, nil
}

func (r *OAuthClientRepository) FindAll(ctx context.Context) ([]model.OAuthClient, error) {
	var clients []model.OAuthClient
	if err := r.db.WithContext(ctx).Order("id ASC").Find(&clients).Error; err != nil {
		return nil, err
	}
	return clients, nil
}

func (r *OAuthClientRepository) FindByUserID(ctx context.Context, userID string) ([]model.OAuthClient, error) {
	var clients []model.OAuthClient
	if userID == "" {
		return clients, nil
	}

	err := r.db.WithContext(ctx).
		Table("oauth_clients AS oc").
		Select("oc.*").
		Joins("JOIN user_oauth_clients AS uoc ON uoc.client_id = oc.client_id").
		Where("uoc.user_id = ?", userID).
		Order("uoc.last_login_at DESC").
		Scan(&clients).Error
	if err != nil {
		return nil, err
	}
	return clients, nil
}

func (r *OAuthClientRepository) ExistsClientID(ctx context.Context, clientID string, excludeID uint) (bool, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(&model.OAuthClient{}).Where("client_id = ?", clientID)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *OAuthClientRepository) Create(ctx context.Context, client *model.OAuthClient) error {
	return r.db.WithContext(ctx).Create(client).Error
}

func (r *OAuthClientRepository) Update(ctx context.Context, client *model.OAuthClient) error {
	return r.db.WithContext(ctx).Save(client).Error
}
