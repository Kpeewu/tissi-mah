package client

import (
	"context"
	"fmt"

	userpb "github.com/Kpeewu/tissi-mah/services/user-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// UserServiceClient est le client gRPC vers user-service.
// Utilise insecure.NewCredentials() pour la communication intra-cluster.
type UserServiceClient struct {
	conn       *grpc.ClientConn
	grpcClient userpb.UserServiceClient
	logger     *zap.Logger
}

// NewUserServiceClient établit la connexion gRPC vers user-service.
// address doit être au format "host:port" (ex: "user-service:50052").
func NewUserServiceClient(address string, logger *zap.Logger) (*UserServiceClient, error) {
	logger.Debug("connecting to user-service", zap.String("address", address))

	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		logger.Error("failed to connect to user-service", zap.Error(err), zap.String("address", address))
		return nil, fmt.Errorf("user-service: failed to connect to %s: %w", address, err)
	}

	return &UserServiceClient{
		conn:       conn,
		grpcClient: userpb.NewUserServiceClient(conn),
		logger:     logger,
	}, nil
}

// Close libère la connexion gRPC. À appeler au shutdown du service.
func (c *UserServiceClient) Close() error {
	return c.conn.Close()
}

// IsVerifiedDriver vérifie que l'utilisateur existe dans user-service
// et que son profil conducteur est vérifié (IsDriverProfileVerified = true).
func (c *UserServiceClient) IsVerifiedDriver(ctx context.Context, userID string) (bool, error) {
	c.logger.Debug("client: IsVerifiedDriver called", zap.String("userID", userID))

	resp, err := c.grpcClient.GetUserByAuthID(ctx, &userpb.GetUserByAuthIDRequest{
		AuthID: userID,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			c.logger.Debug("client: user not found", zap.String("userID", userID))
			return false, nil
		}
		c.logger.Error("client: GetUserByAuthID failed", zap.Error(err), zap.String("userID", userID))
		return false, fmt.Errorf("user-service: GetUserByAuthID failed: %w", err)
	}

	return resp.IsDriverProfileVerified, nil
}
