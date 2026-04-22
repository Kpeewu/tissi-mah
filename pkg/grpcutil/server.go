// Package grpcutil provides shared gRPC utilities for all services.
package grpcutil

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"os"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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

// NewServer creates a new gRPC server. Le serveur écoute en TLS avec un
// certificat self-signed auto-généré en mémoire au démarrage — les clients
// doivent utiliser grpcutil.ClientTransportCredentials (skip-verify).
func NewServer(cfg ServerConfig, logger *zap.Logger, opts ...grpc.ServerOption) (*Server, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return nil, fmt.Errorf("failed to listen on port %d: %w", cfg.Port, err)
	}

	cert, err := generateSelfSignedCert()
	if err != nil {
		return nil, fmt.Errorf("failed to generate self-signed cert: %w", err)
	}
	tlsCreds := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	})

	allOpts := append([]grpc.ServerOption{grpc.Creds(tlsCreds)}, opts...)
	server := grpc.NewServer(allOpts...)

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
	s.logger.Info("starting gRPC server (TLS)", zap.String("address", s.listener.Addr().String()))

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

// generateSelfSignedCert produit un certificat ECDSA P-256 self-signed valable
// 10 ans, destiné à être utilisé uniquement avec des clients en skip-verify.
func generateSelfSignedCert() (tls.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("ecdsa key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("serial: %w", err)
	}

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "tissimah-service"
	}

	template := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: hostname},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create cert: %w", err)
	}

	return tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  priv,
	}, nil
}
