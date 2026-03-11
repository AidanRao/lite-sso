package main

import (
	"log"

	"sso-server/conf"
	"sso-server/dal/db"
	"sso-server/dal/kv"
	"sso-server/handler/server"
)

func main() {
	cfg := conf.Load()

	if err := db.Init(cfg); err != nil {
		log.Fatal(err)
	}

	if err := kv.Init(cfg); err != nil {
		log.Fatal(err)
	}

	srv := server.New(cfg)
	log.Printf("Starting sso-server on %s", cfg.Server.Port)
	if err := srv.Start(); err != nil {
		log.Fatal(err)
	}
}
