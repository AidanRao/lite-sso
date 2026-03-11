package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"sso-server/dal/kv"
	"sso-server/handler/api/auth"
)

func TestAuthCaptcha_ReturnsCaptchaIDAndImage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	h := auth.NewAuthHandler(auth.AuthDeps{KV: kv.NewMemoryStore()})
	r.GET("/api/auth/captcha", h.GenerateCaptcha)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/captcha", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			CaptchaID        string `json:"captcha_id"`
			CaptchaPNGBase64 string `json:"captcha_png_base64"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Code != 200 {
		t.Fatalf("expected code 200, got %d", resp.Code)
	}
	if resp.Data.CaptchaID == "" {
		t.Fatalf("expected captcha_id, got %s", w.Body.String())
	}
	if resp.Data.CaptchaPNGBase64 == "" {
		t.Fatalf("expected captcha_png_base64, got %s", w.Body.String())
	}
}
