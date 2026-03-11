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
