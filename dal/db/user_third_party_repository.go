package db

import (
	"context"

	"gorm.io/gorm"

	"sso-server/model"
)

type UserThirdPartyRepository struct {
	db *gorm.DB
}

func NewUserThirdPartyRepository(db *gorm.DB) *UserThirdPartyRepository {
	return &UserThirdPartyRepository{db: db}
}

func (r *UserThirdPartyRepository) FindByProviderUID(ctx context.Context, provider, providerUID string) (*model.UserThirdParty, error) {
	var binding model.UserThirdParty
	if err := r.db.WithContext(ctx).First(&binding, "provider = ? AND provider_uid = ?", provider, providerUID).Error; err != nil {
		return nil, err
	}
	return &binding, nil
}

func (r *UserThirdPartyRepository) FindByUserID(ctx context.Context, userID, provider string) (*model.UserThirdParty, error) {
	var binding model.UserThirdParty
	if err := r.db.WithContext(ctx).First(&binding, "user_id = ? AND provider = ?", userID, provider).Error; err != nil {
		return nil, err
	}
	return &binding, nil
}

func (r *UserThirdPartyRepository) Create(ctx context.Context, binding *model.UserThirdParty) error {
	return r.db.WithContext(ctx).Create(binding).Error
}

func (r *UserThirdPartyRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&model.UserThirdParty{}, id).Error
}
