package oauth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"sso-server/conf"
	"sso-server/dal/kv"
	apioauth "sso-server/handler/api/oauth"
)

func Test_ThirdPartyCallback_BindingCancellationReturnsToBindingPage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	database, err := gorm.Open(sqlite.Open("file:oauth_callback_binding?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	store := kv.NewMemoryStore()
	if err := store.Set(t.Context(), kv.KeyOAuthState("binding-state"), `{"provider":"github","redirect":"/profile?bind=success","action":"bind","user_id":"u1"}`, 0); err != nil {
		t.Fatalf("store binding state: %v", err)
	}

	handler := apioauth.NewOAuthHandler(apioauth.OAuthDeps{
		Config: &conf.Config{},
		DB:     database,
		KV:     store,
	})
	router := gin.New()
	router.GET("/api/auth/third/:provider/callback", handler.ThirdPartyCallback)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/auth/third/github/callback?state=binding-state&error=access_denied", nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected redirect, got %d: %s", response.Code, response.Body.String())
	}
	if location := response.Header().Get("Location"); location != "/profile/third-party-bind?error=%E7%AC%AC%E4%B8%89%E6%96%B9%E6%8E%88%E6%9D%83%E5%B7%B2%E5%8F%96%E6%B6%88%E6%88%96%E6%9C%AA%E5%AE%8C%E6%88%90&provider=github" {
		t.Fatalf("unexpected redirect location: %s", location)
	}
}
