package db

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"sso-server/model"
)

// UserEmailView contains one email and its currently connected provider labels.
type UserEmailView struct {
	Email   model.UserEmail
	Sources []string
}

// UserEmailRepository persists normalized user email records and provenance.
type UserEmailRepository struct {
	db *gorm.DB
}

// NewUserEmailRepository creates a user email repository.
func NewUserEmailRepository(database *gorm.DB) *UserEmailRepository {
	return &UserEmailRepository{db: database}
}

// ListByUserID returns emails in primary-first creation order with source labels.
func (r *UserEmailRepository) ListByUserID(ctx context.Context, userID string) ([]UserEmailView, error) {
	var records []model.UserEmail
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("is_primary DESC, created_at ASC").
		Find(&records).Error; err != nil {
		return nil, err
	}

	views := make([]UserEmailView, 0, len(records))
	for i := range records {
		var sources []string
		if err := r.db.WithContext(ctx).Model(&model.UserThirdParty{}).
			Select("user_third_party.provider").
			Joins("JOIN user_email_sources ON user_email_sources.user_third_party_id = user_third_party.id").
			Where("user_email_sources.user_email_id = ?", records[i].ID).
			Order("user_third_party.provider ASC").
			Pluck("user_third_party.provider", &sources).Error; err != nil {
			return nil, err
		}
		views = append(views, UserEmailView{Email: records[i], Sources: sources})
	}
	return views, nil
}

// FindByIDForUser returns an email owned by one user.
func (r *UserEmailRepository) FindByIDForUser(ctx context.Context, userID string, emailID string) (*model.UserEmail, error) {
	var record model.UserEmail
	if err := r.db.WithContext(ctx).First(&record, "id = ? AND user_id = ?", emailID, userID).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

// FindVerifiedByEmail returns a globally verified email record.
func (r *UserEmailRepository) FindVerifiedByEmail(ctx context.Context, email string) (*model.UserEmail, error) {
	var record model.UserEmail
	if err := r.db.WithContext(ctx).
		First(&record, "email = ? AND verified_at IS NOT NULL", NormalizeEmail(email)).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

// FindByTokenHash returns the record carrying an active verification token.
func (r *UserEmailRepository) FindByTokenHash(ctx context.Context, tokenHash string) (*model.UserEmail, error) {
	var record model.UserEmail
	if err := r.db.WithContext(ctx).First(&record, "verification_token_hash = ?", tokenHash).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

// CountByUserID returns every address, including unverified addresses.
func (r *UserEmailRepository) CountByUserID(ctx context.Context, userID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.UserEmail{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CountVerifiedByUserID returns the number of verified addresses owned by one user.
func (r *UserEmailRepository) CountVerifiedByUserID(ctx context.Context, userID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.UserEmail{}).
		Where("user_id = ? AND verified_at IS NOT NULL", userID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// Create inserts an email record.
func (r *UserEmailRepository) Create(ctx context.Context, record *model.UserEmail) error {
	return r.db.WithContext(ctx).Create(record).Error
}

// AddSource attaches a provider binding without duplicating provenance.
func (r *UserEmailRepository) AddSource(ctx context.Context, emailID string, bindingID uint) error {
	record := model.UserEmailSource{UserEmailID: emailID, UserThirdPartyID: bindingID}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&record).Error
}

// Delete removes an email owned by the supplied user.
func (r *UserEmailRepository) Delete(ctx context.Context, userID string, emailID string) (int64, error) {
	result := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", emailID, userID).Delete(&model.UserEmail{})
	return result.RowsAffected, result.Error
}
