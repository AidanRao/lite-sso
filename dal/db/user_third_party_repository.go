package db

import (
	"context"
	"time"

	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/model"
)

type UserThirdPartyRepository struct {
	db *gorm.DB
}

func NewUserThirdPartyRepository(db *gorm.DB) *UserThirdPartyRepository {
	return &UserThirdPartyRepository{db: db}
}

func (r *UserThirdPartyRepository) FindByProviderUID(ctx context.Context, provider, providerUID string) (*model.UserThirdParty, error) {
	if err := PurgeExpiredIdentities(ctx, r.db, "id IN (?)", r.db.Model(&model.UserThirdParty{}).Select("user_id").Where("provider = ? AND provider_uid = ?", provider, providerUID)); err != nil {
		return nil, err
	}
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

func (r *UserThirdPartyRepository) ListByUserID(ctx context.Context, userID string) ([]model.UserThirdParty, error) {
	var bindings []model.UserThirdParty
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&bindings).Error; err != nil {
		return nil, err
	}
	return bindings, nil
}

func (r *UserThirdPartyRepository) Create(ctx context.Context, binding *model.UserThirdParty) error {
	return r.db.WithContext(ctx).Create(binding).Error
}

// UpdateProfile refreshes provider account information without changing the binding identity.
func (r *UserThirdPartyRepository) UpdateProfile(ctx context.Context, id uint, email, username string) error {
	return r.db.WithContext(ctx).Model(&model.UserThirdParty{}).Where("id = ?", id).
		Updates(map[string]any{"email": email, "username": username}).Error
}

// UpdateLastUsed records a successful login for the exact provider identity.
func (r *UserThirdPartyRepository) UpdateLastUsed(ctx context.Context, userID, provider, providerUID string, usedAt time.Time) error {
	result := r.db.WithContext(ctx).Model(&model.UserThirdParty{}).
		Where("user_id = ? AND provider = ? AND provider_uid = ?", userID, provider, providerUID).
		Update("last_used_at", usedAt.UTC())
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return common.ErrThirdPartyNotBound
	}
	return nil
}

func (r *UserThirdPartyRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_third_party_id = ?", id).Delete(&model.UserEmailSource{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.UserThirdParty{}, id).Error
	})
}
