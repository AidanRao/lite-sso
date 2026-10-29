package notification

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"sso-server/common/event"
	"sso-server/conf"
	"sso-server/manager/eventbus"
	"sso-server/manager/messagecenter"
	"sso-server/manager/thirdparty"
	"sso-server/model"
	"sso-server/service/auth"
)

type captureSender struct {
	calls     int
	variables map[string]string
	err       error
}

func (s *captureSender) Send(_ context.Context, _, _ string, variables map[string]string) error {
	s.calls++
	s.variables = variables
	return s.err
}

func notificationDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	connection, err := database.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = connection.Close() })
	require.NoError(t, database.AutoMigrate(&model.User{}, &model.UserEmail{}, &model.UserSession{}))
	return database
}

func Test_HandleLogin_OnlyCurrentVerifiedPrimaryExistingAtLogin(t *testing.T) {
	for _, scenario := range []string{"verified primary", "no email", "unverified primary", "verified secondary only", "bound later", "verified later", "existing device", "deleted account"} {
		t.Run(scenario, func(t *testing.T) {
			database := notificationDatabase(t)
			require.NoError(t, database.Create(&model.User{ID: "u1", Status: model.UserStatusActive}).Error)
			before := time.Now().Add(-time.Hour)
			login := event.LoginSucceeded{UserID: "u1", DeviceID: "dev_browser", NewDevice: true, OccurredAt: time.Now()}
			if scenario != "no email" && scenario != "deleted account" {
				record := model.UserEmail{ID: uuid.NewString(), UserID: "u1", Email: "primary@example.com", IsPrimary: true, VerifiedAt: &before, CreatedAt: before}
				switch scenario {
				case "unverified primary":
					record.VerifiedAt = nil
				case "verified secondary only":
					record.IsPrimary = false
				case "bound later":
					record.CreatedAt = login.OccurredAt.Add(time.Minute)
				case "verified later":
					after := login.OccurredAt.Add(time.Minute)
					record.VerifiedAt = &after
				case "existing device":
					login.NewDevice = false
				}
				require.NoError(t, database.Create(&record).Error)
			}
			if scenario == "deleted account" {
				require.NoError(t, database.Delete(&model.User{}, "id = ?", "u1").Error)
			}
			sender := &captureSender{}
			notifier := NewLoginNotifier(database, sender, " https://sso.example.com/ ", nil)
			require.NoError(t, notifier.HandleLogin(t.Context(), login))
			if scenario == "verified primary" {
				require.Equal(t, 1, sender.calls)
				require.Equal(t, "https://sso.example.com/profile/access/sessions", sender.variables["security_url"])
				require.Equal(t, "用户", sender.variables["username"])
				require.Equal(t, "未知设备", sender.variables["device"])
				require.Equal(t, "未知", sender.variables["ip"])
				require.Equal(t, "未知", sender.variables["location"])
			} else {
				require.Zero(t, sender.calls)
			}
		})
	}
}

func Test_HandleLogin_SubmitsCommittedLoginToMessageCenter(t *testing.T) {
	database := notificationDatabase(t)
	username, email := "测试用户 <script>", "primary@example.com"
	require.NoError(t, database.Create(&model.User{ID: "u1", Username: &username, Email: &email, Status: model.UserStatusActive}).Error)
	var received struct {
		SenderKey   string            `json:"senderKey"`
		TemplateKey string            `json:"templateKey"`
		Target      string            `json:"target"`
		Variables   map[string]string `json:"variables"`
		Channel     string            `json:"channel"`
	}
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer remote.Close()
	client, err := messagecenter.NewClient(messagecenter.Config{URL: remote.URL, APIKey: "test-key", SenderKey: "sso"})
	require.NoError(t, err)
	notifier := NewLoginNotifier(database, client, "https://sso.example.com", thirdparty.NewBuiltins(nil))
	bus := eventbus.New(eventbus.Subscription{Type: event.LoginSucceededType, Handle: notifier.HandleLogin})
	t.Cleanup(func() { _ = bus.Shutdown(t.Context()) })
	cfg := &conf.Config{Auth: conf.AuthConfig{JWTSecret: "test-key"}, Tokens: conf.TokenConfig{Session: conf.SessionTokenConfig{AccessTokenTTL: time.Minute, RefreshTokenTTL: time.Hour}}}
	service := auth.NewAuthService(cfg, database, nil, nil, nil, nil, bus)
	_, pair, err := service.CompleteLoginWithContext(t.Context(), "u1", "", auth.LoginMetadata{DeviceID: "dev_browser", IP: "192.0.2.1", UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"}, auth.AuthMethodPassword, auth.LoginOptions{})
	require.NoError(t, err)
	require.NoError(t, bus.Shutdown(t.Context()))
	require.Equal(t, "primary@example.com", received.Target)
	require.Equal(t, "new_device_login_template_key", received.TemplateKey)
	require.Equal(t, "sso", received.SenderKey)
	require.Equal(t, "EMAIL", received.Channel)
	require.Equal(t, username, received.Variables["username"])
	require.Equal(t, "Chrome · macOS", received.Variables["device"])
	require.Equal(t, "192.0.2.1", received.Variables["ip"])
	require.Equal(t, "密码", received.Variables["auth_method"])
	require.Equal(t, "https://sso.example.com/profile/access/sessions", received.Variables["security_url"])
	var session model.UserSession
	require.NoError(t, database.First(&session, "id = ?", pair.SessionID).Error)
	require.Equal(t, session.CreatedAt.In(time.FixedZone("test", 8*3600)).Format("2006-01-02 15:04:05")+"（UTC+08:00）", received.Variables["login_time"])
	template, err := os.ReadFile("../../templates/email/new-device-login.html")
	require.NoError(t, err)
	names := regexp.MustCompile(`\{\{([a-z_]+)\}\}`).FindAllStringSubmatch(string(template), -1)
	require.Len(t, names, len(received.Variables))
	for _, name := range names {
		require.Contains(t, received.Variables, name[1])
	}
}

func Test_HandleLogin_SubmissionFailureIsSanitized(t *testing.T) {
	database := notificationDatabase(t)
	email := "private@example.com"
	require.NoError(t, database.Create(&model.User{ID: "u1", Email: &email}).Error)
	sender := &captureSender{err: errors.New("private@example.com secret-token response-body")}
	notifier := NewLoginNotifier(database, sender, "https://sso.example.com", nil)
	err := notifier.HandleLogin(t.Context(), event.LoginSucceeded{UserID: "u1", DeviceID: "dev_browser", NewDevice: true, OccurredAt: time.Now()})
	require.EqualError(t, err, "notification submission failed")
	require.Equal(t, 1, sender.calls)
}
