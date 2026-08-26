package user_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
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
	apiuser "sso-server/handler/api/user"
	serverhandler "sso-server/handler/server"
	"sso-server/model"
)

type fakeAvatarStore struct {
	objectKey string
	avatarURL string
	uploaded  []byte
}

func (s *fakeAvatarStore) UploadAvatar(_ context.Context, _ string, _ string, body io.Reader, _ int64) (string, string, error) {
	var err error
	s.uploaded, err = io.ReadAll(body)
	if err != nil {
		return "", "", err
	}
	return s.objectKey, s.avatarURL, nil
}

func (s *fakeAvatarStore) DeleteAvatar(_ context.Context, _ string) error {
	return nil
}

func TestUserProfile_RequiresSessionCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.OAuthClient{}, &model.UserThirdParty{}, &model.UserOAuthClient{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	h := apiuser.NewUserHandler(apiuser.UserDeps{
		Config: &conf.Config{},
		DB:     db,
		KV:     kv.NewMemoryStore(),
	})

	r := gin.New()
	r.GET("/api/user/profile", serverhandler.RequireSessionAuth(kv.NewMemoryStore()), h.GetProfile)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/user/profile", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestUserProfile_WithSessionCookieReturnsUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.OAuthClient{}, &model.UserThirdParty{}, &model.UserOAuthClient{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	email := "u1@example.com"
	if err := db.Create(&model.User{
		ID:       "u1",
		Email:    &email,
		IsActive: true,
	}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Create(&model.OAuthClient{
		Name:         "demo app",
		ClientID:     "c1",
		ClientSecret: "s1",
		HomepageURL:  "https://demo.example.com",
		RedirectURI:  "http://localhost/cb",
	}).Error; err != nil {
		t.Fatalf("create oauth client: %v", err)
	}
	if err := db.Create(&model.UserOAuthClient{
		UserID:      "u1",
		ClientID:    "c1",
		LastLoginAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("create user oauth client: %v", err)
	}
	if err := db.Create(&model.UserThirdParty{
		UserID:      "u1",
		Provider:    "github",
		ProviderUID: "gh_1",
	}).Error; err != nil {
		t.Fatalf("create third party binding: %v", err)
	}

	kvStore := kv.NewMemoryStore()
	if err := kvStore.Set(context.Background(), kv.KeySession("sid-1"), "u1", 12*time.Hour); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	h := apiuser.NewUserHandler(apiuser.UserDeps{
		Config: &conf.Config{},
		DB:     db,
		KV:     kvStore,
	})

	r := gin.New()
	r.GET("/api/user/profile", serverhandler.RequireSessionAuth(kvStore), h.GetProfile)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/user/profile", nil)
	req.AddCookie(&http.Cookie{Name: "sso_session", Value: "sid-1"})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			User struct {
				ID string `json:"id"`
			} `json:"user"`
			Applications []struct {
				ClientID    string `json:"client_id"`
				Name        string `json:"name"`
				HomepageURL string `json:"homepage_url"`
			} `json:"applications"`
			ThirdPartyProviders []struct {
				Provider string `json:"provider"`
				Bound    bool   `json:"bound"`
			} `json:"third_party_providers"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Code != 200 || resp.Data.User.ID != "u1" {
		t.Fatalf("expected user u1, got %s", w.Body.String())
	}
	if len(resp.Data.Applications) != 1 || resp.Data.Applications[0].Name != "demo app" {
		t.Fatalf("expected demo app, got %s", w.Body.String())
	}
	if resp.Data.Applications[0].HomepageURL != "https://demo.example.com" {
		t.Fatalf("expected homepage url, got %s", w.Body.String())
	}
	if len(resp.Data.ThirdPartyProviders) != 2 {
		t.Fatalf("expected two providers, got %s", w.Body.String())
	}
	if resp.Data.ThirdPartyProviders[0].Provider != "github" || !resp.Data.ThirdPartyProviders[0].Bound {
		t.Fatalf("expected github bound, got %s", w.Body.String())
	}
}

func TestUserProfile_UpdateUsername(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file:update_username?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.OAuthClient{}, &model.UserThirdParty{}, &model.UserOAuthClient{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Create(&model.User{ID: "u1", IsActive: true}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	kvStore := kv.NewMemoryStore()
	if err := kvStore.Set(context.Background(), kv.KeySession("sid-update"), "u1", 12*time.Hour); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	h := apiuser.NewUserHandler(apiuser.UserDeps{
		Config: &conf.Config{},
		DB:     db,
		KV:     kvStore,
	})

	r := gin.New()
	r.PUT("/api/user/profile", serverhandler.RequireSessionAuth(kvStore), h.UpdateProfile)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/user/profile", bytes.NewBufferString(`{"username":"  alice  "}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "sso_session", Value: "sid-update"})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			User struct {
				Username *string `json:"username"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Data.User.Username == nil || *resp.Data.User.Username != "alice" {
		t.Fatalf("expected trimmed username alice, got %s", w.Body.String())
	}
}

func TestUserProfile_UpdateUsernameRejectsDuplicate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file:update_duplicate_username?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.OAuthClient{}, &model.UserThirdParty{}, &model.UserOAuthClient{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	username := "bob"
	if err := db.Create(&model.User{ID: "u1", IsActive: true}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Create(&model.User{ID: "u2", Username: &username, IsActive: true}).Error; err != nil {
		t.Fatalf("create duplicate owner: %v", err)
	}

	kvStore := kv.NewMemoryStore()
	if err := kvStore.Set(context.Background(), kv.KeySession("sid-duplicate"), "u1", 12*time.Hour); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	h := apiuser.NewUserHandler(apiuser.UserDeps{
		Config: &conf.Config{},
		DB:     db,
		KV:     kvStore,
	})

	r := gin.New()
	r.PUT("/api/user/profile", serverhandler.RequireSessionAuth(kvStore), h.UpdateProfile)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/user/profile", bytes.NewBufferString(`{"username":"bob"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "sso_session", Value: "sid-duplicate"})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestUserProfile_UpdateProfileRejectsAvatarURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file:update_profile_avatar_url?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Create(&model.User{ID: "u1", IsActive: true}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	h := apiuser.NewUserHandler(apiuser.UserDeps{Config: &conf.Config{}, DB: db, KV: kv.NewMemoryStore()})
	r := gin.New()
	r.PUT("/api/user/profile", func(c *gin.Context) {
		c.Set("user_id", "u1")
	}, h.UpdateProfile)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/user/profile", bytes.NewBufferString(`{"avatar_url":"https://attacker.example/avatar.png"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestUserProfile_UploadAvatarAcceptsPNG(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := newAvatarUploadTestDB(t)
	store := &fakeAvatarStore{
		objectKey: "avatars/u1/avatar.png",
		avatarURL: "https://cdn.example.com/avatars/u1/avatar.png",
	}
	h := apiuser.NewUserHandler(apiuser.UserDeps{Config: &conf.Config{}, DB: db, KV: kv.NewMemoryStore(), AvatarStore: store})
	r := newAvatarUploadTestRouter(h)

	body, contentType := avatarMultipartBody(t, "avatar.png", pngAvatar(t))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/user/avatar", &body)
	req.Header.Set("Content-Type", contentType)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	if len(store.uploaded) == 0 {
		t.Fatal("expected avatar to be uploaded")
	}

	var user model.User
	if err := db.First(&user, "id = ?", "u1").Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	if user.AvatarURL == nil || *user.AvatarURL != store.avatarURL {
		t.Fatalf("unexpected avatar URL: %#v", user.AvatarURL)
	}
}

func TestUserProfile_UploadAvatarAcceptsJPEGAndWebP(t *testing.T) {
	testCases := []struct {
		name     string
		fileName string
		content  func(*testing.T) []byte
	}{
		{name: "JPEG", fileName: "avatar.jpeg", content: jpegAvatar},
		{name: "WebP", fileName: "avatar.webp", content: webpAvatar},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			db := newAvatarUploadTestDB(t)
			store := &fakeAvatarStore{
				objectKey: "avatars/u1/avatar",
				avatarURL: "https://cdn.example.com/avatars/u1/avatar",
			}
			h := apiuser.NewUserHandler(apiuser.UserDeps{Config: &conf.Config{}, DB: db, KV: kv.NewMemoryStore(), AvatarStore: store})
			r := newAvatarUploadTestRouter(h)
			body, contentType := avatarMultipartBody(t, testCase.fileName, testCase.content(t))

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/user/avatar", &body)
			req.Header.Set("Content-Type", contentType)
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestUserProfile_UploadAvatarRejectsInvalidInput(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testCases := []struct {
		name     string
		fileName string
		content  []byte
		message  string
	}{
		{name: "unsupported content", fileName: "avatar.txt", content: []byte("not an image"), message: "仅支持 JPEG、PNG 或 WebP 图片"},
		{name: "too large", fileName: "avatar.png", content: []byte(strings.Repeat("a", 2*1024*1024+1)), message: "头像大小不能超过 2MB"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			db := newAvatarUploadTestDB(t)
			h := apiuser.NewUserHandler(apiuser.UserDeps{Config: &conf.Config{}, DB: db, KV: kv.NewMemoryStore()})
			r := newAvatarUploadTestRouter(h)
			body, contentType := avatarMultipartBody(t, testCase.fileName, testCase.content)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/user/avatar", &body)
			req.Header.Set("Content-Type", contentType)
			r.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), testCase.message) {
				t.Fatalf("expected validation error %q, got %d %s", testCase.message, w.Code, w.Body.String())
			}
		})
	}
}

func newAvatarUploadTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := database.Create(&model.User{ID: "u1", IsActive: true}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return database
}

func newAvatarUploadTestRouter(h *apiuser.UserHandler) *gin.Engine {
	r := gin.New()
	r.POST("/api/user/avatar", func(c *gin.Context) {
		c.Set("user_id", "u1")
	}, h.UploadAvatar)
	return r
}

func avatarMultipartBody(t *testing.T, filename string, content []byte) (bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return body, writer.FormDataContentType()
}

func pngAvatar(t *testing.T) []byte {
	t.Helper()
	var body bytes.Buffer
	if err := png.Encode(&body, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
	return body.Bytes()
}

func jpegAvatar(t *testing.T) []byte {
	t.Helper()
	var body bytes.Buffer
	if err := jpeg.Encode(&body, image.NewRGBA(image.Rect(0, 0, 1, 1)), nil); err != nil {
		t.Fatalf("encode JPEG: %v", err)
	}
	return body.Bytes()
}

func webpAvatar(t *testing.T) []byte {
	t.Helper()
	content, err := base64.StdEncoding.DecodeString("UklGRiIAAABXRUJQVlA4IBYAAADQAQCdASoBAAEAAUAmJaQAA3AA/vuUAAA=")
	if err != nil {
		t.Fatalf("decode WebP: %v", err)
	}
	return content
}
