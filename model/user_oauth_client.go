package model

import "time"

type UserOAuthClient struct {
	ID          uint      `gorm:"primaryKey;autoIncrement"`
	UserID      string    `gorm:"type:varchar(36);not null;uniqueIndex:idx_user_oauth_client"`
	ClientID    string    `gorm:"type:varchar(50);not null;uniqueIndex:idx_user_oauth_client"`
	LastLoginAt time.Time `gorm:"not null"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (UserOAuthClient) TableName() string {
	return "user_oauth_clients"
}
