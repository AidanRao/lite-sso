package db

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	"sso-server/model"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) FindByID(ctx context.Context, userID string) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).First(&user, "id = ?", userID).Error; err != nil {
		return nil, err
	}
	if err := r.hydratePrimaryEmail(ctx, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindAll(ctx context.Context) ([]model.User, error) {
	var users []model.User
	if err := r.db.WithContext(ctx).Order("created_at DESC").Find(&users).Error; err != nil {
		return nil, err
	}
	for i := range users {
		if err := r.hydratePrimaryEmail(ctx, &users[i]); err != nil {
			return nil, err
		}
	}
	return users, nil
}

func (r *UserRepository) ExistsEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.UserEmail{}).
		Where("email = ? AND verified_at IS NOT NULL", NormalizeEmail(email)).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *UserRepository) ExistsUsername(ctx context.Context, username string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.User{}).Where("username = ?", username).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *UserRepository) ExistsUsernameExceptID(ctx context.Context, username string, userID string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.User{}).Where("username = ? AND id <> ?", username, userID).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *UserRepository) Create(ctx context.Context, user *model.User) error {
	if user.Email != nil {
		primaryEmail := NormalizeEmail(*user.Email)
		user.Email = &primaryEmail
	}
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).Model(&model.User{}).
		Joins("JOIN user_emails ON user_emails.user_id = users.id").
		Where("user_emails.email = ? AND user_emails.verified_at IS NOT NULL", NormalizeEmail(email)).
		First(&user).Error; err != nil {
		return nil, err
	}
	if err := r.hydratePrimaryEmail(ctx, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) Update(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *UserRepository) hydratePrimaryEmail(ctx context.Context, user *model.User) error {
	var record model.UserEmail
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND is_primary = ? AND verified_at IS NOT NULL", user.ID, true).
		First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			user.Email = nil
			return nil
		}
		return err
	}
	value := record.Email
	user.Email = &value
	return nil
}

// NormalizeEmail returns the canonical representation used by identity lookups.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
