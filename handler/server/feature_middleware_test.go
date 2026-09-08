package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"sso-server/conf"
	"sso-server/dto"
	"sso-server/handler/api/admin"
	"sso-server/handler/api/user"
	"sso-server/handler/audit"
	"sso-server/model"
	"sso-server/service/feature"
)

type featureAuditCapture struct{ events []model.AuditLog }

func (s *featureAuditCapture) TryRecord(event model.AuditLog) bool {
	s.events = append(s.events, event)
	return true
}

func Test_RequireFeature_AdminPoliciesAndProfileAgreement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	conn, err := database.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	require.NoError(t, database.AutoMigrate(&model.User{}, &model.UserEmail{}, &model.FeatureConfig{}, &model.FeatureUser{}))
	require.NoError(t, database.Create(&model.User{ID: "admin"}).Error)
	require.NoError(t, database.Create(&model.User{ID: "ordinary"}).Error)
	cfg := &conf.Config{Admin: conf.AdminConfig{UserIDs: []string{"admin"}}}
	service := feature.NewService(database)
	handler := admin.NewAdminHandler(admin.AdminDeps{Config: cfg, DB: database})
	profile := user.NewUserHandler(user.UserDeps{Config: cfg, DB: database})
	router := gin.New()
	sink := &featureAuditCapture{}
	router.Use(audit.Middleware(sink, cfg, func(_ *http.Request) (string, string) { return "", "" }))
	// Supply the identity that production session authentication establishes.
	router.Use(func(c *gin.Context) {
		if id := c.GetHeader("X-Test-User"); id != "" {
			c.Set("user_id", id)
		}
		c.Next()
	})
	group := router.Group("/api/admin", RequireAdmin(cfg))
	group.GET("/features", handler.ListFeatures)
	group.PUT("/features/:key", handler.UpdateFeature)
	router.GET("/api/user/profile", profile.GetProfile)
	router.GET("/api/user/audit-logs", RequireFeature(service, feature.AuditLogs), func(c *gin.Context) { c.Status(http.StatusOK) })
	request := func(method, path, actor, body string) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("X-Test-User", actor)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		return w
	}
	require.Equal(t, 401, request("GET", "/api/admin/features", "", "").Code)
	require.Equal(t, 403, request("GET", "/api/admin/features", "ordinary", "").Code)
	require.Equal(t, 403, request("PUT", "/api/admin/features/profile.audit_logs", "ordinary", `{}`).Code)
	require.Equal(t, 200, request("GET", "/api/admin/features", "admin", "").Code)
	for _, body := range []string{`{}`, `{"audience":"all","percentage":1.5,"stage":"beta","user_ids":[]}`, `{"audience":"all","percentage":101,"stage":"beta","user_ids":[]}`, `{"audience":"all","percentage":100,"stage":"beta","user_ids":["missing"]}`, `{"audience":"all","percentage":100,"stage":"beta","user_ids":[],"unknown":true}`} {
		require.Equal(t, 400, request("PUT", "/api/admin/features/profile.audit_logs", "admin", body).Code)
	}
	require.Equal(t, 200, request("PUT", "/api/admin/features/profile.audit_logs", "admin", `{"audience":"selected","percentage":0,"stage":"beta","user_ids":["ordinary"]}`).Code)
	require.NotEmpty(t, sink.events)
	saved := sink.events[len(sink.events)-1]
	require.Equal(t, "admin.feature.update", saved.Action)
	require.Equal(t, feature.AuditLogs, saved.TargetID)
	require.Equal(t, "success", saved.Outcome)
	require.NotContains(t, saved.Details, "ordinary")
	require.Contains(t, saved.Details, "user_ids")
	for _, actor := range []string{"ordinary", "admin"} {
		res := request("GET", "/api/user/profile", actor, "")
		require.Equal(t, 200, res.Code, res.Body.String())
		var payload struct {
			Data dto.ProfileResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(res.Body.Bytes(), &payload))
		require.Equal(t, actor == "ordinary", payload.Data.Features[feature.AuditLogs].Enabled)
		require.NotContains(t, res.Body.String(), "user_ids")
		require.NotContains(t, res.Body.String(), "percentage")
		apiResponse := request("GET", "/api/user/audit-logs", actor, "")
		if actor == "ordinary" {
			require.Equal(t, 200, apiResponse.Code)
		} else {
			require.Equal(t, 403, apiResponse.Code)
			require.Contains(t, apiResponse.Body.String(), "FEATURE_NOT_ENABLED")
		}
	}
	zero := 0
	_, err = service.Update(context.Background(), feature.AuditLogs, dto.FeatureUpdate{Audience: "off", Percentage: &zero, Stage: "beta", UserIDs: []string{"ordinary"}})
	require.NoError(t, err)
	require.Equal(t, 403, request("GET", "/api/user/audit-logs", "ordinary", "").Code)
	require.NoError(t, database.Migrator().DropTable(&model.FeatureConfig{}))
	require.Equal(t, 503, request("GET", "/api/user/audit-logs", "ordinary", "").Code)
	require.Equal(t, 500, request("GET", "/api/user/profile", "ordinary", "").Code)
}
