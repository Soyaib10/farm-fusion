package httpapi

import (
	"context"
	"errors"
	"fmt"
	stdlog "log"
	"net/http"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/config"
	"github.com/Soyaib10/farm-fusion/internal/platform/logger"
)

const (
	readHeaderTimeout = 5 * time.Second
	maxHeaderBytes    = 1 << 20 // 1 MiB
)

// Server wraps net/http.Server with hardened timeouts and graceful shutdown.
type Server struct {
	httpServer      *http.Server
	shutdownTimeout time.Duration
	log             *logger.Logger
}

// NewServer constructs a hardened HTTP Server.
func NewServer(cfg config.ServerConfig, handler http.Handler, log *logger.Logger) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:              cfg.Addr(),
			Handler:           handler,
			ReadHeaderTimeout: readHeaderTimeout,
			ReadTimeout:       cfg.ReadTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
			MaxHeaderBytes:    maxHeaderBytes,
			ErrorLog:          stdlog.New(log, "", 0),
		},
		shutdownTimeout: cfg.ShutdownTimeout,
		log:             log,
	}
}

// Serve listens for HTTP traffic and performs a graceful shutdown when ctx is cancelled.
func (s *Server) Serve(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		s.log.Info("HTTP server listening", map[string]string{"addr": s.httpServer.Addr})
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		s.log.Info("Shutting down HTTP server gracefully", nil)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
		defer cancel()

		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("http server shutdown: %w", err)
		}
		return <-errCh
	}
}
