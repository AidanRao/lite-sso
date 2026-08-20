package main

import (
	"context"
	"log"
	"os"

	"sso-server/conf"
	"sso-server/dal/db"
	"sso-server/dal/kv"
	"sso-server/handler/server"
)

const migrateAfterListenArgument = "--migrate-after-listen"

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, arguments []string) error {
	cfg, err := conf.Load()
	if err != nil {
		return err
	}

	if containsArgument(arguments, migrateAfterListenArgument) {
		return runVercel(ctx, cfg, defaultVercelDependencies())
	}

	if err := db.Init(cfg); err != nil {
		return err
	}

	if err := kv.Init(cfg); err != nil {
		return err
	}

	srv, err := server.New(cfg)
	if err != nil {
		return err
	}
	log.Printf("Starting sso-server on %s", cfg.Server.Port)
	return srv.Start()
}

func containsArgument(arguments []string, expected string) bool {
	for _, argument := range arguments {
		if argument == expected {
			return true
		}
	}
	return false
}
