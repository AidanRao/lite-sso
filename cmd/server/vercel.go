package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"sso-server/conf"
	"sso-server/dal/db"
	"sso-server/dal/db/migration"
	"sso-server/dal/kv"
	"sso-server/handler/server"
)

const startupShutdownTimeout = time.Second

type vercelDependencies struct {
	runMigrations func(context.Context, *conf.Config) error
	initDatabase  func(*conf.Config) error
	initKV        func(*conf.Config) error
	newHandler    func(*conf.Config) (http.Handler, error)
}

type startupHandler struct {
	ready   chan struct{}
	once    sync.Once
	handler http.Handler
	failed  bool
}

func defaultVercelDependencies() vercelDependencies {
	return vercelDependencies{
		runMigrations: migration.Up,
		initDatabase:  db.Init,
		initKV:        kv.Init,
		newHandler: func(cfg *conf.Config) (http.Handler, error) {
			return server.New(cfg)
		},
	}
}

func newStartupHandler() *startupHandler {
	return &startupHandler{
		ready: make(chan struct{}),
	}
}

func (h *startupHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	select {
	case <-h.ready:
	case <-request.Context().Done():
		return
	}

	if h.failed || h.handler == nil {
		http.Error(writer, "application initialization failed", http.StatusServiceUnavailable)
		return
	}
	h.handler.ServeHTTP(writer, request)
}

func (h *startupHandler) Ready(handler http.Handler) {
	h.publish(handler, false)
}

func (h *startupHandler) Fail() {
	h.publish(nil, true)
}

func (h *startupHandler) publish(handler http.Handler, failed bool) {
	h.once.Do(func() {
		h.handler = handler
		h.failed = failed
		close(h.ready)
	})
}

func runVercel(ctx context.Context, cfg *conf.Config, dependencies vercelDependencies) error {
	listener, err := net.Listen("tcp4", net.JoinHostPort("0.0.0.0", cfg.Server.Port))
	if err != nil {
		return fmt.Errorf("listen on port %s: %w", cfg.Server.Port, err)
	}

	return serveDuringInitialization(ctx, listener, func(ctx context.Context) (http.Handler, error) {
		return initializeVercelApplication(ctx, cfg, dependencies)
	})
}

func serveDuringInitialization(
	ctx context.Context,
	listener net.Listener,
	initialize func(context.Context) (http.Handler, error),
) error {
	gate := newStartupHandler()
	httpServer := &http.Server{
		Handler: gate,
	}
	serveErrors := make(chan error, 1)
	watchDone := make(chan struct{})

	go func() {
		serveErrors <- httpServer.Serve(listener)
	}()
	go func() {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), startupShutdownTimeout)
			defer cancel()
			_ = httpServer.Shutdown(shutdownCtx)
		case <-watchDone:
		}
	}()
	defer close(watchDone)

	startedAt := time.Now()
	log.Printf("HTTP listener ready on %s", listener.Addr())
	handler, err := initialize(ctx)
	if err != nil {
		gate.Fail()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), startupShutdownTimeout)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
		<-serveErrors
		return err
	}

	gate.Ready(handler)
	log.Printf("Application ready in %s", time.Since(startedAt).Round(time.Millisecond))
	serveErr := <-serveErrors
	if errors.Is(serveErr, http.ErrServerClosed) {
		return nil
	}
	return serveErr
}

func initializeVercelApplication(
	ctx context.Context,
	cfg *conf.Config,
	dependencies vercelDependencies,
) (http.Handler, error) {
	if err := runStartupStep("Database migrations", func() error {
		return dependencies.runMigrations(ctx, cfg)
	}); err != nil {
		return nil, err
	}

	if err := runStartupStep("PostgreSQL initialization", func() error {
		return dependencies.initDatabase(cfg)
	}); err != nil {
		return nil, err
	}

	if err := runStartupStep("Redis initialization", func() error {
		return dependencies.initKV(cfg)
	}); err != nil {
		return nil, err
	}

	startedAt := time.Now()
	log.Print("Application route initialization started")
	handler, err := dependencies.newHandler(cfg)
	if err != nil {
		return nil, fmt.Errorf("application route initialization: %w", err)
	}
	log.Printf("Application route initialization completed in %s", time.Since(startedAt).Round(time.Millisecond))
	return handler, nil
}

func runStartupStep(name string, step func() error) error {
	startedAt := time.Now()
	log.Printf("%s started", name)
	if err := step(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	log.Printf("%s completed in %s", name, time.Since(startedAt).Round(time.Millisecond))
	return nil
}
