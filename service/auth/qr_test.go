package auth

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"sso-server/common"
	"sso-server/conf"
	"sso-server/dal/kv"
	"sso-server/model"
)

func qrTestService(t *testing.T, store kv.Store) *AuthService {
	t.Helper()
	database, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	connection, err := database.DB()
	require.NoError(t, err)
	connection.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = connection.Close() })
	require.NoError(t, database.AutoMigrate(&model.User{}, &model.UserDeletion{}, &model.UserEmail{}, &model.OAuthClient{}, &model.UserSession{}))
	for _, id := range []string{"u1", "u2"} {
		require.NoError(t, database.Create(&model.User{ID: id, Status: model.UserStatusActive}).Error)
	}
	for _, id := range []string{"scanner", "other"} {
		require.NoError(t, database.Create(&model.OAuthClient{ClientID: id, Name: id, Audiences: []string{QRLoginAudience}, AllowedScopes: []string{QRLoginScope}}).Error)
	}
	return NewAuthService(&conf.Config{Auth: conf.AuthConfig{JWTSecret: "qr-test-secret"}, Tokens: conf.TokenConfig{
		OAuth:   conf.OAuthTokenConfig{Issuer: "https://sso.test"},
		Session: conf.SessionTokenConfig{AccessTokenTTL: time.Minute, RefreshTokenTTL: time.Hour},
	}}, database, store, nil, nil, nil)
}

func Test_QRCode_StateMachineBothBackends(t *testing.T) {
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
			ctx := context.Background()
			metadata := LoginMetadata{DeviceID: "dev-browser", IP: "192.0.2.1", UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36 Edg/130.0.2849.68"}
			redirect := "/oauth/authorize?client_id=scanner&state=original&code_challenge=original&code_challenge_method=S256"
			challenge, err := s.GenerateQRCode(ctx, redirect, metadata)
			require.NoError(t, err)
			require.NotContains(t, challenge.QRPayload, challenge.BrowserToken)
			require.NotContains(t, challenge.QRPayload, "redirect")
			var payload map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(challenge.QRPayload), &payload))
			require.Equal(t, "lite-sso.qr-login", payload["type"])
			require.Equal(t, float64(1), payload["version"])
			initialTTL, err := store.TTL(ctx, kv.KeyQR(challenge.Code))
			require.NoError(t, err)
			actor := QRActor{UserID: "u1", ClientID: "scanner"}
			preview, err := s.ScanQRCode(ctx, challenge.Code, actor)
			require.NoError(t, err)
			require.Equal(t, "scanner", preview.TargetName)
			require.Equal(t, metadata.IP, preview.Environment.IP)
			require.Equal(t, "Edge · Windows", preview.Environment.DisplayName)
			rawPreview, err := json.Marshal(preview)
			require.NoError(t, err)
			require.NotContains(t, string(rawPreview), "user_agent")
			require.NotContains(t, string(rawPreview), "Mozilla/")
			require.Equal(t, challenge.ExpiresAt, preview.ExpiresAt)
			_, err = s.ScanQRCode(ctx, challenge.Code, actor)
			require.NoError(t, err)
			for _, intruder := range []QRActor{{UserID: "u2", ClientID: "scanner"}, {UserID: "u1", ClientID: "other"}} {
				_, err = s.ScanQRCode(ctx, challenge.Code, intruder)
				require.ErrorIs(t, err, common.ErrQRCodeInvalidUser)
				require.ErrorIs(t, s.ConfirmQRCode(ctx, challenge.Code, intruder), common.ErrQRCodeInvalidUser)
			}
			require.NoError(t, s.ConfirmQRCode(ctx, challenge.Code, actor))
			data, err := s.PollQRCode(ctx, challenge.Code, metadata.DeviceID, challenge.BrowserToken)
			require.NoError(t, err)
			ticket := data.LoginTicket
			require.NoError(t, s.ConfirmQRCode(ctx, challenge.Code, actor))
			data, err = s.PollQRCode(ctx, challenge.Code, metadata.DeviceID, challenge.BrowserToken)
			require.NoError(t, err)
			require.Equal(t, ticket, data.LoginTicket)
			ttl, err := store.TTL(ctx, kv.KeyQR(challenge.Code))
			require.NoError(t, err)
			require.LessOrEqual(t, ttl, initialTTL)
			for _, credential := range []struct{ device, token string }{
				{device: "dev-other", token: challenge.BrowserToken}, {device: metadata.DeviceID, token: "wrong"},
				{device: metadata.DeviceID}, {token: challenge.BrowserToken},
			} {
				_, err = s.PollQRCode(ctx, challenge.Code, credential.device, credential.token)
				require.ErrorIs(t, err, common.ErrQRCodeBrowserInvalid)
			}
			_, _, err = s.CompleteQRCodeLogin(ctx, challenge.Code, "wrong", challenge.BrowserToken, metadata)
			require.ErrorIs(t, err, common.ErrQRCodeInvalidTicket)
			var successes atomic.Int32
			var group sync.WaitGroup
			for range 8 {
				group.Go(func() {
					result, _, err := s.CompleteQRCodeLogin(ctx, challenge.Code, ticket, challenge.BrowserToken, metadata)
					if err == nil {
						successes.Add(1)
						require.Equal(t, emailSetupRedirect(redirect), result.RedirectURL)
					}
				})
			}
			group.Wait()
			require.Equal(t, int32(1), successes.Load())
			var count int64
			require.NoError(t, s.db.Model(&model.UserSession{}).Count(&count).Error)
			require.Equal(t, int64(1), count)
			data, err = s.PollQRCode(ctx, challenge.Code, metadata.DeviceID, challenge.BrowserToken)
			require.NoError(t, err)
			require.Equal(t, QRCodeStatusConsumed, data.Status)
			require.Empty(t, data.LoginTicket)
		})
	}
}

