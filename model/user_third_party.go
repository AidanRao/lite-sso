package model

type UserThirdParty struct {
	ID          uint   `gorm:"primaryKey;autoIncrement"`
	UserID      string `gorm:"type:varchar(36);not null;index;uniqueIndex:idx_user_provider"`
	Provider    string `gorm:"type:varchar(20);not null;uniqueIndex:idx_user_provider;uniqueIndex:idx_provider_uid"`
	ProviderUID string `gorm:"type:varchar(100);not null;uniqueIndex:idx_provider_uid"`
}

func (UserThirdParty) TableName() string {
	return "user_third_party"
}
