package client

import (
	"context"
	"fmt"
	"time"

	modErrors "github.com/Kpeewu/tissi-mah/services/moderation-service/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	// Stub généré depuis auth.proto (copié dans le service)
	authpb "github.com/Kpeewu/tissi-mah/services/moderation-service/proto/gen/authpb"
)

// AuthClient abstrait les appels vers auth-service.
type AuthClient interface {
	SuspendAccount(ctx context.Context, authID string, suspendedUntil *time.Time, isBanned bool) error
	Close() error
}

type authServiceClient struct {
	conn   *grpc.ClientConn
	client authpb.AuthServiceClient
	logger *zap.Logger
}

func NewAuthServiceClient(address string, logger *zap.Logger) (AuthClient, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("auth-service client dial: %w", err)
	}
	return &authServiceClient{
		conn:   conn,
		client: authpb.NewAuthServiceClient(conn),
		logger: logger,
	}, nil
}

func (c *authServiceClient) SuspendAccount(ctx context.Context, authID string, suspendedUntil *time.Time, isBanned bool) error {
	req := &authpb.SuspendAccountRequest{
		AuthID:   authID,
		IsBanned: isBanned,
	}
	if suspendedUntil != nil {
		req.SuspendedUntil = suspendedUntil.UTC().Format(time.RFC3339)
	}

	_, err := c.client.SuspendAccount(ctx, req)
	if err != nil {
		c.logger.Error("auth-service SuspendAccount failed", zap.Error(err), zap.String("authID", authID))
		return modErrors.ErrorAuthServiceUnavailable
	}
	return nil
}

func (c *authServiceClient) Close() error {
	return c.conn.Close()
}
