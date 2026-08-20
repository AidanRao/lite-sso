// Package migration provides database migration execution for commands and application startup.
package migration

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	"sso-server/conf"
	"sso-server/dal/db"
)

const (
	MigrationDir         = "migrations"
	LockPeriod           = uint64(5)
	LockFailureThreshold = uint64(60)
)

// Provider describes the Goose operations supported by the migration command.
type Provider interface {
	Up(ctx context.Context) ([]*goose.MigrationResult, error)
	UpByOne(ctx context.Context) (*goose.MigrationResult, error)
	UpTo(ctx context.Context, version int64) ([]*goose.MigrationResult, error)
	Down(ctx context.Context) (*goose.MigrationResult, error)
	DownTo(ctx context.Context, version int64) ([]*goose.MigrationResult, error)
	Status(ctx context.Context) ([]*goose.MigrationStatus, error)
	GetDBVersion(ctx context.Context) (int64, error)
}

// Run opens the configured database and executes a migration command.
func Run(
	ctx context.Context,
	cfg *conf.Config,
	command string,
	arguments []string,
	output io.Writer,
) error {
	database, err := db.Open(cfg)
	if err != nil {
		return err
	}

	sqlDB, err := database.DB()
	if err != nil {
		return fmt.Errorf("get sql database: %w", err)
	}
	defer sqlDB.Close()

	provider, err := NewProvider(
		sqlDB,
		os.DirFS(MigrationDir),
		LockPeriod,
		LockFailureThreshold,
	)
	if err != nil {
		return err
	}

	return ExecuteProviderCommand(ctx, provider, command, arguments, output)
}

// Up applies all pending migrations using the production lock configuration.
func Up(ctx context.Context, cfg *conf.Config) error {
	return Run(ctx, cfg, "up", nil, io.Discard)
}

// NewProvider creates a Goose provider protected by a PostgreSQL session lock.
func NewProvider(
	sqlDB *sql.DB,
	migrationFS fs.FS,
	lockPeriod uint64,
	lockFailureMaximum uint64,
) (Provider, error) {
	locker, err := lock.NewPostgresSessionLocker(
		lock.WithLockTimeout(lockPeriod, lockFailureMaximum),
	)
	if err != nil {
		return nil, fmt.Errorf("create migration session locker: %w", err)
	}

	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		sqlDB,
		migrationFS,
		goose.WithSessionLocker(locker),
	)
	if err != nil {
		return nil, fmt.Errorf("create migration provider: %w", err)
	}
	return provider, nil
}

// ExecuteProviderCommand dispatches a database migration command to a Provider.
func ExecuteProviderCommand(
	ctx context.Context,
	provider Provider,
	command string,
	arguments []string,
	output io.Writer,
) error {
	switch command {
	case "up":
		_, err := provider.Up(ctx)
		return err
	case "up-by-one":
		_, err := provider.UpByOne(ctx)
		return err
	case "up-to":
		version, err := parseVersion(command, arguments, false)
		if err != nil {
			return err
		}
		_, err = provider.UpTo(ctx, version)
		return err
	case "down":
		_, err := provider.Down(ctx)
		return err
	case "down-to":
		version, err := parseVersion(command, arguments, true)
		if err != nil {
			return err
		}
		_, err = provider.DownTo(ctx, version)
		return err
	case "reset":
		_, err := provider.DownTo(ctx, 0)
		return err
	case "redo":
		if _, err := provider.Down(ctx); err != nil {
			return err
		}
		_, err := provider.UpByOne(ctx)
		return err
	case "status":
		return printStatus(ctx, provider, output)
	case "version":
		return printVersion(ctx, provider, output)
	default:
		return fmt.Errorf("%q: no such command", command)
	}
}

func parseVersion(command string, arguments []string, allowZero bool) (int64, error) {
	if len(arguments) != 1 {
		return 0, fmt.Errorf("%s requires exactly one VERSION argument", command)
	}

	version, err := strconv.ParseInt(arguments[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("version must be a number (got %q)", arguments[0])
	}
	if version < 0 || !allowZero && version == 0 {
		if allowZero {
			return 0, fmt.Errorf("version must be zero or a positive number (got %d)", version)
		}
		return 0, fmt.Errorf("version must be a positive number (got %d)", version)
	}
	return version, nil
}

func printStatus(ctx context.Context, provider Provider, output io.Writer) error {
	statuses, err := provider.Status(ctx)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintln(output, "    Applied At                  Migration"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(output, "    ======================================="); err != nil {
		return err
	}
	for _, status := range statuses {
		appliedAt := "Pending"
		if status.State == goose.StateApplied {
			appliedAt = status.AppliedAt.Format(time.ANSIC)
		}
		if _, err := fmt.Fprintf(
			output,
			"    %-24s -- %s\n",
			appliedAt,
			filepath.Base(status.Source.Path),
		); err != nil {
			return err
		}
	}
	return nil
}

func printVersion(ctx context.Context, provider Provider, output io.Writer) error {
	version, err := provider.GetDBVersion(ctx)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "goose: version %d\n", version)
	return err
}
