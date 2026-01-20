// Package grpcutil provides shared gRPC utilities for all services.
package grpcutil

import (
	"context"
	"fmt"
	"net"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// ServerConfig holds gRPC server configuration.
type ServerConfig struct {
	Port              int
	EnableReflection  bool
	EnableHealthCheck bool
}

// Server wraps a gRPC server with common functionality.
type Server struct {
	server   *grpc.Server
	listener net.Listener
	logger   *zap.Logger
	health   *health.Server
}

// NewServer creates a new gRPC server.
func NewServer(cfg ServerConfig, logger *zap.Logger, opts ...grpc.ServerOption) (*Server, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return nil, fmt.Errorf("failed to listen on port %d: %w", cfg.Port, err)
	}

	server := grpc.NewServer(opts...)

	s := &Server{
		server:   server,
		listener: listener,
		logger:   logger,
	}

	if cfg.EnableHealthCheck {
		s.health = health.NewServer()
		grpc_health_v1.RegisterHealthServer(server, s.health)
	}

	if cfg.EnableReflection {
		reflection.Register(server)
	}

	return s, nil
}

// Server returns the underlying gRPC server for service registration.
func (s *Server) Server() *grpc.Server {
	return s.server
}

// SetServingStatus sets the health status for a service.
func (s *Server) SetServingStatus(service string, status grpc_health_v1.HealthCheckResponse_ServingStatus) {
	if s.health != nil {
		s.health.SetServingStatus(service, status)
	}
}

// Serve starts the gRPC server.
func (s *Server) Serve(ctx context.Context) error {
	s.logger.Info("starting gRPC server", zap.String("address", s.listener.Addr().String()))

	errCh := make(chan error, 1)
	go func() {
		if err := s.server.Serve(s.listener); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		s.logger.Info("shutting down gRPC server")
		s.server.GracefulStop()
		return nil
	case err := <-errCh:
		return err
	}
}

// Stop gracefully stops the server.
func (s *Server) Stop() {
	s.server.GracefulStop()
}
