package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"

	"sso-server/common/event"
	"sso-server/conf"
	"sso-server/dal/db"
	"sso-server/handler/audit"
	manageraudit "sso-server/manager/audit"
	"sso-server/manager/eventbus"
	"sso-server/manager/messagecenter"
	manageross "sso-server/manager/oss"
	"sso-server/manager/thirdparty"
	"sso-server/service/auth"
	"sso-server/service/notification"
	"sso-server/service/user"
	"sso-server/util/ipgeo"
	"sso-server/util/requestmeta"
)

type Server struct {
	loginEvents         *eventbus.Bus
	userDeletionService *user.DeletionService
	locationResolver    *ipgeo.Locator
	auditRecorder       *manageraudit.Recorder
	cfg                 *conf.Config
	engine              *gin.Engine
	messageCenterClient *messagecenter.Client
	imageStore          manageross.ImageStore
	providerRegistry    *thirdparty.Registry
}

func New(cfg *conf.Config) (*Server, error) {
	if err := cfg.ValidateAuthSecrets(); err != nil {
		return nil, err
	}
	if err := cfg.ValidateOAuthTokenLifetimes(); err != nil {
		return nil, err
	}
	if err := cfg.ValidateSessionTokenLifetimes(); err != nil {
		return nil, err
	}
	if err := cfg.OSS.Validate(); err != nil {
		return nil, err
	}
	if err := cfg.ValidatePasskey(); err != nil {
		return nil, err
	}
	if err := cfg.ValidateReauth(); err != nil {
		return nil, err
	}
	if err := cfg.ValidateEmail(); err != nil {
		return nil, err
	}
	messageCenterClient, err := newMessageCenterClient(cfg)
	if err != nil {
		return nil, err
	}

	engine := gin.New()

	var imageStore manageross.ImageStore
	if cfg.OSS.IsConfigured() {
		imageStore, err = manageross.NewAvatarStorage(cfg.OSS)
		if err != nil {
			return nil, fmt.Errorf("create avatar store: %w", err)
		}
	}

	if err := cfg.Audit.Validate(); err != nil {
		return nil, err
	}
	if db.DB == nil {
		return nil, fmt.Errorf("database is required")
	}
	providerRegistry := thirdparty.NewBuiltins(cfg)
	recorder, err := manageraudit.New(db.NewAuditRepository(db.DB), cfg.Audit, providerRegistry)
	if err != nil {
		return nil, err
	}
	engine.Use(audit.Middleware(recorder, cfg, func(r *http.Request) (string, string) {
		deviceID, _ := auth.DeviceIDFromRequest(r)
		return requestmeta.IP(r, cfg.Server.TrustProxyHeaders), deviceID
	}, providerRegistry))
	engine.Use(gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, _ any) {
		audit.Failure(c, "PANIC")
		log.Printf("http panic recovered: method=%s route=%s", c.Request.Method, c.FullPath())
		c.AbortWithStatus(http.StatusInternalServerError)
	}))
	locationResolver, err := ipgeo.New(cfg.GeoIP.DataDir)
	if err != nil {
		_ = recorder.Shutdown(context.Background())
		return nil, fmt.Errorf("initialize IP geolocation: %w", err)
	}
	var subscriptions []eventbus.Subscription
	if messageCenterClient != nil {
		notifier := notification.NewLoginNotifier(db.DB, messageCenterClient, cfg.Email.VerificationBaseURL, providerRegistry)
		subscriptions = append(subscriptions, eventbus.Subscription{Type: event.LoginSucceededType, Handle: notifier.HandleLogin})
	}
	srv := &Server{
		loginEvents:         eventbus.New(subscriptions...),
		locationResolver:    locationResolver,
		auditRecorder:       recorder,
		cfg:                 cfg,
		engine:              engine,
		messageCenterClient: messageCenterClient,
		imageStore:          imageStore,
		providerRegistry:    providerRegistry,
	}
	if err := srv.registerRoutes(); err != nil {
		_ = srv.Shutdown(context.Background())
		return nil, fmt.Errorf("initialize OAuth: %w", err)
	}
	srv.userDeletionService.Start()
	return srv, nil
}

// Start serves requests until cancellation, then drains background work.
func (s *Server) Start(ctx context.Context) error {
	defer func() {
		drainCtx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
		defer cancel()
		if err := s.Shutdown(drainCtx); err != nil {
			log.Print("server shutdown deadline exceeded")
		}
	}()
	listener, err := net.Listen("tcp", ":"+s.cfg.Server.Port)
	if err != nil {
		return err
	}
	return Serve(ctx, &http.Server{Handler: s.engine}, listener)
}

// Shutdown releases shared resources after HTTP handlers have finished.
func (s *Server) Shutdown(ctx context.Context) error {
	var eventErr error
	if s.loginEvents != nil {
		eventErr = s.loginEvents.Shutdown(ctx)
	}
	var accountErr error
	if s.userDeletionService != nil {
		accountErr = s.userDeletionService.Shutdown(ctx)
	}
	if s.locationResolver != nil {
		s.locationResolver.Close()
	}
	if s.auditRecorder == nil {
		return errors.Join(eventErr, accountErr)
	}
	return errors.Join(eventErr, accountErr, s.auditRecorder.Shutdown(ctx))
}

// ServeHTTP delegates requests to the configured Gin engine.
func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	s.engine.ServeHTTP(writer, request)
}

func newMessageCenterClient(cfg *conf.Config) (*messagecenter.Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("create message center client: configuration is required")
	}
	if cfg.Dev.SkipSendMessage {
		return nil, nil
	}

	client, err := messagecenter.NewClient(messagecenter.Config{
		URL:       cfg.MessageCenter.URL,
		APIKey:    cfg.MessageCenter.APIKey,
		SenderKey: cfg.MessageCenter.SenderKey,
	})
	if err != nil {
		return nil, fmt.Errorf("create message center client: %w", err)
	}
	return client, nil
}
