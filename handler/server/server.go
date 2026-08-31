package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"

	"sso-server/conf"
	"sso-server/dal/db"
	"sso-server/handler/audit"
	manageraudit "sso-server/manager/audit"
	"sso-server/manager/messagecenter"
	manageross "sso-server/manager/oss"
	"sso-server/service/auth"
)

type Server struct {
	auditRecorder       *manageraudit.Recorder
	cfg                 *conf.Config
	engine              *gin.Engine
	messageCenterClient *messagecenter.Client
	imageStore          manageross.ImageStore
}

func New(cfg *conf.Config) (*Server, error) {
	if err := cfg.ValidateAuthSecrets(); err != nil {
		return nil, err
	}
	if err := cfg.ValidateOSS(); err != nil {
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
	recorder, err := manageraudit.New(db.NewAuditRepository(db.DB), cfg.Audit)
	if err != nil {
		return nil, err
	}
	engine.Use(audit.Middleware(recorder, cfg, func(r *http.Request) (string, string) {
		deviceID, _ := auth.DeviceIDFromRequest(r)
		return auth.RequestIP(r, cfg.Server.TrustProxyHeaders), deviceID
	}))
	engine.Use(gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, _ any) {
		audit.Failure(c, "PANIC")
		log.Printf("http panic recovered: method=%s route=%s", c.Request.Method, c.FullPath())
		c.AbortWithStatus(http.StatusInternalServerError)
	}))
	srv := &Server{
		auditRecorder:       recorder,
		cfg:                 cfg,
		engine:              engine,
		messageCenterClient: messageCenterClient,
		imageStore:          imageStore,
	}
	srv.registerRoutes()
	return srv, nil
}

// Start serves requests until cancellation, then drains the audit queue.
func (s *Server) Start(ctx context.Context) error {
	defer func() {
		drainCtx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
		defer cancel()
		if err := s.Shutdown(drainCtx); err != nil {
			log.Print("audit shutdown deadline exceeded")
		}
	}()
	listener, err := net.Listen("tcp", ":"+s.cfg.Server.Port)
	if err != nil {
		return err
	}
	return Serve(ctx, &http.Server{Handler: s.engine}, listener)
}

// Shutdown stops audit workers after HTTP handlers have finished.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.auditRecorder == nil {
		return nil
	}
	return s.auditRecorder.Shutdown(ctx)
}

// ServeHTTP delegates requests to the configured Gin engine.
func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	s.engine.ServeHTTP(writer, request)
}

func newMessageCenterClient(cfg *conf.Config) (*messagecenter.Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("create message center client: configuration is required")
	}
	if conf.GetEnvironmentName() == string(conf.EnvLocal) && cfg.Dev.SkipSendMessage {
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
