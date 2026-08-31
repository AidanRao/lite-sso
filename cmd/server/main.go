package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"sso-server/conf"
	"sso-server/dal/db"
	"sso-server/dal/kv"
	"sso-server/handler/server"
)

const migrateAfterListenArgument = "--migrate-after-listen"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
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
	return srv.Start(ctx)
}

func containsArgument(arguments []string, expected string) bool {
	for _, argument := range arguments {
		if argument == expected {
			return true
		}
	}
	return false
}
