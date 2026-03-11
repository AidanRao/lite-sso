package db

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"sso-server/conf"
	"sso-server/model"
)

var DB *gorm.DB

func Init(cfg *conf.Config) error {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Name,
	)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return err
	}

	return DB.AutoMigrate(
		&model.User{},
		&model.OAuthClient{},
		&model.UserThirdParty{},
	)
}
