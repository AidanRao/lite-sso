package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/conf"
	"sso-server/model"
)

func TestListLoginDevices_GroupsAndFiltersActiveSessions(t *testing.T) {
	database := newDeviceSessionTestDB(t)
	now := time.Now()
	revokedAt := now.Add(-time.Minute)

	sessions := []model.UserSession{
		newDeviceSession("ses-other", "u1", "dev-other", now.Add(-time.Minute), now.Add(time.Hour)),
		newDeviceSession("ses-current-newer", "u1", "dev-current", now.Add(-5*time.Minute), now.Add(time.Hour)),
		newDeviceSession("ses-current", "u1", "dev-current", now.Add(-30*time.Minute), now.Add(time.Hour)),
		newDeviceSession("ses-expired", "u1", "dev-expired", now.Add(-time.Hour), now.Add(-time.Minute)),
		newDeviceSession("ses-foreign", "u2", "dev-foreign", now, now.Add(time.Hour)),
		newDeviceSession("ses-revoked", "u1", "dev-revoked", now, now.Add(time.Hour)),
	}
	sessions[1].AuthMethod = string(AuthMethodGitHub)
	sessions[5].RevokedAt = &revokedAt
	if err := database.Create(&sessions).Error; err != nil {
		t.Fatalf("seed sessions: %v", err)
	}

	service := NewAuthService(&conf.Config{}, database, nil, nil, nil)
	devices, err := service.ListLoginDevices(context.Background(), "u1", "ses-current")
	if err != nil {
		t.Fatalf("list devices: %v", err)
	}
	if len(devices) != 2 {
		t.Fatalf("expected two active devices, got %#v", devices)
	}
	if devices[0].DeviceID != "dev-current" || !devices[0].Current {
		t.Fatalf("expected current device first, got %#v", devices)
	}
	if devices[0].AuthMethod != string(AuthMethodGitHub) {
		t.Fatalf("expected latest session metadata, got %#v", devices[0])
	}
	if devices[1].DeviceID != "dev-other" || devices[1].Current {
		t.Fatalf("expected other device second, got %#v", devices)
	}
	if _, err := service.ListLoginDevices(context.Background(), "u1", "ses-foreign"); !errors.Is(err, common.ErrSessionRevoked) {
		t.Fatalf("expected foreign current session rejection, got %v", err)
	}
}

func TestRevokeLoginDevice_RevokesOnlyOtherActiveDeviceSessions(t *testing.T) {
	database := newDeviceSessionTestDB(t)
	now := time.Now()
	sessions := []model.UserSession{
		newDeviceSession("ses-current", "u1", "dev-current", now, now.Add(time.Hour)),
		newDeviceSession("ses-target-1", "u1", "dev-target", now, now.Add(time.Hour)),
		newDeviceSession("ses-target-2", "u1", "dev-target", now.Add(-time.Minute), now.Add(time.Hour)),
		newDeviceSession("ses-target-expired", "u1", "dev-target", now.Add(-time.Hour), now.Add(-time.Minute)),
		newDeviceSession("ses-foreign", "u2", "dev-target", now, now.Add(time.Hour)),
	}
	if err := database.Create(&sessions).Error; err != nil {
		t.Fatalf("seed sessions: %v", err)
	}

	service := NewAuthService(&conf.Config{}, database, nil, nil, nil)
	if err := service.RevokeLoginDevice(context.Background(), "u1", "ses-current", "dev-target"); err != nil {
		t.Fatalf("revoke target device: %v", err)
	}

	var stored []model.UserSession
	if err := database.Order("id").Find(&stored).Error; err != nil {
		t.Fatalf("load sessions: %v", err)
	}
	for _, session := range stored {
		switch session.ID {
		case "ses-target-1", "ses-target-2":
			if session.RevokedAt == nil || session.RevokeReason == nil || *session.RevokeReason != deviceRevokeReason {
				t.Fatalf("expected %s revoked for device reason, got %#v", session.ID, session)
			}
		default:
			if session.RevokedAt != nil {
				t.Fatalf("expected %s preserved, got %#v", session.ID, session)
			}
		}
	}

	if err := service.RevokeLoginDevice(context.Background(), "u1", "ses-current", "dev-target"); !errors.Is(err, common.ErrDeviceNotFound) {
		t.Fatalf("expected repeated revoke to return ErrDeviceNotFound, got %v", err)
	}
	if err := service.RevokeLoginDevice(context.Background(), "u1", "ses-current", "dev-current"); !errors.Is(err, common.ErrCurrentDevice) {
		t.Fatalf("expected current device rejection, got %v", err)
	}
	if err := service.RevokeLoginDevice(context.Background(), "u1", "ses-current", "dev-foreign"); !errors.Is(err, common.ErrDeviceNotFound) {
		t.Fatalf("expected foreign device to be hidden, got %v", err)
	}
}

func newDeviceSessionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	databaseName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	database, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", databaseName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.AutoMigrate(&model.UserSession{}); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	return database
}

func newDeviceSession(id string, userID string, deviceID string, lastSeenAt time.Time, expiresAt time.Time) model.UserSession {
	return model.UserSession{
		ID:               id,
		UserID:           userID,
		DeviceID:         deviceID,
		AuthMethod:       string(AuthMethodPassword),
		RefreshTokenHash: strings.Repeat("a", 64),
		IP:               "203.0.113.10",
		UserAgent:        "Mozilla/5.0",
		CreatedAt:        lastSeenAt.Add(-time.Minute),
		LastSeenAt:       lastSeenAt,
		ExpiresAt:        expiresAt,
	}
}
