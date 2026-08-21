package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"sso-server/common"
	"sso-server/conf"
	"sso-server/dal/kv"
	apiauth "sso-server/handler/api/auth"
	"sso-server/model"
	serviceauth "sso-server/service/auth"
)

type testMessageSender struct {
	lastTarget      string
	lastTemplateKey string
	lastVariables   map[string]string
	sendCount       int
	err             error
}

func (m *testMessageSender) Send(ctx context.Context, target string, templateKey string, variables map[string]string) error {
	m.sendCount++
	m.lastTarget = target
	m.lastTemplateKey = templateKey
	m.lastVariables = variables
	return m.err
}

func TestAuthEmailSend_CreatesHMACChallenge(t *testing.T) {
	t.Setenv("ENV", "local")
	gin.SetMode(gin.TestMode)
	store := kv.NewMemoryStore()
	_ = store.Set(context.Background(), kv.KeyCaptcha("cid"), "1234", time.Minute)
	sender := &testMessageSender{}
	cfg := &conf.Config{Dev: conf.DevConfig{FixedEmailOTP: "123456"}}
	h := apiauth.NewAuthHandler(apiauth.AuthDeps{Config: cfg, KV: store, MessageSender: sender})

	r := gin.New()
	r.POST("/api/auth/email/send", h.SendEmailOTP)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/email/send", strings.NewReader(`{"email":"u1@example.com","captcha_id":"cid","captcha":"1234","purpose":"LOGIN"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var response struct {
		Data struct {
			ChallengeID string `json:"challenge_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if response.Data.ChallengeID == "" || sender.lastVariables["code"] != "123456" {
		t.Fatalf("expected challenge and sent code, response=%s sender=%#v", w.Body.String(), sender)
	}
	raw, err := store.Get(context.Background(), kv.KeyChallenge(response.Data.ChallengeID))
	if err != nil || strings.Contains(raw, "123456") {
		t.Fatalf("expected challenge without plaintext otp, raw=%q err=%v", raw, err)
	}
}

func TestAuthEmailSend_LocalFixedOTP_VerifiesChallenge(t *testing.T) {
	t.Setenv("ENV", "local")
	store := kv.NewMemoryStore()
	_ = store.Set(context.Background(), kv.KeyCaptcha("cid"), "1234", time.Minute)
	cfg := &conf.Config{Dev: conf.DevConfig{FixedEmailOTP: "654321", SkipSendMessage: true}}
	h := apiauth.NewAuthHandler(apiauth.AuthDeps{Config: cfg, KV: store})
	r := gin.New()
	r.POST("/api/auth/email/send", h.SendEmailOTP)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/email/send", strings.NewReader(`{"email":"u1@example.com","captcha_id":"cid","captcha":"1234","purpose":"LOGIN"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var response struct{ Data struct{ ChallengeID string `json:"challenge_id"` } `json:"data"` }
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	deviceCookie := w.Result().Cookies()[0].Value
	service := serviceauth.NewAuthService(cfg, nil, store, nil, nil)
	email, err := service.VerifyChallengeForPurpose(context.Background(), response.Data.ChallengeID, "654321", deviceCookie, serviceauth.ChallengePurposeLogin)
	if err != nil || email != "u1@example.com" {
		t.Fatalf("expected challenge verification, email=%q err=%v", email, err)
	}
}

func TestAuthEmailSend_RateLimited(t *testing.T) {
	store := kv.NewMemoryStore()
	cfg := &conf.Config{Dev: conf.DevConfig{FixedEmailOTP: "123456", SkipSendMessage: true}}
	h := apiauth.NewAuthHandler(apiauth.AuthDeps{Config: cfg, KV: store})
	r := gin.New()
	r.POST("/api/auth/email/send", h.SendEmailOTP)
	for i := 0; i < 2; i++ {
		captchaID := "cid" + string(rune('0'+i))
		_ = store.Set(context.Background(), kv.KeyCaptcha(captchaID), "1234", time.Minute)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/auth/email/send", strings.NewReader(`{"email":"u1@example.com","captcha_id":"`+captchaID+`","captcha":"1234","purpose":"LOGIN"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		if i == 0 && w.Code != http.StatusOK {
			t.Fatalf("first send expected 200, got %d", w.Code)
		}
		if i == 1 && w.Code != http.StatusTooManyRequests {
			t.Fatalf("second send expected 429, got %d, body=%s", w.Code, w.Body.String())
		}
	}
}

func TestAuthEmailSend_MessageSenderFailure_LogsSanitizedError(t *testing.T) {
	t.Setenv("ENV", "local")
	gin.SetMode(gin.TestMode)
	store := kv.NewMemoryStore()
	_ = store.Set(context.Background(), kv.KeyCaptcha("cid"), "1234", time.Minute)
	sender := &testMessageSender{err: errors.New("message center returned status 502")}
	cfg := &conf.Config{Dev: conf.DevConfig{FixedEmailOTP: "123456"}}
	h := apiauth.NewAuthHandler(apiauth.AuthDeps{Config: cfg, KV: store, MessageSender: sender})
	r := gin.New()
	r.POST("/api/auth/email/send", h.SendEmailOTP)

	var logs bytes.Buffer
	originalOutput := log.Writer()
	originalFlags := log.Flags()
	log.SetOutput(&logs)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(originalOutput)
		log.SetFlags(originalFlags)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/email/send", strings.NewReader(`{"email":"u1@example.com","captcha_id":"cid","captcha":"1234","purpose":"LOGIN"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d, body=%s", w.Code, w.Body.String())
	}
	output := logs.String()
	if !strings.Contains(output, "auth email OTP send failed: stage=send_otp purpose=LOGIN ip=192.0.2.1 error=message center returned status 502") {
		t.Fatalf("expected diagnostic log, got %q", output)
	}
	if strings.Contains(output, "u1@example.com") || strings.Contains(output, "123456") {
		t.Fatalf("expected log without email or OTP, got %q", output)
	}
}

