package audit_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"sso-server/conf"
	"sso-server/dal/kv"
	apiauth "sso-server/handler/api/auth"
	"sso-server/handler/audit"
	"sso-server/handler/server"
	"sso-server/model"
	"sso-server/service/auth"
)

type capture struct{ records []model.AuditLog }

func (s *capture) TryRecord(event model.AuditLog) bool {
	s.records = append(s.records, event)
	return true
}

func router(cfg *conf.Config) (*gin.Engine, *capture) {
	gin.SetMode(gin.TestMode)
	sink := &capture{}
	r := gin.New()
	r.Use(audit.Middleware(sink, cfg, func(request *http.Request) (string, string) {
		device, _ := auth.DeviceIDFromRequest(request)
		return auth.RequestIP(request, cfg.Server.TrustProxyHeaders), device
	}))
	r.Use(gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, _ any) { audit.Failure(c, "PANIC"); c.AbortWithStatus(500) }))
	return r, sink
}

func request(r *gin.Engine, method, path, body, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X)")
	req.Header.Set("X-User-ID", "forged-user")
	req.AddCookie(&http.Cookie{Name: "device_id", Value: "dev_audit"})
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func responseData(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	require.Equal(t, 200, w.Code, w.Body.String())
	var response struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	return response.Data
}

func Test_Middleware_AllRegisteredOperationsCaptureEarlyRejection(t *testing.T) {
	seen := make(map[string]bool)
	for _, rule := range audit.Rules() {
		t.Run(rule.Action, func(t *testing.T) {
			key := rule.Method + " " + rule.Route
			require.False(t, seen[key])
			seen[key] = true
			r, sink := router(&conf.Config{})
			r.Handle(rule.Method, rule.Route, func(c *gin.Context) { c.AbortWithStatus(401) })
			path := strings.NewReplacer(":binding_id", "private-binding-token", ":device_id", "dev_target", ":provider", "github", ":id", "123").Replace(rule.Route)
			request(r, rule.Method, path+"?code=private-code&state=private-state", `{"password":"private-password"}`, "private-token")
			require.Len(t, sink.records, 1)
			record := sink.records[0]
			require.Equal(t, "denied", record.Outcome)
			require.Nil(t, record.UserID)
			require.Equal(t, rule.Route, record.Route)
			encoded, err := json.Marshal(record)
			require.NoError(t, err)
			for _, secret := range []string{"private-binding-token", "private-code", "private-state", "private-password", "private-token", "forged-user"} {
				require.NotContains(t, string(encoded), secret)
			}
		})
	}
}

func Test_Middleware_ExcludesReadPollingAndTokenEndpoints(t *testing.T) {
	r, sink := router(&conf.Config{})
	for _, path := range []string{"/healthz", "/api/user/profile", "/api/auth/qr/poll", "/api/auth/captcha"} {
		r.GET(path, func(c *gin.Context) { c.Status(200) })
		request(r, "GET", path, "", "")
	}
	for _, path := range []string{"/oauth/token", "/api/auth/token/refresh"} {
		r.POST(path, func(c *gin.Context) { c.Status(200) })
		request(r, "POST", path, "", "")
	}
	require.Empty(t, sink.records)
}

func Test_Middleware_RedirectsPanicAndPartialCompletion(t *testing.T) {
	for _, test := range []struct {
		name, outcome, reason string
		run                   gin.HandlerFunc
	}{
		{name: "unknown redirect", outcome: "failure", reason: "UNCLASSIFIED_REDIRECT", run: func(c *gin.Context) { c.Redirect(302, "/login") }},
		{name: "denied redirect", outcome: "denied", reason: "AUTHENTICATION_REQUIRED", run: func(c *gin.Context) { audit.Denied(c, "AUTHENTICATION_REQUIRED"); c.Redirect(302, "/login") }},
		{name: "successful redirect", outcome: "success", reason: "OK", run: func(c *gin.Context) { audit.Success(c); c.Redirect(302, "/callback?code=secret") }},
		{name: "panic", outcome: "failure", reason: "PANIC", run: func(c *gin.Context) { panic("secret panic") }},
		{name: "partial", outcome: "failure", reason: "EMAIL_DELIVERY_FAILED", run: func(c *gin.Context) {
			audit.Completed(c, "email_created")
			audit.Failure(c, "EMAIL_DELIVERY_FAILED")
			c.Status(502)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			r, sink := router(&conf.Config{})
			r.GET("/oauth/authorize", test.run)
			request(r, "GET", "/oauth/authorize", "", "")
			require.Len(t, sink.records, 1)
			require.Equal(t, test.outcome, sink.records[0].Outcome)
			require.Equal(t, test.reason, sink.records[0].ReasonCode)
			if test.name == "partial" {
				require.Contains(t, sink.records[0].Details, "email_created")
			}
		})
	}
}

