package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"sso-server/conf"
	"sso-server/dal/kv"
	apiauth "sso-server/handler/api/auth"
	"sso-server/model"
	serviceauth "sso-server/service/auth"
)

func TestLoginWithPassword_RequiresCaptchaAfterRepeatedFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database, err := gorm.Open(sqlite.Open("file:auth_password_login_risk_v2?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.AutoMigrate(&model.User{}); err != nil {
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
	h := apiauth.NewAuthHandler(apiauth.AuthDeps{Config: &conf.Config{}, DB: database, KV: kv.NewMemoryStore()})
	r := gin.New()
	r.POST("/api/auth/login/password", h.LoginWithPassword)
	for i := 0; i < 6; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login/password", strings.NewReader(`{"email":"u1@example.com","password":"wrong-password"}`))
		req.AddCookie(&http.Cookie{Name: serviceauth.DeviceCookieName, Value: "dev_risk"})
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		if i < 5 && w.Code != http.StatusBadRequest {
			t.Fatalf("attempt %d expected 400, got %d, body=%s", i+1, w.Code, w.Body.String())
		}
		if i == 5 {
			if w.Code != http.StatusTooManyRequests {
				t.Fatalf("attempt %d expected 429, got %d, body=%s", i+1, w.Code, w.Body.String())
			}
			var response struct{ Data struct{ Code string `json:"code"` } `json:"data"` }
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || response.Data.Code != "CAPTCHA_REQUIRED" {
				t.Fatalf("expected captcha requirement, body=%s err=%v", w.Body.String(), err)
			}
		}
	}
}
