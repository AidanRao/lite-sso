package db

import (
	"context"
	"errors"
	"slices"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"sso-server/model"
)

// ErrFeatureUserNotFound indicates an invalid release audience member.
var ErrFeatureUserNotFound = errors.New("feature user not found")

// FeatureRepository persists complete release policies.
type FeatureRepository struct{ database *gorm.DB }

// NewFeatureRepository creates a release policy repository.
func NewFeatureRepository(database *gorm.DB) *FeatureRepository {
	return &FeatureRepository{database: database}
}

// Find loads policies and memberships in one SQL statement for a consistent snapshot.
func (r *FeatureRepository) Find(ctx context.Context) ([]model.FeatureConfig, error) {
	type row struct {
		Key        string
		Audience   string
		Percentage int
		Stage      string
		UserID     *string
	}
	var rows []row
	err := r.database.WithContext(ctx).Table("feature_configs AS f").Select("f.key, f.audience, f.percentage, f.stage, u.user_id").Joins("LEFT JOIN feature_users AS u ON u.feature_key = f.key").Order("f.key, u.user_id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := []model.FeatureConfig{}
	for _, item := range rows {
		if len(result) == 0 || result[len(result)-1].Key != item.Key {
			result = append(result, model.FeatureConfig{Key: item.Key, Audience: item.Audience, Percentage: item.Percentage, Stage: item.Stage})
		}
		if item.UserID != nil {
			i := len(result) - 1
			result[i].Users = append(result[i].Users, model.FeatureUser{FeatureKey: item.Key, UserID: *item.UserID})
		}
	}
	return result, nil
}

// Save atomically replaces a policy and reports fields that actually changed.
func (r *FeatureRepository) Save(ctx context.Context, config model.FeatureConfig) ([]string, error) {
	changed := []string{}
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		initial := model.FeatureConfig{Key: config.Key, Audience: "off", Stage: "beta"}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Omit("Users").Create(&initial).Error; err != nil {
			return err
		}
		var previous model.FeatureConfig
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&previous, "key = ?", config.Key).Error; err != nil {
			return err
		}
		if err := tx.Where("feature_key = ?", config.Key).Order("user_id").Find(&previous.Users).Error; err != nil {
			return err
		}
		ids := make([]string, 0, len(config.Users))
		for _, user := range config.Users {
			ids = append(ids, user.UserID)
		}
		if len(ids) > 0 {
			var count int64
			if err := tx.Model(&model.User{}).Where("id IN ?", ids).Count(&count).Error; err != nil {
				return err
			}
			if count != int64(len(ids)) {
				return ErrFeatureUserNotFound
			}
		}
		if previous.Audience != config.Audience {
			changed = append(changed, "audience")
		}
		if previous.Percentage != config.Percentage {
			changed = append(changed, "percentage")
		}
		if previous.Stage != config.Stage {
			changed = append(changed, "stage")
		}
		oldIDs := make([]string, 0, len(previous.Users))
		for _, user := range previous.Users {
			oldIDs = append(oldIDs, user.UserID)
		}
		if !slices.Equal(oldIDs, ids) {
			changed = append(changed, "user_ids")
		}
		if err := tx.Model(&model.FeatureConfig{}).Where("key = ?", config.Key).Updates(map[string]any{"audience": config.Audience, "percentage": config.Percentage, "stage": config.Stage}).Error; err != nil {
			return err
		}
		if err := tx.Where("feature_key = ?", config.Key).Delete(&model.FeatureUser{}).Error; err != nil {
			return err
		}
		if len(config.Users) > 0 {
			return tx.Create(&config.Users).Error
		}
		return nil
	})
	return changed, err
}
