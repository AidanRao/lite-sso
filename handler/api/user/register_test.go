package user_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"sso-server/conf"
	"sso-server/dal/kv"
	"sso-server/handler/api/user"
	"sso-server/model"
	serviceauth "sso-server/service/auth"
)

func TestUserRegister_CreatesUserAndReturnsTokens(t *testing.T) {
	database, err := gorm.Open(sqlite.Open("file:user_register_session_v2?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.AutoMigrate(&model.User{}, &model.UserEmail{}, &model.UserSession{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	store := kv.NewMemoryStore()
	cfg := &conf.Config{Dev: conf.DevConfig{FixedEmailOTP: "123456", SkipSendMessage: true}}
	challenge := seedChallenge(t, cfg, database, store, "u1@example.com", serviceauth.ChallengePurposeRegister, "dev_register")
	h := user.NewUserHandler(user.UserDeps{Config: cfg, DB: database, KV: store})
	r := gin.New()
	r.POST("/api/user/register", h.Register)
	body := `{"email":"u1@example.com","password":"password123456","username":"u1","challenge_id":"` + challenge + `","code":"123456"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: serviceauth.DeviceCookieName, Value: "dev_register"})
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var response struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || response.Data.AccessToken == "" {
		t.Fatalf("expected access token, body=%s err=%v", w.Body.String(), err)
	}
	foundRefresh := false
	foundSession := false
	for _, cookie := range w.Result().Cookies() {
		foundRefresh = foundRefresh || cookie.Name == serviceauth.RefreshTokenCookieName && cookie.Value != ""
		foundSession = foundSession || cookie.Name == serviceauth.SessionCookieName && cookie.Value != "" && cookie.Path == "/"
	}
	if !foundRefresh || !foundSession {
		t.Fatalf("expected refresh and session cookies, cookies=%#v", w.Result().Cookies())
	}
}

func TestUserResetPassword_UsesChallengeAndArgon2id(t *testing.T) {
	database, err := gorm.Open(sqlite.Open("file:user_reset_password_v2?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.AutoMigrate(&model.User{}, &model.UserEmail{}, &model.UserSession{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	oldHash, err := serviceauth.HashPassword("old-password-123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	email := "u1@example.com"
	if err := database.Create(&model.User{ID: "u1", Email: &email, PasswordHash: &oldHash, IsActive: true}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	store := kv.NewMemoryStore()
	cfg := &conf.Config{Dev: conf.DevConfig{FixedEmailOTP: "123456", SkipSendMessage: true}}
	challenge := seedChallenge(t, cfg, database, store, email, serviceauth.ChallengePurposePasswordReset, "dev_reset")
	h := user.NewUserHandler(user.UserDeps{Config: cfg, DB: database, KV: store})
	r := gin.New()
	r.POST("/api/user/password/reset", h.ResetPassword)
	body := `{"email":"u1@example.com","password":"new-password-123","challenge_id":"` + challenge + `","code":"123456"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/user/password/reset", strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: serviceauth.DeviceCookieName, Value: "dev_reset"})
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var updated model.User
	if err := database.First(&updated, "id = ?", "u1").Error; err != nil || updated.PasswordHash == nil {
		t.Fatalf("expected updated hash, err=%v", err)
	}
	matched, err := serviceauth.VerifyPassword("new-password-123", *updated.PasswordHash)
	if err != nil || !matched {
		t.Fatalf("expected argon2id password, matched=%t err=%v", matched, err)
	}
}

func TestUserChangePassword_UpdatesPasswordAndKeepsCurrentSession(t *testing.T) {
	database, err := gorm.Open(sqlite.Open("file:user_change_password?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.AutoMigrate(&model.User{}, &model.UserEmail{}, &model.UserSession{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	oldHash, err := serviceauth.HashPassword("old-password-123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	email := "u1@example.com"
	if err := database.Create(&model.User{ID: "u1", Email: &email, PasswordHash: &oldHash, IsActive: true}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	now := time.Now()
	if err := database.Create([]model.UserSession{
		{ID: "ses-current", UserID: "u1", DeviceID: "dev-current", AuthMethod: string(serviceauth.AuthMethodPassword), RefreshTokenHash: strings.Repeat("a", 64), IP: "192.0.2.1", UserAgent: "current", CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(time.Hour)},
		{ID: "ses-other", UserID: "u1", DeviceID: "dev-other", AuthMethod: string(serviceauth.AuthMethodPassword), RefreshTokenHash: strings.Repeat("b", 64), IP: "192.0.2.2", UserAgent: "other", CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(time.Hour)},
	}).Error; err != nil {
		t.Fatalf("create sessions: %v", err)
	}

	h := user.NewUserHandler(user.UserDeps{Config: &conf.Config{}, DB: database, KV: kv.NewMemoryStore()})
	r := gin.New()
	r.PUT("/api/user/password", func(c *gin.Context) {
		c.Set("user_id", "u1")
		c.Set("session_id", "ses-current")
	}, h.ChangePassword)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/user/password", strings.NewReader(`{"old_password":"old-password-123","new_password":"new-password-456"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var other model.UserSession
	if err := database.First(&other, "id = ?", "ses-other").Error; err != nil || other.RevokedAt == nil {
		t.Fatalf("expected other session revoked, session=%#v err=%v", other, err)
	}
}

func seedChallenge(t *testing.T, cfg *conf.Config, database *gorm.DB, store kv.Store, email string, purpose serviceauth.ChallengePurpose, deviceID string) string {
	t.Helper()
	_ = store.Set(context.Background(), kv.KeyCaptcha("seed"), "1234", time.Minute)
	service := serviceauth.NewAuthService(cfg, database, store, nil, nil)
	result, err := service.SendEmailOTP(context.Background(), email, "seed", "1234", serviceauth.OTPRequestContext{DeviceID: deviceID, IP: "192.0.2.1"}, purpose)
	if err != nil {
		t.Fatalf("seed challenge: %v", err)
	}
	return result.ChallengeID
}
