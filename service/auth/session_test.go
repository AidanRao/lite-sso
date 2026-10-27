package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"sso-server/dal/kv"
	"sso-server/model"
)

func Test_CompleteLoginWithContext_PersistencePolicyForEveryMethod(t *testing.T) {
	for _, method := range []AuthMethod{AuthMethodPassword, AuthMethodEmailOTP, AuthMethodQR, "GOOGLE"} {
		for _, remember := range []bool{false, true} {
			name := string(method) + "/temporary"
			if remember {
				name = string(method) + "/remembered"
			}
			t.Run(name, func(t *testing.T) {
				service, database, _ := newPasswordLoginTestServiceWithDB(t, "password123456")
				service.cfg.Tokens.Session.AccessTokenTTL = 15 * time.Minute
				service.cfg.Tokens.Session.RefreshTokenTTL = 30 * 24 * time.Hour
				service.cfg.Tokens.Session.TemporarySessionTTL = 12 * time.Hour
				_, pair, err := service.CompleteLoginWithContext(context.Background(), "u1", "/profile", LoginMetadata{DeviceID: "browser"}, method, LoginOptions{RememberMe: remember})
				require.NoError(t, err)
				var session model.UserSession
				require.NoError(t, database.First(&session, "id = ?", pair.SessionID).Error)
				expected := 12 * time.Hour
				if remember {
					expected = 30 * 24 * time.Hour
				}
				require.Equal(t, expected, session.ExpiresAt.Sub(session.CreatedAt))
				require.Equal(t, remember, session.RememberMe)
				require.Equal(t, remember, pair.RememberMe)
				require.True(t, pair.SessionExpiresAt.Equal(session.ExpiresAt))
				require.Equal(t, 900, pair.ExpiresIn)
			})
		}
	}
}

func Test_RefreshTokens_PreservesStoredPersistenceAndAbsoluteExpiry(t *testing.T) {
	for _, remember := range []bool{false, true} {
		name := "temporary"
		if remember {
			name = "remembered"
		}
		t.Run(name, func(t *testing.T) {
			service, database, _ := newPasswordLoginTestServiceWithDB(t, "password123456")
			service.cfg.Tokens.Session.AccessTokenTTL = 15 * time.Minute
			service.cfg.Tokens.Session.RefreshTokenTTL = 30 * 24 * time.Hour
			_, pair, err := service.CompleteLoginWithContext(context.Background(), "u1", "", LoginMetadata{}, AuthMethodPassword, LoginOptions{RememberMe: remember})
			require.NoError(t, err)
			deadline := time.Now().Add(2 * time.Hour).Truncate(time.Second)
			require.NoError(t, database.Model(&model.UserSession{}).Where("id = ?", pair.SessionID).Update("expires_at", deadline).Error)
			// A configuration change must not upgrade or extend an already-created session.
			service.cfg.Tokens.Session.RefreshTokenTTL = 90 * 24 * time.Hour
			rotated, err := service.RefreshTokens(context.Background(), pair.RefreshToken)
			require.NoError(t, err)
			require.NotEqual(t, pair.RefreshToken, rotated.RefreshToken)
			require.Equal(t, pair.SessionID, rotated.SessionID)
			require.Equal(t, remember, rotated.RememberMe)
			require.True(t, deadline.Equal(rotated.SessionExpiresAt))
			var stored model.UserSession
			require.NoError(t, database.First(&stored, "id = ?", pair.SessionID).Error)
			require.True(t, deadline.Equal(stored.ExpiresAt))
			require.Equal(t, remember, stored.RememberMe)
			require.Equal(t, refreshHash(rotated.RefreshToken), stored.RefreshTokenHash)
			require.NoError(t, database.Model(&stored).Update("expires_at", time.Now().Add(-time.Second)).Error)
			_, err = service.RefreshTokens(context.Background(), rotated.RefreshToken)
			require.Error(t, err)
		})
	}
}

func Test_SessionLifetime_TemporarySessionRespectsConfiguredMaximum(t *testing.T) {
	service, _ := newPasswordLoginTestService(t, "password123456")
	service.cfg.Tokens.Session.TemporarySessionTTL = 12 * time.Hour
	service.cfg.Tokens.Session.RefreshTokenTTL = time.Hour
	require.Equal(t, time.Hour, service.sessionLifetime(LoginOptions{}))
}

func Test_CompleteQRCodeLogin_UsesSavedPersistenceChoice(t *testing.T) {
	for _, remember := range []bool{false, true} {
		name := "temporary"
		if remember {
			name = "remembered"
		}
		t.Run(name, func(t *testing.T) {
			service := qrTestService(t, kv.NewMemoryStore())
			browser := LoginMetadata{DeviceID: "browser"}
			challenge, err := service.GenerateQRCode(context.Background(), "", browser, LoginOptions{RememberMe: remember})
			require.NoError(t, err)
			require.NotContains(t, challenge.QRPayload, "remember_me")
			actor := QRActor{UserID: "u1", ClientID: "scanner"}
			_, err = service.ScanQRCode(context.Background(), challenge.Code, actor)
			require.NoError(t, err)
			require.NoError(t, service.ConfirmQRCode(context.Background(), challenge.Code, actor))
			confirmed, err := service.PollQRCode(context.Background(), challenge.Code, browser.DeviceID, challenge.BrowserToken)
			require.NoError(t, err)
			_, pair, err := service.CompleteQRCodeLogin(context.Background(), challenge.Code, confirmed.LoginTicket, challenge.BrowserToken, browser)
			require.NoError(t, err)
			require.Equal(t, remember, pair.RememberMe)
			var session model.UserSession
			require.NoError(t, service.db.First(&session, "id = ?", pair.SessionID).Error)
			require.Equal(t, remember, session.RememberMe)
		})
	}
}
