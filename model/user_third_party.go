package model

type UserThirdParty struct {
	ID          uint   `gorm:"primaryKey;autoIncrement"`
	UserID      string `gorm:"type:varchar(36);not null;index"`
	Provider    string `gorm:"type:varchar(20);not null"`
	ProviderUID string `gorm:"type:varchar(100);not null"`
}

func (UserThirdParty) TableName() string {
	return "user_third_party"
}
