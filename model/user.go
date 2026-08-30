package model

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID              string  `gorm:"type:varchar(36);primaryKey"`
	Username        *string `gorm:"type:varchar(50);uniqueIndex"`
	Email           *string `gorm:"-"`
	PasswordHash    *string `gorm:"type:varchar(255)"`
	AvatarURL       *string `gorm:"type:varchar(255)"`
	AvatarObjectKey *string `gorm:"type:varchar(512)"`
	IsActive        bool    `gorm:"default:true"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (User) TableName() string {
	return "users"
}

// AfterCreate persists the initial verified primary email supplied with a new user.
func (u *User) AfterCreate(tx *gorm.DB) error {
	if u.Email == nil {
		return nil
	}
	email := strings.ToLower(strings.TrimSpace(*u.Email))
	if email == "" {
		return nil
	}
	now := time.Now()
	record := UserEmail{ID: uuid.NewString(), UserID: u.ID, Email: email, VerifiedAt: &now, IsPrimary: true}
	if err := tx.Create(&record).Error; err != nil {
		return err
	}
	u.Email = &email
	return nil
}
