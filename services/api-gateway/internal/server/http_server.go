package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Config contient la configuration du serveur HTTP
type Config struct {
	Host string
	Port string
}

// HTTPServer encapsule un serveur HTTP avec graceful shutdown
type HTTPServer struct {
	server *http.Server
	logger *zap.Logger
}

// New crée un nouveau serveur HTTP
func New(cfg Config, handler http.Handler, logger *zap.Logger) *HTTPServer {
	addr := net.JoinHostPort(cfg.Host, cfg.Port)
	return &HTTPServer{
		server: &http.Server{
			Addr:              addr,
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       120 * time.Second,
		},
		logger: logger,
	}
}

// Serve démarre le serveur HTTP et attend le signal d'arrêt via le contexte.
// Pattern identique à pkg/grpcutil/server.go : bloque jusqu'à ctx.Done() puis graceful shutdown.
func (s *HTTPServer) Serve(ctx context.Context) error {
	s.logger.Info("starting HTTP server", zap.String("address", s.server.Addr))

	errCh := make(chan error, 1)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		s.logger.Info("shutting down HTTP server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := s.server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("http server shutdown: %w", err)
		}
		return nil
	case err := <-errCh:
		return err
	}
}
