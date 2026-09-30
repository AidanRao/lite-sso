package feature

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"sso-server/dto"
	"sso-server/model"
)

func Test_Bucket_StableContract(t *testing.T) {
	// Values calculated independently with Python hashlib.
	require.Equal(t, uint64(8615), Bucket(AuditLogs, "user-1"))
}

func Test_Enabled_BoundariesAndMonotonicity(t *testing.T) {
	policy := model.FeatureConfig{Key: AuditLogs, Audience: "percentage", Stage: "beta"}
	for i := 0; i < 1000; i++ {
		id := fmt.Sprintf("user-%d", i)
		policy.Percentage = 0
		require.False(t, Enabled(policy, id))
		policy.Percentage = 100
		require.True(t, Enabled(policy, id))
		last := false
		for p := 0; p <= 100; p++ {
			policy.Percentage = p
			current := Enabled(policy, id)
			if last {
				require.True(t, current)
			}
			require.Equal(t, Bucket(AuditLogs, id) < uint64(p*100), current)
			last = current
		}
	}
	policy.Users = []model.FeatureUser{{FeatureKey: AuditLogs, UserID: "chosen"}}
	policy.Percentage = 0
	require.True(t, Enabled(policy, "chosen"))
	policy.Audience = "selected"
	require.True(t, Enabled(policy, "chosen"))
	require.False(t, Enabled(policy, "other"))
	policy.Stage = "stable"
	require.True(t, Enabled(policy, "chosen"))
	policy.Audience = "off"
	require.False(t, Enabled(policy, "chosen"))
	policy.Audience = "all"
	require.True(t, Enabled(policy, "other"))
	require.False(t, Enabled(policy, ""))
}

func Test_Service_PersistenceValidationAndRollback(t *testing.T) {
	database, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	connection, err := database.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = connection.Close() })
	require.NoError(t, database.AutoMigrate(&model.User{}, &model.FeatureConfig{}, &model.FeatureUser{}))
	require.NoError(t, database.Create(&model.User{ID: "chosen"}).Error)
	service := NewService(database)
	ctx := context.Background()
	states, err := service.States(ctx, "chosen")
	require.NoError(t, err)
	require.False(t, states[AuditLogs].Enabled)
	percentage := 25
	req := dto.FeatureUpdate{Audience: "selected", Percentage: &percentage, Stage: "beta", UserIDs: []string{"chosen"}}
	fields, err := service.Update(ctx, AuditLogs, req)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"audience", "percentage", "user_ids"}, fields)
	fields, err = service.Update(ctx, AuditLogs, req)
	require.NoError(t, err)
	require.Empty(t, fields)
	states, err = service.States(ctx, "chosen")
	require.NoError(t, err)
	require.True(t, states[AuditLogs].Enabled)
	states, err = service.States(ctx, "other")
	require.NoError(t, err)
	require.False(t, states[AuditLogs].Enabled)
	for _, bad := range []dto.FeatureUpdate{
		{Audience: "wrong", Percentage: &percentage, Stage: "beta", UserIDs: []string{}},
		{Audience: "all", Stage: "beta", UserIDs: []string{}},
		{Audience: "all", Percentage: &percentage, Stage: "wrong", UserIDs: []string{}},
		{Audience: "all", Percentage: &percentage, Stage: "stable", UserIDs: []string{"missing"}},
	} {
		_, err = service.Update(ctx, AuditLogs, bad)
		require.ErrorIs(t, err, ErrInvalidPolicy)
	}
	_, err = service.Update(ctx, "unregistered", req)
	require.ErrorIs(t, err, ErrInvalidPolicy)
	// Fail after the config update and membership deletion; the original policy must survive.
	require.NoError(t, database.Exec("CREATE TRIGGER reject_feature_user BEFORE INSERT ON feature_users BEGIN SELECT RAISE(ABORT, 'test failure'); END").Error)
	req.Audience = "off"
	_, err = service.Update(ctx, AuditLogs, req)
	require.Error(t, err)
	states, err = service.States(ctx, "chosen")
	require.NoError(t, err)
	require.True(t, states[AuditLogs].Enabled)
	require.NoError(t, database.Migrator().DropTable(&model.FeatureUser{}))
	_, err = service.States(ctx, "chosen")
	require.Error(t, err)
}

func Test_Migration_InitialPolicyAndConstraints(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	connection, err := database.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = connection.Close() })
	require.NoError(t, database.Exec("PRAGMA foreign_keys = ON").Error)
	require.NoError(t, database.Exec("CREATE TABLE users (id varchar(36) PRIMARY KEY)").Error)
	sql, err := os.ReadFile("../../migrations/00010_add_feature_releases.sql")
	require.NoError(t, err)
	parts := strings.Split(string(sql), "-- +goose Down")
	require.NoError(t, database.Exec(parts[0]).Error)
	states, err := NewService(database).States(context.Background(), "existing-user")
	require.NoError(t, err)
	require.Equal(t, dto.FeatureState{Enabled: true, Stage: "stable"}, states[AuditLogs])
	require.Error(t, database.Exec("UPDATE feature_configs SET percentage = 101").Error)
	require.Error(t, database.Exec("INSERT INTO feature_users VALUES ('profile.audit_logs','missing')").Error)
	require.NoError(t, database.Exec(parts[1]).Error)
	require.False(t, database.Migrator().HasTable("feature_configs"))
}