func Test_Middleware_RealPasswordEmailQRCodeAndLogoutFlows(t *testing.T) {
	t.Setenv("ENV", "local")
	database, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := database.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, database.AutoMigrate(&model.User{}, &model.UserEmail{}, &model.UserSession{}, &model.OAuthClient{}, &model.UserOAuthClient{}))
	email := "private@example.com"
	hash, err := auth.HashPassword("Password12345")
	require.NoError(t, err)
	require.NoError(t, database.Create(&model.User{ID: "u1", Email: &email, PasswordHash: &hash, IsActive: true}).Error)
	store := kv.NewMemoryStore()
	cfg := &conf.Config{Dev: conf.DevConfig{FixedEmailOTP: "123456", SkipSendMessage: true}}
	h := apiauth.NewAuthHandler(apiauth.AuthDeps{Config: cfg, DB: database, KV: store})
	r, sink := router(cfg)
	r.POST("/api/auth/login/password", h.LoginWithPassword)
	r.POST("/api/auth/login/email", h.LoginWithEmailOTP)
	r.POST("/api/auth/email/send", h.SendEmailOTP)
	r.GET("/api/auth/qr/generate", h.GenerateQRCode)
	r.GET("/api/auth/qr/poll", h.PollQRCode)
	r.POST("/api/auth/qr/scan", server.RequireSessionAuth(h.Service()), h.ScanQRCode)
	r.POST("/api/auth/qr/confirm", server.RequireSessionAuth(h.Service()), h.ConfirmQRCode)
	r.POST("/api/auth/qr/complete", h.CompleteQRCode)
	r.POST("/api/auth/logout", h.Logout)
	failed := request(r, "POST", "/api/auth/login/password", `{"email":"private@example.com","password":"incorrect"}`, "forged")
	require.Equal(t, 400, failed.Code)
	require.Nil(t, sink.records[0].UserID)
	require.Equal(t, "INVALID_CREDENTIALS", sink.records[0].ReasonCode)
	login := responseData(t, request(r, "POST", "/api/auth/login/password", `{"email":"private@example.com","password":"Password12345"}`, ""))
	token := login["access_token"].(string)
	require.Equal(t, "u1", *sink.records[1].UserID)
	require.Len(t, sink.records[1].SessionHash, 64)
	require.NoError(t, store.Set(context.Background(), kv.KeyCaptcha("audit-captcha"), "1234", time.Minute))
	challenge := responseData(t, request(r, "POST", "/api/auth/email/send", `{"email":"private@example.com","captcha_id":"audit-captcha","captcha":"1234","purpose":"LOGIN"}`, ""))["challenge_id"].(string)
	responseData(t, request(r, "POST", "/api/auth/login/email", `{"challenge_id":"`+challenge+`","code":"123456"}`, ""))
	qr := responseData(t, request(r, "GET", "/api/auth/qr/generate", "", ""))["code"].(string)
	responseData(t, request(r, "POST", "/api/auth/qr/scan", `{"code":"`+qr+`"}`, token))
	responseData(t, request(r, "POST", "/api/auth/qr/confirm", `{"code":"`+qr+`"}`, token))
	ticket := responseData(t, request(r, "GET", "/api/auth/qr/poll?code="+qr, "", ""))["login_ticket"].(string)
	responseData(t, request(r, "POST", "/api/auth/qr/complete", `{"code":"`+qr+`","login_ticket":"`+ticket+`"}`, ""))
	responseData(t, request(r, "POST", "/api/auth/logout", "", token))
	require.Len(t, sink.records, 8)
	for _, record := range sink.records[3:] {
		require.Equal(t, "u1", *record.UserID)
		require.Equal(t, "success", record.Outcome)
	}
	encoded, err := json.Marshal(sink.records)
	require.NoError(t, err)
	for _, secret := range []string{email, "Password12345", "123456", challenge, qr, ticket, token, "forged-user"} {
		require.NotContains(t, string(encoded), secret)
	}
}
