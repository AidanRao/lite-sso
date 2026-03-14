package model

type OAuthClient struct {
	ID           uint   `gorm:"primaryKey;autoIncrement"`
	Name         string `gorm:"type:varchar(50);not null"`
	ClientID     string `gorm:"type:varchar(50);uniqueIndex;not null"`
	ClientSecret string `gorm:"type:varchar(255);not null"`
	RedirectURIs string `gorm:"type:text;not null"`
}

func (OAuthClient) TableName() string {
	return "oauth_clients"
}
