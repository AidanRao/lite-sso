package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sso-server/dal/db/migration"
)

type fakeMigrationProvider struct {
	calls    []string
	statuses []*goose.MigrationStatus
	version  int64
	err      error
}

func (p *fakeMigrationProvider) Up(context.Context) ([]*goose.MigrationResult, error) {
	p.calls = append(p.calls, "up")
	return nil, p.err
}

func (p *fakeMigrationProvider) UpByOne(context.Context) (*goose.MigrationResult, error) {
	p.calls = append(p.calls, "up-by-one")
	return nil, p.err
}

func (p *fakeMigrationProvider) UpTo(_ context.Context, version int64) ([]*goose.MigrationResult, error) {
	p.calls = append(p.calls, "up-to:"+formatVersion(version))
	return nil, p.err
}

func (p *fakeMigrationProvider) Down(context.Context) (*goose.MigrationResult, error) {
	p.calls = append(p.calls, "down")
	return nil, p.err
}

func (p *fakeMigrationProvider) DownTo(_ context.Context, version int64) ([]*goose.MigrationResult, error) {
	p.calls = append(p.calls, "down-to:"+formatVersion(version))
	return nil, p.err
}

func (p *fakeMigrationProvider) Status(context.Context) ([]*goose.MigrationStatus, error) {
	p.calls = append(p.calls, "status")
	return p.statuses, p.err
}

func (p *fakeMigrationProvider) GetDBVersion(context.Context) (int64, error) {
	p.calls = append(p.calls, "version")
	return p.version, p.err
}

func Test_ExecuteProviderCommand_Dispatch(t *testing.T) {
	testCases := []struct {
		name        string
		command     string
		arguments   []string
		expectCalls []string
	}{
		{name: "up", command: "up", expectCalls: []string{"up"}},
		{name: "up by one", command: "up-by-one", expectCalls: []string{"up-by-one"}},
		{name: "up to", command: "up-to", arguments: []string{"3"}, expectCalls: []string{"up-to:3"}},
		{name: "down", command: "down", expectCalls: []string{"down"}},
		{name: "down to", command: "down-to", arguments: []string{"2"}, expectCalls: []string{"down-to:2"}},
		{name: "reset", command: "reset", expectCalls: []string{"down-to:0"}},
		{name: "redo", command: "redo", expectCalls: []string{"down", "up-by-one"}},
		{name: "status", command: "status", expectCalls: []string{"status"}},
		{name: "version", command: "version", expectCalls: []string{"version"}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			provider := &fakeMigrationProvider{}
			var output bytes.Buffer

			err := migration.ExecuteProviderCommand(
				context.Background(),
				provider,
				testCase.command,
				testCase.arguments,
				&output,
			)

			require.NoError(t, err)
			assert.Equal(t, testCase.expectCalls, provider.calls)
		})
	}
}

func Test_ParseVersion_Validation(t *testing.T) {
	testCases := []struct {
		name       string
		command    string
		arguments  []string
		expectCall string
		wantError  string
	}{
		{name: "positive", command: "up-to", arguments: []string{"12"}, expectCall: "up-to:12"},
		{name: "down to zero", command: "down-to", arguments: []string{"0"}, expectCall: "down-to:0"},
		{name: "missing", command: "up-to", wantError: "exactly one VERSION"},
		{name: "too many", command: "up-to", arguments: []string{"1", "2"}, wantError: "exactly one VERSION"},
		{name: "not a number", command: "down-to", arguments: []string{"latest"}, wantError: "must be a number"},
		{name: "up to zero", command: "up-to", arguments: []string{"0"}, wantError: "positive number"},
		{name: "negative", command: "down-to", arguments: []string{"-1"}, wantError: "zero or a positive number"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			provider := &fakeMigrationProvider{}
			err := migration.ExecuteProviderCommand(
				context.Background(),
				provider,
				testCase.command,
				testCase.arguments,
				&bytes.Buffer{},
			)
			if testCase.wantError != "" {
				require.ErrorContains(t, err, testCase.wantError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, []string{testCase.expectCall}, provider.calls)
		})
	}
}

