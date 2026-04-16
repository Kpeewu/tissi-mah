// Package grpcutil provides shared gRPC utilities for all services.
package grpcutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// NewClientConn crée une connexion gRPC vers un service interne.
// En local : credentials insecure (pas de TLS).
// En vps-dev / staging / prod : TLS avec le pool de certificats système.
func NewClientConn(address string, environment string, logger *zap.Logger, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	transportCreds, err := transportCredentials(environment)
	if err != nil {
		return nil, fmt.Errorf("grpc transport credentials: %w", err)
	}

	allOpts := append([]grpc.DialOption{grpc.WithTransportCredentials(transportCreds)}, opts...)

	logger.Debug("dialing gRPC service",
		zap.String("address", address),
		zap.String("environment", environment),
		zap.Bool("tls", environment != "local"),
	)

	conn, err := grpc.NewClient(address, allOpts...)
	if err != nil {
		return nil, fmt.Errorf("grpc dial %s: %w", address, err)
	}

	return conn, nil
}

// transportCredentials retourne les credentials adaptées à l'environnement.
func transportCredentials(environment string) (credentials.TransportCredentials, error) {
	if environment == "local" {
		return insecure.NewCredentials(), nil
	}

	// Charger le pool de certificats système
	certPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("failed to load system cert pool: %w", err)
	}

	return credentials.NewTLS(&tls.Config{
		RootCAs:    certPool,
		MinVersion: tls.VersionTLS12,
	}), nil
}
