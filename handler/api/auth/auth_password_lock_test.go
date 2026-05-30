package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"sso-server/dal/kv"
	"sso-server/handler/api/auth"
	"sso-server/model"
)

func Test_LoginWithPassword_LocksAfterFailedAttempts(t *testing.T) {
	gin.SetMode(gin.TestMode)

	gormDB, err := gorm.Open(sqlite.Open("file:auth_password_login_lock?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := gormDB.AutoMigrate(&model.User{}, &model.OAuthClient{}, &model.UserThirdParty{}, &model.UserOAuthClient{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), 12)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	hashStr := string(hash)
	email := "u1@example.com"
	if err := gormDB.Create(&model.User{
		ID:           "u1",
		Email:        &email,
		PasswordHash: &hashStr,
		IsActive:     true,
	}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	h := auth.NewAuthHandler(auth.AuthDeps{
		DB: gormDB,
		KV: kv.NewMemoryStore(),
	})

	r := gin.New()
	r.POST("/api/auth/login/password", h.LoginWithPassword)

	for i := 1; i <= 5; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login/password", strings.NewReader(`{"email":"u1@example.com","password":"wrong-password"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		if i < 5 && w.Code != http.StatusBadRequest {
			t.Fatalf("attempt %d expected 400, got %d, body=%s", i, w.Code, w.Body.String())
		}
		if i == 5 && w.Code != http.StatusTooManyRequests {
			t.Fatalf("attempt %d expected 429, got %d, body=%s", i, w.Code, w.Body.String())
		}
		if i == 5 {
			var resp struct {
				Data struct {
					RetryAfterSeconds int `json:"retry_after_seconds"`
				} `json:"data"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if resp.Data.RetryAfterSeconds <= 1 {
				t.Fatalf("expected retry seconds, got %d", resp.Data.RetryAfterSeconds)
			}
			if w.Header().Get("Retry-After") == "" {
				t.Fatalf("expected Retry-After header")
			}
		}
	}
}
