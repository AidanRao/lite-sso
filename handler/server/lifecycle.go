package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

// ShutdownTimeout bounds HTTP draining and, separately, audit draining.
const ShutdownTimeout = 10 * time.Second

// Serve waits for active HTTP handlers before returning, including on cancellation.
// Both normal startup and the migration-gated listener use this lifecycle.
func Serve(ctx context.Context, httpServer *http.Server, listener net.Listener) error {
	finished := make(chan error, 1)
	go func() { finished <- httpServer.Serve(listener) }()
	var serveErr error
	select {
	case serveErr = <-finished:
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
		err := httpServer.Shutdown(shutdownCtx)
		cancel()
		if err != nil {
			_ = httpServer.Close()
		}
		serveErr = <-finished
	}
	if errors.Is(serveErr, http.ErrServerClosed) {
		return nil
	}
	return serveErr
}
