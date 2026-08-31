package user

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"sso-server/dto"
	"sso-server/model"
)

func Test_ListAuditLogs_OwnershipWindowSearchAndCursor(t *testing.T) {
	database, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	connection, err := database.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = connection.Close() })
	require.NoError(t, database.AutoMigrate(&model.AuditLog{}))
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	owner, other := "owner", "other"
	rows := []model.AuditLog{
		{ID: "00000000-0000-4000-8000-000000000003", UserID: &owner, OccurredAt: now.Add(-time.Hour), Action: "auth.login.third_party", Outcome: "success", IP: "192.0.2.1", Details: `{"provider":"github","completed_steps":["session_created"],"secret":"never-return"}`, SessionHash: "private-session-hash", DeviceID: "private-device", CreatedAt: now},
		{ID: "00000000-0000-4000-8000-000000000002", UserID: &owner, OccurredAt: now.Add(-time.Hour), Action: "user.password.update", Outcome: "failure", Details: `{}`},
		{ID: "00000000-0000-4000-8000-000000000001", UserID: &owner, OccurredAt: now.Add(-auditQueryWindow), Action: "user.email.add", Outcome: "success", TargetID: "literal_%!", Details: `{}`},
		{ID: uuid.NewString(), UserID: &owner, OccurredAt: now.Add(-auditQueryWindow - time.Microsecond), Action: "too.old", Details: `{}`},
		{ID: uuid.NewString(), UserID: &owner, OccurredAt: now, Action: "upper.bound", Details: `{}`},
		{ID: uuid.NewString(), UserID: &other, OccurredAt: now.Add(-time.Minute), Action: "auth.login.third_party", Outcome: "success", Details: `{"provider":"github"}`},
		{ID: uuid.NewString(), OccurredAt: now.Add(-time.Minute), Action: "auth.login.third_party", Outcome: "success", Details: `{"provider":"github"}`},
	}
	require.NoError(t, database.Create(&rows).Error)
	s := &UserService{db: database}
	ctx := context.Background()
	limit := 1
	query := dto.AuditLogQuery{Limit: &limit, From: now.Add(-365 * 24 * time.Hour).Format(time.RFC3339), To: now.Add(time.Hour).Format(time.RFC3339)}
	var got []string
	for range 3 {
		page, err := s.listAuditLogs(ctx, owner, query, now)
		require.NoError(t, err)
		require.Len(t, page.Items, 1)
		got = append(got, page.Items[0].ID)
		query.Cursor = page.NextCursor
		encoded, err := json.Marshal(page)
		require.NoError(t, err)
		for _, secret := range []string{"private-session-hash", "private-device", "never-return", "user_id", "created_at"} {
			require.NotContains(t, string(encoded), secret)
		}
	}
	require.Empty(t, query.Cursor)
	require.Equal(t, []string{rows[0].ID, rows[1].ID, rows[2].ID}, got)
	for _, test := range []struct {
		query dto.AuditLogQuery
		ids   []string
	}{
		{query: dto.AuditLogQuery{Q: "GITHUB"}, ids: []string{rows[0].ID}},
		{query: dto.AuditLogQuery{Q: "192.0.2.1", Actions: "user.password.update", Outcome: "failure"}, ids: []string{rows[1].ID}},
		{query: dto.AuditLogQuery{Q: "密码", Actions: "user.password.update"}, ids: []string{rows[1].ID}},
		{query: dto.AuditLogQuery{Q: "_%!"}, ids: []string{rows[2].ID}},
		{query: dto.AuditLogQuery{Q: "' OR 1=1 --"}, ids: []string{}},
		{query: dto.AuditLogQuery{From: now.Add(-time.Hour).Format(time.RFC3339), To: now.Add(-30 * time.Minute).Format(time.RFC3339)}, ids: []string{rows[0].ID, rows[1].ID}},
	} {
		page, err := s.listAuditLogs(ctx, owner, test.query, now)
		require.NoError(t, err)
		ids := []string{}
		for _, item := range page.Items {
			ids = append(ids, item.ID)
		}
		require.Equal(t, test.ids, ids)
	}
	// A valid cursor copied from another user can change position, never ownership.
	cursor, err := json.Marshal(auditCursor{Time: rows[5].OccurredAt, ID: rows[5].ID})
	require.NoError(t, err)
	page, err := s.listAuditLogs(ctx, owner, dto.AuditLogQuery{Cursor: base64.RawURLEncoding.EncodeToString(cursor)}, now)
	require.NoError(t, err)
	require.Len(t, page.Items, 3)
}

