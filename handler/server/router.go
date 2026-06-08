package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"sso-server/dal/db"
	"sso-server/dal/kv"
	"sso-server/handler/api/admin"
	"sso-server/handler/api/auth"
	"sso-server/handler/api/oauth"
	"sso-server/handler/api/user"
	"sso-server/handler/health"
	"sso-server/handler/oauth2"
	"sso-server/util/mailer"
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

	kvStore := kv.Store(kv.NewMemoryStore())
	if kv.Client != nil {
		kvStore = kv.NewRedisStore(kv.Client)
	}

	var mailerImpl mailer.Mailer
	if s.cfg != nil {
		mailerImpl = mailer.NewSMTPMailer(mailer.SMTPConfig{
			Host: s.cfg.Email.SMTPHost,
			Port: s.cfg.Email.SMTPPort,
			User: s.cfg.Email.SMTPUser,
			Pass: s.cfg.Email.SMTPPass,
			From: s.cfg.Email.SMTPFrom,
		})
	}

	authHandler := auth.NewAuthHandler(auth.AuthDeps{
		Config: s.cfg,
		DB:     db.DB,
		KV:     kvStore,
		Mailer: mailerImpl,
		OAuth2: o,
	})

	userHandler := user.NewUserHandler(user.UserDeps{
		Config: s.cfg,
		DB:     db.DB,
		KV:     kvStore,
		OAuth2: o,
	})

	oauthHandler := oauth.NewOAuthHandler(oauth.OAuthDeps{
		Config: s.cfg,
		DB:     db.DB,
		KV:     kvStore,
		OAuth2: o,
	})

	adminHandler := admin.NewAdminHandler(admin.AdminDeps{
		Config: s.cfg,
		DB:     db.DB,
	})

	authRequired := RequireSessionAuth(kvStore)
	authRequiredOrRedirect := RequireSessionAuthOrRedirect(kvStore)
	adminRequired := RequireAdmin(s.cfg)

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
			authGroup.POST("/qr/scan", authHandler.ScanQRCode)
			authGroup.POST("/qr/confirm", authHandler.ConfirmQRCode)
			authGroup.POST("/qr/complete", authHandler.CompleteQRCode)

			authGroup.GET("/third/:provider", oauthHandler.ThirdPartyLogin)
			authGroup.GET("/third/:provider/callback", oauthHandler.ThirdPartyCallback)

			authProtected := authGroup.Group("")
			authProtected.Use(authRequired)
			authProtected.POST("/logout", authHandler.Logout)

		}

		oauthAPIGroup := apiGroup.Group("/oauth")
		{
			oauthAPIGroup.GET("/client", oauthHandler.ClientInfo)
		}

		userGroup := apiGroup.Group("/user")
		{
			userGroup.POST("/register", userHandler.Register)
			userGroup.POST("/password/reset", userHandler.ResetPassword)

			userProtected := userGroup.Group("")
			userProtected.Use(authRequired)
			userProtected.GET("/profile", userHandler.GetProfile)
			userProtected.PUT("/profile", userHandler.UpdateProfile)
			userProtected.GET("/third/:provider/bind", oauthHandler.ThirdPartyBind)
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
		}
	}

	if o != nil {
		s.engine.GET("/oauth/authorize", authRequiredOrRedirect, o.HandleAuthorize)
		s.engine.POST("/oauth/token", o.HandleToken)
		s.engine.GET("/oauth/userinfo", oauthHandler.HandleUserinfo)
	}
}
