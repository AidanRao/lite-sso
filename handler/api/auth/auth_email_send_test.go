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

	"sso-server/conf"
	"sso-server/dal/kv"
	"sso-server/handler/api/auth"
	"sso-server/util/mailer"
)

type testMailer struct {
	lastEmail string
	lastOTP   string
}

func (m *testMailer) SendOTP(ctx context.Context, email string, otp string) error {
	m.lastEmail = email
	m.lastOTP = otp
	return nil
}

func TestAuthEmailSend_SetsOTPAndRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	kvStore := kv.NewMemoryStore()
	_ = kvStore.Set(context.Background(), kv.KeyCaptcha("cid"), "1234", time.Minute)

	m := &testMailer{}
	h := auth.NewAuthHandler(auth.AuthDeps{
		Config: &conf.Config{},
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

	if m.lastEmail != "u1@example.com" {
		t.Fatalf("expected mailer called with email, got %q", m.lastEmail)
	}
	if len(m.lastOTP) != 6 {
		t.Fatalf("expected 6-digit otp, got %q", m.lastOTP)
	}

	val, err := kvStore.Get(context.Background(), kv.KeyOTP("u1@example.com"))
	if err != nil {
		t.Fatalf("expected otp in store, got err %v", err)
	}
	if val != m.lastOTP {
		t.Fatalf("expected stored otp equals sent otp, got %q vs %q", val, m.lastOTP)
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

var _ mailer.Mailer = (*testMailer)(nil)
