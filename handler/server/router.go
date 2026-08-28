package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"sso-server/conf"
	"sso-server/dal/db"
	"sso-server/dal/kv"
	"sso-server/handler/api/admin"
	"sso-server/handler/api/auth"
	"sso-server/handler/api/oauth"
	passkeyapi "sso-server/handler/api/passkey"
	"sso-server/handler/api/user"
	"sso-server/handler/health"
	"sso-server/handler/oauth2"
	"sso-server/service/reauth"
)

func (s *Server) registerRoutes() {
	// Static files
	s.engine.Static("/assets", "./web/assets")
	s.engine.StaticFile("/register.html", "./web/register.html")

	// SPA root - catch all routes
	s.engine.NoRoute(func(c *gin.Context) {
		c.File("./web/index.html")
	})

	healthHandler := health.NewHealthHandler()

	s.engine.GET("/healthz", healthHandler.Healthz)

	o, err := oauth2.New(s.cfg)
	if err != nil {
		s.engine.GET("/oauth/authorize", func(c *gin.Context) { c.Status(http.StatusInternalServerError) })
		s.engine.POST("/oauth/token", func(c *gin.Context) { c.Status(http.StatusInternalServerError) })
		s.engine.GET("/oauth/userinfo", func(c *gin.Context) { c.Status(http.StatusInternalServerError) })
		o = nil
	}

	baseKVStore := kv.Store(kv.NewMemoryStore())
	if kv.Client != nil {
		baseKVStore = kv.NewRedisStore(kv.Client)
	}
	kvStore := kv.Store(kv.NewNamespacedStore(baseKVStore, conf.GetEnvironmentName()))

	authHandler := auth.NewAuthHandler(auth.AuthDeps{
		Config:        s.cfg,
		DB:            db.DB,
		KV:            kvStore,
		MessageSender: s.messageCenterClient,
		OAuth2:        o,
	})
	reauthService := reauth.NewService(s.cfg, kvStore)
	passkeyHandler := passkeyapi.NewHandler(passkeyapi.Deps{
		Config: s.cfg, DB: db.DB, KV: kvStore, Auth: authHandler.Service(),
	}, reauthService)

	userHandler := user.NewUserHandler(user.UserDeps{
		Config:     s.cfg,
		DB:         db.DB,
		KV:         kvStore,
		OAuth2:     o,
		ImageStore: s.imageStore,
	})

	oauthHandler := oauth.NewOAuthHandler(oauth.OAuthDeps{
		Config: s.cfg,
		DB:     db.DB,
		KV:     kvStore,
		OAuth2: o,
	})

	adminHandler := admin.NewAdminHandler(admin.AdminDeps{
		Config:     s.cfg,
		DB:         db.DB,
		ImageStore: s.imageStore,
	})

	authRequired := RequireSessionAuth(authHandler.Service())
	authRequiredOrRedirect := RequireSessionAuthOrRedirect(authHandler.Service())
	adminRequired := RequireAdmin(s.cfg)
	passkeyRequired := RequirePasskeyReauth(reauthService)

	apiGroup := s.engine.Group("/api")
	{
		authGroup := apiGroup.Group("/auth")
		{
			authGroup.GET("/captcha", authHandler.GenerateCaptcha)
			authGroup.POST("/login/password", authHandler.LoginWithPassword)
			authGroup.POST("/login/email", authHandler.LoginWithEmailOTP)
			authGroup.POST("/email/send", authHandler.SendEmailOTP)

			authGroup.GET("/qr/generate", authHandler.GenerateQRCode)
			authGroup.GET("/qr/poll", authHandler.PollQRCode)
			authGroup.POST("/qr/scan", authRequired, authHandler.ScanQRCode)
			authGroup.POST("/qr/confirm", authRequired, authHandler.ConfirmQRCode)
			authGroup.POST("/qr/complete", authHandler.CompleteQRCode)

			authGroup.GET("/third/:provider", oauthHandler.ThirdPartyLogin)
			authGroup.GET("/third/:provider/callback", oauthHandler.ThirdPartyCallback)

			authProtected := authGroup.Group("")
			authProtected.POST("/logout", authHandler.Logout)
			authProtected.POST("/token/refresh", authHandler.RefreshToken)

		}

		oauthAPIGroup := apiGroup.Group("/oauth")
		{
			oauthAPIGroup.GET("/client", oauthHandler.ClientInfo)
		}

		userGroup := apiGroup.Group("/user")
		{
			userGroup.POST("/register", userHandler.Register)
			userGroup.POST("/password/reset", userHandler.ResetPassword)

			userBrowserProtected := userGroup.Group("")
			userBrowserProtected.Use(authRequiredOrRedirect)
			userBrowserProtected.GET("/third/:provider/bind", oauthHandler.ThirdPartyBind)

			userProtected := userGroup.Group("")
			userProtected.Use(authRequired)
			userProtected.GET("/profile", userHandler.GetProfile)
			userProtected.PUT("/profile", userHandler.UpdateProfile)
			userProtected.POST("/avatar", userHandler.UploadAvatar)
			userProtected.GET("/devices", userHandler.GetLoginDevices)
			userProtected.DELETE("/devices/:device_id", userHandler.RevokeLoginDevice)
			userProtected.GET("/third/bindings/:binding_id", oauthHandler.GetThirdPartyBindingPreview)
			userProtected.POST("/third/bindings/:binding_id/confirm", passkeyRequired, oauthHandler.ConfirmThirdPartyBinding)
			userProtected.DELETE("/third/:provider", passkeyRequired, userHandler.UnbindThirdParty)
			userProtected.GET("/passkeys", passkeyHandler.List)
			userProtected.POST("/passkeys/registration/email/send", passkeyHandler.SendRegistrationEmail)
			userProtected.POST("/passkeys/registration/options", passkeyHandler.RegistrationOptions)
			userProtected.POST("/passkeys/registration/verify", passkeyHandler.RegistrationVerify)
			userProtected.PATCH("/passkeys/:id", passkeyHandler.Rename)
			userProtected.DELETE("/passkeys/:id", passkeyRequired, passkeyHandler.Delete)
			userProtected.POST("/reauth/passkey/options", passkeyHandler.ReauthOptions)
			userProtected.POST("/reauth/passkey/verify", passkeyHandler.ReauthVerify)
		}

		adminGroup := apiGroup.Group("/admin")
		adminGroup.Use(authRequired, adminRequired)
		{
			adminGroup.GET("/users", adminHandler.ListUsers)
			adminGroup.GET("/users/:id", adminHandler.GetUserDetail)
			adminGroup.GET("/oauth-clients", adminHandler.ListOAuthClients)
			adminGroup.GET("/oauth-clients/:id/secret", adminHandler.GetOAuthClientSecret)
			adminGroup.POST("/oauth-clients", adminHandler.CreateOAuthClient)
			adminGroup.PUT("/oauth-clients/:id", adminHandler.UpdateOAuthClient)
			adminGroup.POST("/oauth-clients/:id/logo", adminHandler.UploadOAuthClientLogo)
			adminGroup.DELETE("/oauth-clients/:id/logo", adminHandler.ClearOAuthClientLogo)
		}
	}

	if o != nil {
		s.engine.GET("/oauth/authorize", authRequiredOrRedirect, o.HandleAuthorize)
		s.engine.POST("/oauth/token", o.HandleToken)
		s.engine.GET("/oauth/userinfo", oauthHandler.HandleUserinfo)
	}
}
