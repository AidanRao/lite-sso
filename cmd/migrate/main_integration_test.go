package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3/lock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sso-server/dal/db/migration"
)

const migrationIntegrationDatabaseEnvironment = "MIGRATION_TEST_DATABASE_DSN"

type postgresTestDatabase struct {
	adminDB          *sql.DB
	connectionConfig *pgx.ConnConfig
}

func Test_MigrationProvider_ConcurrentUp(t *testing.T) {
	testDatabase := newPostgresTestDatabase(t)
	const providerCount = 5

	providers := make([]migration.Provider, 0, providerCount)
	databases := make([]*sql.DB, 0, providerCount)
	for range providerCount {
		database := testDatabase.open(t)
		databases = append(databases, database)
		provider, err := migration.NewProvider(
			database,
			os.DirFS("../../migrations"),
			1,
			11,
		)
		require.NoError(t, err)
		providers = append(providers, provider)
	}
	t.Cleanup(func() {
		for _, database := range databases {
			require.NoError(t, database.Close())
		}
	})

	start := make(chan struct{})
	errorsFromProviders := make(chan error, providerCount)
	var waitGroup sync.WaitGroup
	for _, provider := range providers {
		waitGroup.Add(1)
		go func(provider migration.Provider) {
			defer waitGroup.Done()
			<-start
			_, err := provider.Up(context.Background())
			errorsFromProviders <- err
		}(provider)
	}
	close(start)
	waitGroup.Wait()
	close(errorsFromProviders)

	for err := range errorsFromProviders {
		require.NoError(t, err)
	}

	assertMigrationVersionsRecordedOnce(t, databases[0])
	assertFinalDatabaseStructure(t, databases[0])
}

func Test_MigrationProvider_LockTimeout(t *testing.T) {
	testDatabase := newPostgresTestDatabase(t)
	holderConnection, err := testDatabase.adminDB.Conn(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, holderConnection.Close())
	})

	_, err = holderConnection.ExecContext(
		context.Background(),
		"SELECT pg_advisory_lock($1)",
		lock.DefaultLockID,
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, unlockError := holderConnection.ExecContext(
			context.Background(),
			"SELECT pg_advisory_unlock($1)",
			lock.DefaultLockID,
		)
		require.NoError(t, unlockError)
	})

	database := testDatabase.open(t)
	t.Cleanup(func() {
		require.NoError(t, database.Close())
	})
	provider, err := migration.NewProvider(database, os.DirFS("../../migrations"), 1, 1)
	require.NoError(t, err)

	startedAt := time.Now()
	_, err = provider.Up(context.Background())
	elapsed := time.Since(startedAt)

	require.ErrorContains(t, err, "failed to acquire lock")
	assert.GreaterOrEqual(t, elapsed, time.Second)
	assert.Less(t, elapsed, 3*time.Second)
}