func Test_ParseAuditQuery_InvalidAndEmptyRanges(t *testing.T) {
	now := time.Now().UTC()
	zero, large := 0, 101
	for _, query := range []dto.AuditLogQuery{
		{Limit: &zero}, {Limit: &large}, {Outcome: "anything"}, {Q: strings.Repeat("字", 101)},
		{From: "yesterday"}, {To: "2026-08-01"}, {Cursor: "not-a-cursor"},
		{Actions: "auth.login.password,') OR true"}, {Actions: strings.Repeat("a,", 65)},
		{From: now.Format(time.RFC3339), To: now.Add(-time.Hour).Format(time.RFC3339)},
		{Cursor: base64.RawURLEncoding.EncodeToString([]byte(`{"time":"2026-08-31T00:00:00Z","id":"bad"}`))},
	} {
		_, err := parseAuditQuery("owner", query, now)
		require.ErrorIs(t, err, ErrAuditQueryInvalid)
	}
	_, err := parseAuditQuery("", dto.AuditLogQuery{}, now)
	require.ErrorIs(t, err, ErrAuditQueryInvalid)
	filter, err := parseAuditQuery("owner", dto.AuditLogQuery{To: now.Add(-31 * 24 * time.Hour).Format(time.RFC3339)}, now)
	require.NoError(t, err)
	require.False(t, filter.From.Before(filter.To))
}

func Test_ListAuditLogs_CurrentApplicationMetadata(t *testing.T) {
	database, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	connection, err := database.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = connection.Close() })
	require.NoError(t, database.AutoMigrate(&model.AuditLog{}, &model.OAuthClient{}))
	now := time.Now().UTC()
	owner, other := "owner", "other"
	clientID, missingID, otherID := "own-app", "deleted-app", "other-app"
	logo, objectKey := "https://example.com/logo.png", "private-object-key"
	homepage := "https://example.com/app"
	clients := []model.OAuthClient{
		{ClientID: clientID, Name: "我的应用", LogoURL: &logo, HomepageURL: homepage, ClientSecret: "private-client-secret", LogoObjectKey: &objectKey},
		{ClientID: otherID, Name: "other-user-app-name", ClientSecret: "other-secret"},
	}
	require.NoError(t, database.Create(&clients).Error)
	rows := []model.AuditLog{
		{ID: uuid.NewString(), UserID: &owner, ClientID: &clientID, OccurredAt: now.Add(-time.Minute), Action: "oauth.authorize", Outcome: "success", Details: `{}`},
		{ID: uuid.NewString(), UserID: &owner, ClientID: &clientID, OccurredAt: now.Add(-2 * time.Minute), Action: "oauth.authorize", Outcome: "success", Details: `{}`},
		{ID: uuid.NewString(), UserID: &owner, ClientID: &missingID, OccurredAt: now.Add(-3 * time.Minute), Action: "oauth.authorize", Outcome: "success", Details: `{}`},
		{ID: uuid.NewString(), UserID: &owner, OccurredAt: now.Add(-4 * time.Minute), Action: "auth.login.password", Outcome: "success", Details: `{}`},
		{ID: uuid.NewString(), UserID: &other, ClientID: &otherID, OccurredAt: now.Add(-time.Minute), Action: "oauth.authorize", Outcome: "success", Details: `{}`},
	}
	require.NoError(t, database.Create(&rows).Error)
	s := &UserService{db: database}
	ctx := context.Background()
	page, err := s.listAuditLogs(ctx, owner, dto.AuditLogQuery{}, now)
	require.NoError(t, err)
	require.Len(t, page.Items, 4)
	for _, item := range page.Items[:2] {
		require.Equal(t, &dto.AuditApplicationResponse{ClientID: clientID, Name: "我的应用", LogoURL: &logo, HomepageURL: homepage}, item.Application)
	}
	require.Nil(t, page.Items[2].Application)
	require.Equal(t, &missingID, page.Items[2].ClientID)
	require.Nil(t, page.Items[3].Application)
	encoded, err := json.Marshal(page)
	require.NoError(t, err)
	for _, secret := range []string{"client_secret", "private-client-secret", "logo_object_key", objectKey, otherID, "other-user-app-name"} {
		require.NotContains(t, string(encoded), secret)
	}

	// Display metadata follows the current application without changing the audit event.
	require.NoError(t, database.Model(&model.OAuthClient{}).Where("client_id = ?", clientID).
		Updates(map[string]any{"name": "已改名应用", "logo_url": nil, "homepage_url": ""}).Error)
	page, err = s.listAuditLogs(ctx, owner, dto.AuditLogQuery{}, now)
	require.NoError(t, err)
	require.Equal(t, &dto.AuditApplicationResponse{ClientID: clientID, Name: "已改名应用"}, page.Items[0].Application)
	require.Equal(t, rows[0].ID, page.Items[0].ID)
	require.NoError(t, database.Where("client_id = ?", clientID).Delete(&model.OAuthClient{}).Error)
	page, err = s.listAuditLogs(ctx, owner, dto.AuditLogQuery{}, now)
	require.NoError(t, err)
	require.Len(t, page.Items, 4)
	require.Nil(t, page.Items[0].Application)
	require.Equal(t, &clientID, page.Items[0].ClientID)
}
