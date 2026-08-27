package server

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"sso-server/conf"
	"sso-server/manager/messagecenter"
	manageross "sso-server/manager/oss"
)

type Server struct {
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
	messageCenterClient, err := newMessageCenterClient(cfg)
	if err != nil {
		return nil, err
	}

	engine := gin.New()
	engine.Use(gin.Recovery())

	var imageStore manageross.ImageStore
	if cfg.OSS.IsConfigured() {
		imageStore, err = manageross.NewAvatarStorage(cfg.OSS)
		if err != nil {
			return nil, fmt.Errorf("create avatar store: %w", err)
		}
	}

	srv := &Server{
		cfg:                 cfg,
		engine:              engine,
		messageCenterClient: messageCenterClient,
		imageStore:          imageStore,
	}
	srv.registerRoutes()
	return srv, nil
}

func (s *Server) Start() error {
	httpServer := &http.Server{
		Addr:    ":" + s.cfg.Server.Port,
		Handler: s.engine,
	}
	return httpServer.ListenAndServe()
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
