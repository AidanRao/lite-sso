package main

import (
	"context"
	"log"
	"os"

	"github.com/pressly/goose/v3"

	"sso-server/conf"
	"sso-server/dal/db"
)

const migrationDir = "migrations"

func main() {
	command := "up"
	var args []string
	if len(os.Args) > 1 {
		command = os.Args[1]
		args = os.Args[2:]
	}

	if command == "create" {
		goose.SetSequential(true)
		if err := goose.RunContext(context.Background(), command, nil, migrationDir, args...); err != nil {
			log.Fatal(err)
		}
		return
	}

	cfg, err := conf.Load()
	if err != nil {
		log.Fatal(err)
	}

	database, err := db.Open(cfg)
	if err != nil {
		log.Fatal(err)
	}

	sqlDB, err := database.DB()
	if err != nil {
		log.Fatal(err)
	}
	defer sqlDB.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatal(err)
	}
	if err := goose.RunContext(context.Background(), command, sqlDB, migrationDir, args...); err != nil {
		log.Fatal(err)
	}
}