func Test_NewMigrationProvider_LockerInitializationError(t *testing.T) {
	migrationFS := fstest.MapFS{
		"00001_init.sql": &fstest.MapFile{Data: []byte("-- +goose Up\nSELECT 1;\n")},
	}

	provider, err := migration.NewProvider(&sql.DB{}, migrationFS, 0, 1)

	assert.Nil(t, provider)
	require.ErrorContains(t, err, "create migration session locker")
}

func Test_NewMigrationProvider_ProviderInitializationError(t *testing.T) {
	provider, err := migration.NewProvider(&sql.DB{}, fstest.MapFS{}, 1, 1)

	assert.Nil(t, provider)
	require.ErrorContains(t, err, "create migration provider")
	require.ErrorIs(t, err, goose.ErrNoMigrations)
}

func Test_UserEmailsMigration_WrapsDollarQuotedBlock(t *testing.T) {
	content, err := os.ReadFile("../../migrations/00008_add_user_emails.sql")
	require.NoError(t, err)

	migrationSQL := string(content)
	statementBegin := strings.Index(migrationSQL, "-- +goose StatementBegin\nDO $$")
	statementEnd := strings.Index(migrationSQL, "END $$;\n-- +goose StatementEnd")
	require.NotEqual(t, -1, statementBegin)
	require.Greater(t, statementEnd, statementBegin)
}

func Test_ExecuteProviderCommand_MigrationError(t *testing.T) {
	migrationError := errors.New("migration failed")
	provider := &fakeMigrationProvider{err: migrationError}

	err := migration.ExecuteProviderCommand(context.Background(), provider, "up", nil, &bytes.Buffer{})

	require.ErrorIs(t, err, migrationError)
}

func Test_ExecuteProviderCommand_RedoStopsAfterDownError(t *testing.T) {
	migrationError := errors.New("down failed")
	provider := &fakeMigrationProvider{err: migrationError}

	err := migration.ExecuteProviderCommand(context.Background(), provider, "redo", nil, &bytes.Buffer{})

	require.ErrorIs(t, err, migrationError)
	assert.Equal(t, []string{"down"}, provider.calls)
}

func Test_ExecuteProviderCommand_StatusOutput(t *testing.T) {
	appliedAt := time.Date(2026, time.August, 14, 10, 30, 0, 0, time.UTC)
	provider := &fakeMigrationProvider{
		statuses: []*goose.MigrationStatus{
			{
				Source: &goose.Source{Path: "00001_initial.sql", Version: 1},
				State:  goose.StatePending,
			},
			{
				Source:    &goose.Source{Path: "nested/00002_users.sql", Version: 2},
				State:     goose.StateApplied,
				AppliedAt: appliedAt,
			},
		},
	}
	var output bytes.Buffer

	err := migration.ExecuteProviderCommand(context.Background(), provider, "status", nil, &output)

	require.NoError(t, err)
	assert.Contains(t, output.String(), "Pending")
	assert.Contains(t, output.String(), "00001_initial.sql")
	assert.Contains(t, output.String(), appliedAt.Format(time.ANSIC))
	assert.Contains(t, output.String(), "00002_users.sql")
}

func Test_ExecuteProviderCommand_VersionOutput(t *testing.T) {
	provider := &fakeMigrationProvider{version: 3}
	var output bytes.Buffer

	err := migration.ExecuteProviderCommand(context.Background(), provider, "version", nil, &output)

	require.NoError(t, err)
	assert.Equal(t, "goose: version 3\n", output.String())
}

func Test_ExecuteProviderCommand_UnknownCommand(t *testing.T) {
	err := migration.ExecuteProviderCommand(
		context.Background(),
		&fakeMigrationProvider{},
		"unknown",
		nil,
		&bytes.Buffer{},
	)

	require.ErrorContains(t, err, "no such command")
}

func Test_IsFileCommand_CommandType(t *testing.T) {
	assert.True(t, isFileCommand("create"))
	assert.True(t, isFileCommand("fix"))
	assert.False(t, isFileCommand("up"))
}

func formatVersion(version int64) string {
	return strconv.FormatInt(version, 10)
}
