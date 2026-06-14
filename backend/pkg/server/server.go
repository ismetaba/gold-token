// Package server provides the shared HTTP bootstrap used by every service's
// main: standard timeouts and a graceful-shutdown run loop. It exists so the
// ~20 lines of identical listen/select/Shutdown boilerplate (and its easy-to-
// get-wrong error handling) live in one place rather than in 15 copies.
package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Timeouts configures an http.Server's timeouts. The zero value of a field
// means "no timeout" (matching net/http), which is what long-lived endpoints
// such as WebSocket upgrades need for WriteTimeout.
type Timeouts struct {
	ReadHeader time.Duration
	Read       time.Duration
	Write      time.Duration
	Idle       time.Duration
}

// DefaultTimeouts returns the conservative defaults used by most services
// (5s header, 15s read, 15s write).
func DefaultTimeouts() Timeouts {
	return Timeouts{
		ReadHeader: 5 * time.Second,
		Read:       15 * time.Second,
		Write:      15 * time.Second,
	}
}

// NewHTTPServer builds an http.Server with the given address, handler and
// timeouts.
func NewHTTPServer(addr string, h http.Handler, t Timeouts) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: t.ReadHeader,
		ReadTimeout:       t.Read,
		WriteTimeout:      t.Write,
		IdleTimeout:       t.Idle,
	}
}

// Serve runs srv until ctx is cancelled, then gracefully shuts it down within
// shutdownTimeout (10s if non-positive). It returns the listen error, if any;
// a clean shutdown returns nil.
func Serve(ctx context.Context, srv *http.Server, log *zap.Logger, shutdownTimeout time.Duration) error {
	if shutdownTimeout <= 0 {
		shutdownTimeout = 10 * time.Second
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("http listen", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutting down")
		shutCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return srv.Shutdown(shutCtx)
	}
}
