package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"sso-server/dal/db"
	"sso-server/dal/kv"
	"sso-server/handler/api/auth"
	"sso-server/handler/api/oauth"
	"sso-server/handler/api/user"
	"sso-server/handler/health"
	"sso-server/handler/oauth2"
	"sso-server/util/mailer"
)

func (s *Server) registerRoutes() {
	healthHandler := health.NewHealthHandler()

	s.engine.GET("/healthz", healthHandler.Healthz)

	o, err := oauth2.New(s.cfg)
	if err != nil {
		s.engine.GET("/oauth/authorize", func(c *gin.Context) { c.Status(http.StatusInternalServerError) })
		s.engine.POST("/oauth/token", func(c *gin.Context) { c.Status(http.StatusInternalServerError) })
		s.engine.GET("/oauth/userinfo", func(c *gin.Context) { c.Status(http.StatusInternalServerError) })
		o = nil
	}

	// Initialize KV store
	kvStore := kv.Store(kv.NewMemoryStore())
	if kv.Client != nil {
		kvStore = kv.NewRedisStore(kv.Client)
	}

	// Initialize mailer
	var mailerImpl mailer.Mailer
	if s.cfg != nil {
		mailerImpl = mailer.NewSMTPMailer(mailer.SMTPConfig{
			Addr: s.cfg.Email.SMTPAddr,
			User: s.cfg.Email.SMTPUser,
			Pass: s.cfg.Email.SMTPPass,
			From: s.cfg.Email.SMTPFrom,
		})
	}

	// Initialize handlers
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

	// API routes
	apiGroup := s.engine.Group("/api")
	{
		// Auth endpoints
		authGroup := apiGroup.Group("/auth")
		{
			authGroup.GET("/captcha", authHandler.GenerateCaptcha)
			authGroup.POST("/login/password", authHandler.LoginWithPassword)
			authGroup.POST("/login/email", authHandler.LoginWithEmailOTP)
			authGroup.POST("/email/send", authHandler.SendEmailOTP)

			// QR code endpoints
			authGroup.GET("/qr/generate", authHandler.GenerateQRCode)
			authGroup.GET("/qr/poll", authHandler.PollQRCode)
			authGroup.POST("/qr/scan", authHandler.ScanQRCode)
			authGroup.POST("/qr/confirm", authHandler.ConfirmQRCode)

			// Third-party auth endpoints
			authGroup.GET("/third/:provider", oauthHandler.ThirdPartyLogin)
			authGroup.GET("/third/:provider/callback", oauthHandler.ThirdPartyCallback)
			authGroup.POST("/third/bind", oauthHandler.BindThirdPartyAccount)

			// Deprecated: Use /api/user/register instead
			authGroup.POST("/register", func(c *gin.Context) {
				// Log deprecation warning
				c.Set("deprecated", "use /api/user/register instead")
				userHandler.Register(c)
			})
		}

		// User endpoints
		userGroup := apiGroup.Group("/user")
		{
			userGroup.POST("/register", userHandler.Register)
			userGroup.GET("/profile", userHandler.GetProfile)
			userGroup.PUT("/profile", userHandler.UpdateProfile)
		}
	}

	// OAuth2 protocol endpoints
	if o != nil {
		s.engine.GET("/oauth/authorize", o.HandleAuthorize)
		s.engine.POST("/oauth/token", o.HandleToken)
		s.engine.GET("/oauth/userinfo", oauthHandler.HandleUserinfo)
	}
}
