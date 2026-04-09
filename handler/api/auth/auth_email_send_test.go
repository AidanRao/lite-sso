package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	gooauth2store "github.com/go-oauth2/oauth2/v4/store"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"sso-server/conf"
	"sso-server/dal/kv"
	"sso-server/handler/api/auth"
	"sso-server/handler/oauth2"
	"sso-server/model"
	"sso-server/util/mailer"
)

type testMailer struct {
	lastEmail    string
	lastSubject  string
	lastTextBody string
	lastHtmlBody string
}

func (m *testMailer) SendEmail(ctx context.Context, email string, subject string, textBody string, htmlBody string) error {
	m.lastEmail = email
	m.lastSubject = subject
	m.lastTextBody = textBody
	m.lastHtmlBody = htmlBody
	return nil
}

func TestAuthEmailSend_SetsOTPAndRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	kvStore := kv.NewMemoryStore()
	_ = kvStore.Set(context.Background(), kv.KeyCaptcha("cid"), "1234", time.Minute)

	m := &testMailer{}
	h := auth.NewAuthHandler(auth.AuthDeps{
		Config: &conf.Config{
			Dev: conf.DevConfig{
				EchoOTP: true,
			},
		},
		KV:     kvStore,
		Mailer: m,
	})

	r := gin.New()
	r.POST("/api/auth/email/send", h.SendEmailOTP)

	body := `{"email":"u1@example.com","captcha_id":"cid","captcha":"1234"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/email/send", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Code != 200 {
		t.Fatalf("expected code 200, got %d", resp.Code)
	}

	_, err := kvStore.Get(context.Background(), kv.KeyOTP("u1@example.com"))
	if err != nil {
		t.Fatalf("expected otp in store, got err %v", err)
	}

	_, err = kvStore.Get(context.Background(), kv.KeyRateLimitEmail("u1@example.com"))
	if err != nil {
		t.Fatalf("expected rate limit key set, got err %v", err)
	}
}

func TestAuthEmailSend_RateLimited(t *testing.T) {
	gin.SetMode(gin.TestMode)

	kvStore := kv.NewMemoryStore()
	_ = kvStore.Set(context.Background(), kv.KeyCaptcha("cid"), "1234", time.Minute)
	_, _ = kvStore.SetNX(context.Background(), kv.KeyRateLimitEmail("u1@example.com"), "1", time.Minute)

	h := auth.NewAuthHandler(auth.AuthDeps{
		Config: &conf.Config{},
		KV:     kvStore,
		Mailer: &testMailer{},
	})

	r := gin.New()
	r.POST("/api/auth/email/send", h.SendEmailOTP)

	body := `{"email":"u1@example.com","captcha_id":"cid","captcha":"1234"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/email/send", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d, body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Code != 429 {
		t.Fatalf("expected code 429, got %d", resp.Code)
	}
}

func TestAuthPasswordLogin_SetsSessionCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file:auth_password_login_session?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.OAuthClient{}, &model.UserThirdParty{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), 12)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	hashStr := string(hash)

	if err := db.Create(&model.User{
		ID:           "u1",
		Email:        "u1@example.com",
		PasswordHash: &hashStr,
		IsActive:     true,
	}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	tokenStore, err := gooauth2store.NewMemoryTokenStore()
	if err != nil {
		t.Fatalf("token store: %v", err)
	}

	cfg := &conf.Config{}
	cfg.Security.AccessTokenExpire = time.Hour

	o, err := oauth2.NewWithStores(cfg, db, tokenStore)
	if err != nil {
		t.Fatalf("new oauth2: %v", err)
	}

	h := auth.NewAuthHandler(auth.AuthDeps{
		Config: cfg,
		DB:     db,
		KV:     kv.NewMemoryStore(),
		OAuth2: o,
	})

	r := gin.New()
	r.POST("/api/auth/login/password", h.LoginWithPassword)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login/password", strings.NewReader(`{"email":"u1@example.com","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Code != 200 {
		t.Fatalf("expected code 200, got %d", resp.Code)
	}

	found := false
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "sso_session" && cookie.Value != "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected sso_session cookie, got %#v", w.Result().Cookies())
	}
}

func TestAuthEmailLogin_SetsSessionCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file:auth_email_login_session?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.OAuthClient{}, &model.UserThirdParty{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if err := db.Create(&model.User{
		ID:       "u1",
		Email:    "u1@example.com",
		IsActive: true,
	}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	tokenStore, err := gooauth2store.NewMemoryTokenStore()
	if err != nil {
		t.Fatalf("token store: %v", err)
	}

	cfg := &conf.Config{}
	cfg.Security.AccessTokenExpire = time.Hour

	o, err := oauth2.NewWithStores(cfg, db, tokenStore)
	if err != nil {
		t.Fatalf("new oauth2: %v", err)
	}

	kvStore := kv.NewMemoryStore()
	if err := kvStore.Set(context.Background(), kv.KeyOTP("u1@example.com"), "123456", time.Minute); err != nil {
		t.Fatalf("seed otp: %v", err)
	}

	h := auth.NewAuthHandler(auth.AuthDeps{
		Config: cfg,
		DB:     db,
		KV:     kvStore,
		OAuth2: o,
	})

	r := gin.New()
	r.POST("/api/auth/login/email", h.LoginWithEmailOTP)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login/email", strings.NewReader(`{"email":"u1@example.com","otp":"123456"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}

	found := false
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "sso_session" && cookie.Value != "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected sso_session cookie, got %#v", w.Result().Cookies())
	}
}

var _ mailer.Mailer = (*testMailer)(nil)
