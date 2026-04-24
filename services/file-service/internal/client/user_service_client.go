package client

import (
	"context"
	"fmt"

	"github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	userpb "github.com/Kpeewu/tissi-mah/services/user-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UserServiceClient est le client gRPC vers user-service.
// Utilise TLS intra-cluster (skip-verify) via grpcutil.
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
		grpc.WithTransportCredentials(grpcutil.ClientTransportCredentials()),
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

// GetInternalUserIDByFirebaseID résout un Firebase UID en UserID interne MongoDB
// via user-service.GetUserByFirebaseID.
//
// Utilisé par UploadIdDocument pour garantir que les documents sont stockés
// avec l'UUID interne (convention partagée avec user-service et kyc-service)
// plutôt que le Firebase UID fourni par le client.
func (c *UserServiceClient) GetInternalUserIDByFirebaseID(ctx context.Context, firebaseUID string) (string, error) {
	c.logger.Debug("client: GetInternalUserIDByFirebaseID called", zap.String("firebaseUID", firebaseUID))

	resp, err := c.grpcClient.GetUserByFirebaseID(ctx, &userpb.GetUserByFirebaseIDRequest{
		FirebaseID: firebaseUID,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			c.logger.Debug("client: user not found by firebaseUID", zap.String("firebaseUID", firebaseUID))
			return "", fmt.Errorf("user-service: user not found for firebaseUID %s", firebaseUID)
		}
		c.logger.Error("client: GetUserByFirebaseID failed", zap.Error(err), zap.String("firebaseUID", firebaseUID))
		return "", fmt.Errorf("user-service: GetUserByFirebaseID failed: %w", err)
	}

	return resp.UserID, nil
}
