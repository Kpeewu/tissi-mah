package client

import (
	"context"
	"fmt"

	"github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	fileErrors "github.com/Kpeewu/tissi-mah/services/file-service/pkg/errors"
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

// GetUserIDByFirebaseID résout le Firebase UID de l'appelant en UserID interne (utilisé par
// l'intercepteur). Retourne ErrorUserNotFound si aucun profil ne correspond.
func (c *UserServiceClient) GetUserIDByFirebaseID(ctx context.Context, firebaseUID string) (string, error) {
	resp, err := c.grpcClient.GetUserByFirebaseID(ctx, &userpb.GetUserByFirebaseIDRequest{
		FirebaseID: firebaseUID,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return "", fileErrors.ErrorUserNotFound
		}
		c.logger.Error("client: GetUserByFirebaseID failed", zap.Error(err))
		return "", fmt.Errorf("user-service: GetUserByFirebaseID failed: %w", err)
	}
	if resp.UserID == "" {
		return "", fileErrors.ErrorUserNotFound
	}
	return resp.UserID, nil
}

// GetUserProfileByFirebaseID retourne l'UUID interne + prenom/nom du user.
// Utilisé par UploadVehicleDocuments pour construire un docName humain-lisible.
func (c *UserServiceClient) GetUserProfileByFirebaseID(ctx context.Context, firebaseUID string) (*UserProfile, error) {
	c.logger.Debug("client: GetUserProfileByFirebaseID called", zap.String("firebaseUID", firebaseUID))

	resp, err := c.grpcClient.GetUserByFirebaseID(ctx, &userpb.GetUserByFirebaseIDRequest{
		FirebaseID: firebaseUID,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			c.logger.Debug("client: user not found by firebaseUID", zap.String("firebaseUID", firebaseUID))
			return nil, fmt.Errorf("user-service: user not found for firebaseUID %s", firebaseUID)
		}
		c.logger.Error("client: GetUserByFirebaseID failed", zap.Error(err), zap.String("firebaseUID", firebaseUID))
		return nil, fmt.Errorf("user-service: GetUserByFirebaseID failed: %w", err)
	}

	return &UserProfile{
		UserID:    resp.UserID,
		FirstName: resp.FirstName,
		LastName:  resp.Name,
	}, nil
}
