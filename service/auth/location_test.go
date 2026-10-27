package auth

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"sso-server/dal/kv"
	"sso-server/model"
)

type locationLookup func(string) string

func (lookup locationLookup) Lookup(ip string) string { return lookup(ip) }

func Test_QRCode_LocationSnapshotsUseBrowserRequests(t *testing.T) {
	for _, backend := range []string{"memory", "redis"} {
		t.Run(backend, func(t *testing.T) {
			var store kv.Store = kv.NewMemoryStore()
			if backend == "redis" {
				server := miniredis.RunT(t)
				client := redis.NewClient(&redis.Options{Addr: server.Addr()})
				t.Cleanup(func() { _ = client.Close() })
				store = kv.NewRedisStore(client)
			}
			s := qrTestService(t, store)
			s.locationResolver = locationLookup(func(ip string) string {
				switch ip {
				case "1.1.1.1":
					return "中国 · 北京"
				default:
					t.Fatalf("unexpected location lookup for %q", ip)
					return ""
				}
			})
			ctx := context.Background()
			browser := LoginMetadata{DeviceID: "browser", IP: "1.1.1.1"}
			challenge, err := s.GenerateQRCode(ctx, "", browser, LoginOptions{})
			require.NoError(t, err)
			// Replacing the resolver verifies that scanning uses the saved snapshot.
			s.locationResolver = locationLookup(func(ip string) string {
				require.Equal(t, "8.8.8.8", ip)
				return "中国 · 上海"
			})
			actor := QRActor{UserID: "u1", ClientID: "scanner"}
			preview, err := s.ScanQRCode(ctx, challenge.Code, actor)
			require.NoError(t, err)
			require.Equal(t, "中国 · 北京", preview.Environment.Location)
			require.NoError(t, s.ConfirmQRCode(ctx, challenge.Code, actor))
			confirmed, err := s.PollQRCode(ctx, challenge.Code, browser.DeviceID, challenge.BrowserToken)
			require.NoError(t, err)
			browser.IP = "8.8.8.8"
			_, pair, err := s.CompleteQRCodeLogin(ctx, challenge.Code, confirmed.LoginTicket, challenge.BrowserToken, browser)
			require.NoError(t, err)
			var session model.UserSession
			require.NoError(t, s.db.First(&session, "id = ?", pair.SessionID).Error)
			require.Equal(t, "8.8.8.8", session.IP)
			require.Equal(t, "中国 · 上海", session.Location)
			s.locationResolver = locationLookup(func(string) string {
				t.Fatal("device listing must use the stored location")
				return ""
			})
			devices, err := s.ListLoginDevices(ctx, "u1", pair.SessionID)
			require.NoError(t, err)
			require.Len(t, devices, 1)
			require.Equal(t, session.Location, devices[0].Environment.Location)
		})
	}
}

func Test_CompleteLoginWithContext_LocationIsOptionalForEveryMethod(t *testing.T) {
	for _, method := range []AuthMethod{AuthMethodPassword, AuthMethodEmailOTP, AuthMethodQR, "GITHUB", "PASSKEY"} {
		t.Run(string(method), func(t *testing.T) {
			s := qrTestService(t, kv.NewMemoryStore())
			s.locationResolver = locationLookup(func(string) string { return "" })
			_, pair, err := s.CompleteLoginWithContext(context.Background(), "u1", "", LoginMetadata{DeviceID: "browser", IP: "192.168.1.2"}, method, LoginOptions{})
			require.NoError(t, err)
			var session model.UserSession
			require.NoError(t, s.db.First(&session, "id = ?", pair.SessionID).Error)
			require.Empty(t, session.Location)
		})
	}
}
