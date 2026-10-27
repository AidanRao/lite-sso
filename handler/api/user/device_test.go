package user_test

import (
	"context"
	"encoding/json"
	"errors"
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
	"sso-server/dto"
	apiuser "sso-server/handler/api/user"
	serverhandler "sso-server/handler/server"
	"sso-server/model"
	serviceauth "sso-server/service/auth"
)

type deviceLocationResolver struct{}

func (deviceLocationResolver) Lookup(string) string { return "中国 · 北京" }

func TestLoginDevices_ListRevokeAndInvalidateTargetTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database, err := gorm.Open(sqlite.Open("file:user_devices_api?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.AutoMigrate(&model.User{}, &model.UserDeletion{}, &model.UserEmail{}, &model.UserSession{}); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	email := "user@example.com"
	if err := database.Create(&model.User{ID: "u1", Email: &email, Status: model.UserStatusActive}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	cfg := &conf.Config{Tokens: conf.TokenConfig{Session: conf.SessionTokenConfig{AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 30 * 24 * time.Hour}}, Auth: conf.AuthConfig{JWTSecret: "device-test-jwt-secret"}}
	authService := serviceauth.NewAuthService(cfg, database, kv.NewMemoryStore(), nil, nil, deviceLocationResolver{})
	currentResult, _, err := authService.CompleteLoginWithContext(context.Background(), "u1", "", serviceauth.LoginMetadata{
		DeviceID:  "dev-current",
		IP:        "203.0.113.1",
		UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
	}, serviceauth.AuthMethodPassword, serviceauth.LoginOptions{})
	if err != nil {
		t.Fatalf("create current session: %v", err)
	}
	targetResult, targetPair, err := authService.CompleteLoginWithContext(context.Background(), "u1", "", serviceauth.LoginMetadata{
		DeviceID:  "dev-target",
		IP:        "203.0.113.2",
		UserAgent: "target browser",
	}, serviceauth.AuthMethodEmailOTP, serviceauth.LoginOptions{})
	if err != nil {
		t.Fatalf("create target session: %v", err)
	}

	handler := apiuser.NewUserHandler(apiuser.UserDeps{Config: cfg, DB: database, KV: kv.NewMemoryStore()})
	requireAuth := serverhandler.RequireSessionAuth(authService)
	router := gin.New()
	router.GET("/api/user/devices", requireAuth, handler.GetLoginDevices)
	router.DELETE("/api/user/devices/:device_id", requireAuth, handler.RevokeLoginDevice)
	router.GET("/protected", requireAuth, func(c *gin.Context) { c.Status(http.StatusNoContent) })

	listResponse := performBearerRequest(router, http.MethodGet, "/api/user/devices", currentResult.AccessToken)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected device list 200, got %d: %s", listResponse.Code, listResponse.Body.String())
	}
	if body := listResponse.Body.String(); !stringsContainAll(body, `"device_id":"dev-current"`, `"current":true`, `"device_id":"dev-target"`) {
		t.Fatalf("unexpected device list: %s", body)
	}
	var payload struct {
		Data struct {
			Devices []dto.LoginDeviceResponse `json:"devices"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listResponse.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Data.Devices) != 2 || payload.Data.Devices[0].Environment.DisplayName != "Chrome · macOS" || payload.Data.Devices[0].Environment.IP != "203.0.113.1" {
		t.Fatalf("unexpected structured environment: %#v", payload)
	}
	if payload.Data.Devices[0].Environment.Location != "中国 · 北京" {
		t.Fatalf("expected stored location for this session, got %#v", payload)
	}
	if strings.Contains(listResponse.Body.String(), "user_agent") || strings.Contains(listResponse.Body.String(), "Mozilla/") {
		t.Fatalf("raw User-Agent exposed: %s", listResponse.Body.String())
	}

	revokeResponse := performBearerRequest(router, http.MethodDelete, "/api/user/devices/dev-target", currentResult.AccessToken)
	if revokeResponse.Code != http.StatusOK {
		t.Fatalf("expected revoke 200, got %d: %s", revokeResponse.Code, revokeResponse.Body.String())
	}

	targetResponse := performBearerRequest(router, http.MethodGet, "/protected", targetResult.AccessToken)
	if targetResponse.Code != http.StatusUnauthorized {
		t.Fatalf("expected revoked access token 401, got %d", targetResponse.Code)
	}
	if _, err := authService.RefreshTokens(context.Background(), targetPair.RefreshToken); !errors.Is(err, common.ErrRefreshTokenInvalid) {
		t.Fatalf("expected revoked refresh token rejection, got %v", err)
	}

	currentResponse := performBearerRequest(router, http.MethodDelete, "/api/user/devices/dev-current", currentResult.AccessToken)
	if currentResponse.Code != http.StatusBadRequest {
		t.Fatalf("expected current device 400, got %d: %s", currentResponse.Code, currentResponse.Body.String())
	}
	missingResponse := performBearerRequest(router, http.MethodDelete, "/api/user/devices/dev-missing", currentResult.AccessToken)
	if missingResponse.Code != http.StatusNotFound {
		t.Fatalf("expected missing device 404, got %d: %s", missingResponse.Code, missingResponse.Body.String())
	}
}

func performBearerRequest(router http.Handler, method string, target string, token string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func stringsContainAll(value string, expected ...string) bool {
	for _, item := range expected {
		if !strings.Contains(value, item) {
			return false
		}
	}
	return true
}
