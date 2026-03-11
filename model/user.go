package model

import "time"

type User struct {
	ID           string  `gorm:"type:varchar(36);primaryKey"`
	Username     *string `gorm:"type:varchar(50);uniqueIndex"`
	Email        string  `gorm:"type:varchar(100);uniqueIndex;not null"`
	PasswordHash *string `gorm:"type:varchar(255)"`
	AvatarURL    *string `gorm:"type:varchar(255)"`
	IsActive     bool    `gorm:"default:true"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (User) TableName() string {
	return "users"
}
