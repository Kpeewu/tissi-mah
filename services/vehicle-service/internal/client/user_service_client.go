package client

import (
	"context"
	"fmt"

	"github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	userpb "github.com/Kpeewu/tissi-mah/services/user-service/proto/gen"
	vehicleErrors "github.com/Kpeewu/tissi-mah/services/vehicle-service/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UserServiceClient est le client gRPC vers user-service.
type UserServiceClient struct {
	conn       *grpc.ClientConn
	grpcClient userpb.UserServiceClient
	logger     *zap.Logger
}

// NewUserServiceClient établit la connexion gRPC vers user-service.
// address doit être au format "host:port" (ex: "user-service:50052").
func NewUserServiceClient(address string, logger *zap.Logger) (*UserServiceClient, error) {
	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(grpcutil.ClientTransportCredentials()),
	)
	if err != nil {
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

// GetUserIDByFirebaseID résout le Firebase UID de l'appelant en UserID interne.
// Les véhicules référencent leur propriétaire par UserID interne.
// Retourne ErrorUserNotFound si aucun profil ne correspond.
func (c *UserServiceClient) GetUserIDByFirebaseID(ctx context.Context, firebaseUID string) (string, error) {
	resp, err := c.grpcClient.GetUserByFirebaseID(ctx, &userpb.GetUserByFirebaseIDRequest{
		FirebaseID: firebaseUID,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return "", vehicleErrors.ErrorUserNotFound
		}
		c.logger.Error("client: GetUserByFirebaseID failed", zap.Error(err))
		return "", fmt.Errorf("user-service: GetUserByFirebaseID failed: %w", err)
	}
	if resp.UserID == "" {
		return "", vehicleErrors.ErrorUserNotFound
	}
	return resp.UserID, nil
}
