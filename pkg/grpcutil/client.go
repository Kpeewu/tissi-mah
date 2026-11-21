// Package grpcutil provides shared gRPC utilities for all services.
package grpcutil

import (
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// NewClientConn crée une connexion gRPC TLS vers un service intra-cluster.
// Le handshake utilise InsecureSkipVerify : le chiffrement en transit est garanti,
// l'authentification du serveur repose sur le trust boundary du cluster.
func NewClientConn(address string, logger *zap.Logger, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	allOpts := append(
		[]grpc.DialOption{grpc.WithTransportCredentials(ClientTransportCredentials())},
		opts...,
	)

	logger.Debug("dialing gRPC service", zap.String("address", address))

	conn, err := grpc.NewClient(address, allOpts...)
	if err != nil {
		return nil, fmt.Errorf("grpc dial %s: %w", address, err)
	}
	return conn, nil
}
