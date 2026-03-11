package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"sso-server/conf"
)

type Server struct {
	cfg    *conf.Config
	engine *gin.Engine
}

func New(cfg *conf.Config) *Server {
	engine := gin.New()
	engine.Use(gin.Recovery())

	srv := &Server{
		cfg:    cfg,
		engine: engine,
	}
	srv.registerRoutes()
	return srv
}

func (s *Server) Start() error {
	httpServer := &http.Server{
		Addr:    ":" + s.cfg.Server.Port,
		Handler: s.engine,
	}
	return httpServer.ListenAndServe()
}