func Test_UserEmailsMigration_AllowsUsersWithoutEmail(t *testing.T) {
	testDatabase := newPostgresTestDatabase(t)
	database := testDatabase.open(t)
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	provider, err := migration.NewProvider(database, os.DirFS("../../migrations"), 1, 11)
	require.NoError(t, err)

	_, err = provider.UpTo(context.Background(), 7)
	require.NoError(t, err)
	_, err = database.ExecContext(context.Background(), `
		INSERT INTO users (id, email, is_active, created_at, updated_at)
		VALUES
			('without-email', NULL, true, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('with-email', ' Owner@Example.COM ', true, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	require.NoError(t, err)

	_, err = provider.UpTo(context.Background(), 8)
	require.NoError(t, err)
	var withoutEmailCount int
	require.NoError(t, database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM user_emails WHERE user_id = 'without-email'").Scan(&withoutEmailCount))
	assert.Zero(t, withoutEmailCount)
	var primaryEmail string
	require.NoError(t, database.QueryRowContext(context.Background(), "SELECT email FROM user_emails WHERE user_id = 'with-email' AND is_primary").Scan(&primaryEmail))
	assert.Equal(t, "owner@example.com", primaryEmail)

	_, err = provider.Down(context.Background())
	require.NoError(t, err)
	var restoredEmail sql.NullString
	require.NoError(t, database.QueryRowContext(context.Background(), "SELECT email FROM users WHERE id = 'without-email'").Scan(&restoredEmail))
	assert.False(t, restoredEmail.Valid)
}

func Test_ThirdPartyProfileMigration_UpAndDown(t *testing.T) {
	testDatabase := newPostgresTestDatabase(t)
	database := testDatabase.open(t)
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	provider, err := migration.NewProvider(database, os.DirFS("../../migrations"), 1, 11)
	require.NoError(t, err)
	ctx := context.Background()
	_, err = provider.UpTo(ctx, 14)
	require.NoError(t, err)
	_, err = database.ExecContext(ctx, "INSERT INTO user_third_party (user_id, provider, provider_uid) VALUES ('u1', 'github', 'identity')")
	require.NoError(t, err)
	_, err = provider.UpTo(ctx, 15)
	require.NoError(t, err)
	var email, username string
	require.NoError(t, database.QueryRowContext(ctx, "SELECT email, username FROM user_third_party WHERE user_id = 'u1'").Scan(&email, &username))
	assert.Empty(t, email)
	assert.Empty(t, username)
	_, err = database.ExecContext(ctx, "UPDATE user_third_party SET email = 'github@example.com', username = 'github-login' WHERE user_id = 'u1'")
	require.NoError(t, err)
	_, err = provider.Down(ctx)
	require.NoError(t, err)
	var providerUID string
	require.NoError(t, database.QueryRowContext(ctx, "SELECT provider_uid FROM user_third_party WHERE user_id = 'u1'").Scan(&providerUID))
	assert.Equal(t, "identity", providerUID)
	var profileColumnCount int
	require.NoError(t, database.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'user_third_party' AND column_name IN ('email', 'username')").Scan(&profileColumnCount))
	assert.Zero(t, profileColumnCount)
}

func Test_ThirdPartyTimestampsMigration_UpAndDown(t *testing.T) {
	testDatabase := newPostgresTestDatabase(t)
	database := testDatabase.open(t)
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	provider, err := migration.NewProvider(database, os.DirFS("../../migrations"), 1, 11)
	require.NoError(t, err)
	ctx := context.Background()
	_, err = provider.UpTo(ctx, 15)
	require.NoError(t, err)
	_, err = database.ExecContext(ctx, "INSERT INTO user_third_party (user_id, provider, provider_uid) VALUES ('u1', 'github', 'identity')")
	require.NoError(t, err)
	_, err = provider.UpTo(ctx, 16)
	require.NoError(t, err)
	var boundAt, lastUsedAt sql.NullTime
	require.NoError(t, database.QueryRowContext(ctx, "SELECT bound_at, last_used_at FROM user_third_party WHERE user_id = 'u1'").Scan(&boundAt, &lastUsedAt))
	assert.False(t, boundAt.Valid)
	assert.False(t, lastUsedAt.Valid)
	_, err = provider.Down(ctx)
	require.NoError(t, err)
	var providerUID string
	require.NoError(t, database.QueryRowContext(ctx, "SELECT provider_uid FROM user_third_party WHERE user_id = 'u1'").Scan(&providerUID))
	assert.Equal(t, "identity", providerUID)
	var columnCount int
	require.NoError(t, database.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'user_third_party' AND column_name IN ('bound_at', 'last_used_at')").Scan(&columnCount))
	assert.Zero(t, columnCount)
}

func Test_OAuthClientRedirectURIsMigration_UpAndDown(t *testing.T) {
	testDatabase := newPostgresTestDatabase(t)
	database := testDatabase.open(t)
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	provider, err := migration.NewProvider(database, os.DirFS("../../migrations"), 1, 11)
	require.NoError(t, err)
	ctx := t.Context()
	_, err = provider.UpTo(ctx, 16)
	require.NoError(t, err)
	_, err = database.ExecContext(ctx, `INSERT INTO oauth_clients (name, client_id, client_secret, homepage_url, redirect_uri)
		VALUES ('App', 'app', 'secret', 'https://app.example.com', 'https://app.example.com/callback?source=sso'),
		       ('Empty', 'empty', 'secret', 'https://empty.example.com', '')`)
	require.NoError(t, err)
	_, err = provider.UpTo(ctx, 17)
	require.NoError(t, err)
	var callbacks, columnType string
	require.NoError(t, database.QueryRowContext(ctx, "SELECT redirect_uris::text FROM oauth_clients WHERE client_id = 'app'").Scan(&callbacks))
	assert.JSONEq(t, `["https://app.example.com/callback?source=sso"]`, callbacks)
	require.NoError(t, database.QueryRowContext(ctx, "SELECT redirect_uris::text FROM oauth_clients WHERE client_id = 'empty'").Scan(&callbacks))
	assert.JSONEq(t, `[]`, callbacks)
	require.NoError(t, database.QueryRowContext(ctx, "SELECT data_type FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'oauth_clients' AND column_name = 'redirect_uris'").Scan(&columnType))
	assert.Equal(t, "jsonb", columnType)
	_, err = database.ExecContext(ctx, `UPDATE oauth_clients SET redirect_uris = '["https://app.example.com/callback?source=sso","app://oauth/callback"]'::jsonb WHERE client_id = 'app'`)
	require.NoError(t, err)
	_, err = provider.Down(ctx)
	require.NoError(t, err)
	require.NoError(t, database.QueryRowContext(ctx, "SELECT redirect_uri FROM oauth_clients WHERE client_id = 'app'").Scan(&callbacks))
	assert.Equal(t, "https://app.example.com/callback?source=sso", callbacks)
	require.NoError(t, database.QueryRowContext(ctx, "SELECT redirect_uri FROM oauth_clients WHERE client_id = 'empty'").Scan(&callbacks))
	assert.Empty(t, callbacks)
	_, err = provider.UpTo(ctx, 17)
	require.NoError(t, err)
}

func Test_MigrationLock_ProductionTimeout(t *testing.T) {
	assert.Equal(t, uint64(5), migration.LockPeriod)
	assert.Equal(t, uint64(60), migration.LockFailureThreshold)
	assert.Equal(t, uint64(300), migration.LockPeriod*migration.LockFailureThreshold)
}

func newPostgresTestDatabase(t *testing.T) *postgresTestDatabase {
	t.Helper()

	dsn := os.Getenv(migrationIntegrationDatabaseEnvironment)
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL migration integration tests", migrationIntegrationDatabaseEnvironment)
	}

	adminConfig, err := pgx.ParseConfig(dsn)
	require.NoError(t, err)
	adminDB := stdlib.OpenDB(*adminConfig)
	require.NoError(t, adminDB.PingContext(context.Background()))

	schema := "goose_provider_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = adminDB.ExecContext(
		context.Background(),
		fmt.Sprintf("CREATE SCHEMA %s", schema),
	)
	require.NoError(t, err)

	connectionConfig := adminConfig.Copy()
	if connectionConfig.RuntimeParams == nil {
		connectionConfig.RuntimeParams = make(map[string]string)
	}
	connectionConfig.RuntimeParams["search_path"] = schema

	t.Cleanup(func() {
		_, dropError := adminDB.ExecContext(
			context.Background(),
			fmt.Sprintf("DROP SCHEMA %s CASCADE", schema),
		)
		require.NoError(t, dropError)
		require.NoError(t, adminDB.Close())
	})

	return &postgresTestDatabase{
		adminDB:          adminDB,
		connectionConfig: connectionConfig,
	}
}

func assertMigrationVersionsRecordedOnce(t *testing.T, database *sql.DB) {
	t.Helper()

	rows, err := database.QueryContext(
		context.Background(),
		"SELECT version_id, COUNT(*) FROM goose_db_version WHERE is_applied AND version_id > 0 GROUP BY version_id ORDER BY version_id",
	)
	require.NoError(t, err)
	defer rows.Close()

	versions := make(map[int64]int)
	for rows.Next() {
		var version int64
		var count int
		require.NoError(t, rows.Scan(&version, &count))
		versions[version] = count
	}
	require.NoError(t, rows.Err())
	assert.Equal(t, map[int64]int{1: 1, 2: 1, 3: 1, 4: 1, 5: 1, 6: 1, 7: 1, 8: 1, 9: 1, 10: 1, 11: 1, 12: 1, 13: 1, 14: 1, 15: 1, 16: 1, 17: 1, 18: 1, 19: 1}, versions)
}

func assertFinalDatabaseStructure(t *testing.T, database *sql.DB) {
	t.Helper()

	var tableCount int
	err := database.QueryRowContext(
		context.Background(),
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = current_schema() AND table_name IN ('users', 'oauth_clients', 'user_third_party', 'user_oauth_clients', 'user_session', 'webauthn_users', 'webauthn_credentials', 'user_emails', 'user_email_sources', 'audit_logs', 'user_deletions', 'user_cleanup_tasks')",
	).Scan(&tableCount)
	require.NoError(t, err)
	assert.Equal(t, 12, tableCount)

	var legacyEmailColumnCount int
	err = database.QueryRowContext(
		context.Background(),
		"SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'users' AND column_name = 'email'",
	).Scan(&legacyEmailColumnCount)
	require.NoError(t, err)
	assert.Zero(t, legacyEmailColumnCount)

	rows, err := database.QueryContext(
		context.Background(),
		"SELECT column_name FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'oauth_clients'",
	)
	require.NoError(t, err)
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var column string
		require.NoError(t, rows.Scan(&column))
		columns[column] = true
	}
	require.NoError(t, rows.Err())
	assert.True(t, columns["redirect_uris"])
	assert.True(t, columns["registration_id"])
	assert.True(t, columns["homepage_url"])
	assert.True(t, columns["logout_uri"])
	assert.True(t, columns["client_type"])
	assert.False(t, columns["redirect_uri"])
	assert.False(t, columns["logout_uris"])
}

func (d *postgresTestDatabase) open(t *testing.T) *sql.DB {
	t.Helper()
	database := stdlib.OpenDB(*d.connectionConfig.Copy())
	require.NoError(t, database.PingContext(context.Background()))
	return database
}

func Test_UserStatusMigration_PreservesSuspensionAndCanRollback(t *testing.T) {
	testDatabase := newPostgresTestDatabase(t)
	database := testDatabase.open(t)
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	provider, err := migration.NewProvider(database, os.DirFS("../../migrations"), 1, 10)
	require.NoError(t, err)
	_, err = provider.UpTo(t.Context(), 13)
	require.NoError(t, err)
	_, err = database.ExecContext(t.Context(), "INSERT INTO users (id,is_active) VALUES ('active',true),('disabled',false)")
	require.NoError(t, err)
	_, err = provider.UpTo(t.Context(), 14)
	require.NoError(t, err)
	var active, disabled string
	require.NoError(t, database.QueryRowContext(t.Context(), "SELECT status FROM users WHERE id = 'active'").Scan(&active))
	require.NoError(t, database.QueryRowContext(t.Context(), "SELECT status FROM users WHERE id = 'disabled'").Scan(&disabled))
	assert.Equal(t, "active", active)
	assert.Equal(t, "disabled", disabled)
	_, err = provider.Down(t.Context())
	require.NoError(t, err)
	var enabled bool
	require.NoError(t, database.QueryRowContext(t.Context(), "SELECT is_active FROM users WHERE id = 'disabled'").Scan(&enabled))
	assert.False(t, enabled)
	_, err = provider.Up(t.Context())
	require.NoError(t, err)
}

func Test_SessionRememberMeMigration_RevokesExistingSessions(t *testing.T) {
	testDatabase := newPostgresTestDatabase(t)
	database := testDatabase.open(t)
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	provider, err := migration.NewProvider(database, os.DirFS("../../migrations"), 1, 11)
	require.NoError(t, err)
	_, err = provider.UpTo(context.Background(), 18)
	require.NoError(t, err)
	_, err = database.ExecContext(context.Background(), `
 INSERT INTO user_session (id,user_id,device_id,auth_method,refresh_token_hash,ip,user_agent,created_at,last_seen_at,expires_at,revoked_at,revoke_reason)
 VALUES
 ('active','user','dev_test','PASSWORD',repeat('a',64),'127.0.0.1','test',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP+INTERVAL '1 day',NULL,NULL),
 ('revoked','user','dev_test','PASSWORD',repeat('b',64),'127.0.0.1','test',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP+INTERVAL '1 day',CURRENT_TIMESTAMP,'logout')`)
	require.NoError(t, err)
	_, err = provider.UpTo(context.Background(), 19)
	require.NoError(t, err)
	var revoked bool
	var reason string
	var remember bool
	require.NoError(t, database.QueryRowContext(context.Background(), "SELECT revoked_at IS NOT NULL,revoke_reason,remember_me FROM user_session WHERE id='active'").Scan(&revoked, &reason, &remember))
	require.True(t, revoked)
	require.Equal(t, "session_policy_changed", reason)
	require.False(t, remember)
	require.NoError(t, database.QueryRowContext(context.Background(), "SELECT revoke_reason FROM user_session WHERE id='revoked'").Scan(&reason))
	require.Equal(t, "logout", reason)
	var nullable string
	var columnDefault sql.NullString
	require.NoError(t, database.QueryRowContext(context.Background(), "SELECT is_nullable,column_default FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='user_session' AND column_name='remember_me'").Scan(&nullable, &columnDefault))
	require.Equal(t, "NO", nullable)
	require.False(t, columnDefault.Valid)
	_, err = database.ExecContext(context.Background(), `
 INSERT INTO user_session (id,user_id,device_id,auth_method,refresh_token_hash,ip,user_agent,created_at,last_seen_at,expires_at,remember_me)
 VALUES ('new','user','dev_test','PASSWORD',repeat('c',64),'127.0.0.1','test',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP+INTERVAL '1 day',true)`)
	require.NoError(t, err)
	require.NoError(t, database.QueryRowContext(context.Background(), "SELECT remember_me FROM user_session WHERE id='new'").Scan(&remember))
	require.True(t, remember)
	_, err = provider.Down(context.Background())
	require.NoError(t, err)
	// Rolling back the column must not restore revoked credentials.
	require.NoError(t, database.QueryRowContext(context.Background(), "SELECT revoked_at IS NOT NULL FROM user_session WHERE id='active'").Scan(&revoked))
	require.True(t, revoked)
}