func Test_QRCode_CancelRejectRevocationAndExpiry(t *testing.T) {
	ctx := context.Background()
	s := qrTestService(t, kv.NewMemoryStore())
	actor := QRActor{UserID: "u1", ClientID: "scanner"}
	metadata := LoginMetadata{DeviceID: "dev-browser"}
	generate := func() *QRCodeChallenge {
		result, err := s.GenerateQRCode(ctx, "", metadata)
		require.NoError(t, err)
		return result
	}
	cancelled := generate()
	require.ErrorIs(t, s.CancelQRCode(ctx, cancelled.Code, "dev-other", cancelled.BrowserToken), common.ErrQRCodeBrowserInvalid)
	require.NoError(t, s.CancelQRCode(ctx, cancelled.Code, metadata.DeviceID, cancelled.BrowserToken))
	_, err := s.ScanQRCode(ctx, cancelled.Code, actor)
	require.Error(t, err)
	rejected := generate()
	_, err = s.ScanQRCode(ctx, rejected.Code, actor)
	require.NoError(t, err)
	require.NoError(t, s.RejectQRCode(ctx, rejected.Code, actor))
	require.NoError(t, s.RejectQRCode(ctx, rejected.Code, actor))
	require.ErrorIs(t, s.ConfirmQRCode(ctx, rejected.Code, actor), common.ErrQRCodeInvalidStatus)
	approved := generate()
	_, err = s.ScanQRCode(ctx, approved.Code, actor)
	require.NoError(t, err)
	require.NoError(t, s.ConfirmQRCode(ctx, approved.Code, actor))
	data, err := s.PollQRCode(ctx, approved.Code, metadata.DeviceID, approved.BrowserToken)
	require.NoError(t, err)
	require.NoError(t, s.db.Model(&model.OAuthClient{}).Where("client_id = ?", "scanner").Update("allowed_scopes", "[]").Error)
	require.ErrorIs(t, s.ConfirmQRCode(ctx, approved.Code, actor), common.ErrQRCodeClientForbidden)
	_, _, err = s.CompleteQRCodeLogin(ctx, approved.Code, data.LoginTicket, approved.BrowserToken, metadata)
	require.ErrorIs(t, err, common.ErrQRCodeClientForbidden)
	expired := generate()
	data, _, err = s.loadQRCode(ctx, expired.Code)
	require.NoError(t, err)
	data.ExpiresAt = time.Now().Add(-time.Second)
	raw, err := json.Marshal(data)
	require.NoError(t, err)
	require.NoError(t, s.kv.Set(ctx, kv.KeyQR(expired.Code), string(raw), time.Minute))
	_, err = s.PollQRCode(ctx, expired.Code, metadata.DeviceID, expired.BrowserToken)
	require.ErrorIs(t, err, common.ErrQRCodeExpired)
	require.ErrorIs(t, s.ValidateQRCodeActor(ctx, QRActor{UserID: "u1", ClientID: "absent"}), common.ErrQRCodeClientForbidden)
}

type failingQRStore struct{ kv.Store }

func (s failingQRStore) Get(ctx context.Context, key string) (string, error) {
	return "", errors.New("storage offline")
}

func Test_QRCode_StorageFailureIsNotExpiry(t *testing.T) {
	s := qrTestService(t, failingQRStore{Store: kv.NewMemoryStore()})
	_, err := s.PollQRCode(context.Background(), "request", "dev", "secret")
	require.Error(t, err)
	require.NotErrorIs(t, err, common.ErrQRCodeExpired)
}

func Test_QRCode_RedisCompareAndSetRejectsStaleSnapshots(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := kv.NewRedisStore(client)
	s := qrTestService(t, store)
	ctx := context.Background()
	challenge, err := s.GenerateQRCode(ctx, "", LoginMetadata{DeviceID: "dev-test"})
	require.NoError(t, err)
	raw, err := store.Get(ctx, kv.KeyQR(challenge.Code))
	require.NoError(t, err)
	script, err := qrTransitionScript.ReadFile("qr_transition.lua")
	require.NoError(t, err)
	result, err := store.Eval(ctx, string(script), []string{kv.KeyQR(challenge.Code)}, "stale-snapshot", raw)
	require.NoError(t, err)
	require.EqualValues(t, 0, result)
	result, err = store.Eval(ctx, string(script), []string{kv.KeyQR(challenge.Code)}, raw, raw)
	require.NoError(t, err)
	require.EqualValues(t, 1, result)
	server.FastForward(6 * time.Minute)
	result, err = store.Eval(ctx, string(script), []string{kv.KeyQR(challenge.Code)}, raw, raw)
	require.NoError(t, err)
	require.EqualValues(t, -1, result)
}
