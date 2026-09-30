package model

// FeatureConfig stores a registered feature's release policy.
type FeatureConfig struct {
	Key        string        `json:"key" gorm:"primaryKey;size:128"`
	Audience   string        `json:"audience" gorm:"not null"`
	Percentage int           `json:"percentage" gorm:"not null"`
	Stage      string        `json:"stage" gorm:"not null"`
	Users      []FeatureUser `json:"-" gorm:"foreignKey:FeatureKey;references:Key"`
}

// FeatureUser grants a user access in selected and percentage modes.
type FeatureUser struct {
	FeatureKey string `json:"feature_key" gorm:"primaryKey;size:128"`
	UserID     string `json:"user_id" gorm:"primaryKey;size:36"`
}
