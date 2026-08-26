package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/pressly/goose/v3"

	"sso-server/conf"
	"sso-server/dal/db/migration"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, arguments []string, output io.Writer) error {
	command := "up"
	var commandArguments []string
	if len(arguments) > 0 {
		command = arguments[0]
		commandArguments = arguments[1:]
	}

	if isFileCommand(command) {
		return executeFileCommand(command, commandArguments)
	}

	cfg, err := conf.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	return migration.Run(ctx, cfg, command, commandArguments, output)
}

func executeFileCommand(command string, arguments []string) error {
	switch command {
	case "create":
		if len(arguments) == 0 || len(arguments) > 2 {
			return fmt.Errorf("create requires NAME and optional [go|sql] arguments")
		}
		migrationType := "go"
		if len(arguments) == 2 {
			migrationType = arguments[1]
		}
		goose.SetSequential(true)
		return goose.Create(nil, migration.MigrationDir, arguments[0], migrationType)
	case "fix":
		if len(arguments) != 0 {
			return fmt.Errorf("fix does not accept arguments")
		}
		return goose.Fix(migration.MigrationDir)
	default:
		return fmt.Errorf("%q: not a file command", command)
	}
}

func isFileCommand(command string) bool {
	return command == "create" || command == "fix"
}