func TestAuthPasswordLogin_ReturnsAccessAndRefreshTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database, err := gorm.Open(sqlite.Open("file:auth_password_login_session_v2?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.AutoMigrate(&model.User{}, &model.UserSession{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	hash, err := serviceauth.HashPassword("password123456")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	email := "u1@example.com"
	if err := database.Create(&model.User{ID: "u1", Email: &email, PasswordHash: &hash, IsActive: true}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	cfg := &conf.Config{}
	h := apiauth.NewAuthHandler(apiauth.AuthDeps{Config: cfg, DB: database, KV: kv.NewMemoryStore()})
	r := gin.New()
	r.POST("/api/auth/login/password", h.LoginWithPassword)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login/password", strings.NewReader(`{"email":"u1@example.com","password":"password123456"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var response struct{ Data struct{ AccessToken string `json:"access_token"` } `json:"data"` }
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || response.Data.AccessToken == "" {
		t.Fatalf("expected access token, body=%s err=%v", w.Body.String(), err)
	}
	foundRefresh := false
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == serviceauth.RefreshTokenCookieName && cookie.Value != "" && cookie.HttpOnly {
			foundRefresh = true
		}
	}
	if !foundRefresh {
		t.Fatalf("expected refresh cookie, cookies=%#v", w.Result().Cookies())
	}
}

func TestAuthEmailLogin_ConsumesChallengeAndCreatesSession(t *testing.T) {
	database, err := gorm.Open(sqlite.Open("file:auth_email_login_session_v2?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.AutoMigrate(&model.User{}, &model.UserSession{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	email := "u1@example.com"
	if err := database.Create(&model.User{ID: "u1", Email: &email, IsActive: true}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	store := kv.NewMemoryStore()
	cfg := &conf.Config{Dev: conf.DevConfig{FixedEmailOTP: "123456", SkipSendMessage: true}}
	_ = store.Set(context.Background(), kv.KeyCaptcha("send"), "1234", time.Minute)
	service := serviceauth.NewAuthService(cfg, database, store, nil, nil)
	challenge, err := service.SendEmailOTP(context.Background(), email, "send", "1234", serviceauth.OTPRequestContext{DeviceID: "dev_test", IP: "192.0.2.1"}, serviceauth.ChallengePurposeLogin)
	if err != nil {
		t.Fatalf("create challenge: %v", err)
	}
	h := apiauth.NewAuthHandler(apiauth.AuthDeps{Config: cfg, DB: database, KV: store})
	r := gin.New()
	r.POST("/api/auth/login/email", h.LoginWithEmailOTP)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login/email", strings.NewReader(`{"challenge_id":"`+challenge.ChallengeID+`","code":"123456"}`))
	req.AddCookie(&http.Cookie{Name: serviceauth.DeviceCookieName, Value: "dev_test"})
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
}

var _ serviceauth.MessageSender = (*testMessageSender)(nil)
var _ = common.ErrInvalidCredentials
