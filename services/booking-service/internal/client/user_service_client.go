package client

import (
	"context"
	"fmt"
	"strings"

	"github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	bookingErrors "github.com/Kpeewu/tissi-mah/services/booking-service/pkg/errors"
	userpb "github.com/Kpeewu/tissi-mah/services/user-service/proto/gen"
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

// Close libère la connexion gRPC.
func (c *UserServiceClient) Close() error {
	return c.conn.Close()
}

// IsPassengerVerified vérifie qu'un utilisateur existe et que son profil passager est vérifié.
func (c *UserServiceClient) IsPassengerVerified(ctx context.Context, userID string) (bool, error) {
	c.logger.Debug("client: IsPassengerVerified called", zap.String("userID", userID))

	resp, err := c.getUser(ctx, userID)
	if err != nil {
		return false, err
	}
	if resp == nil {
		return false, nil
	}

	return resp.IsPassengerProfileVerified, nil
}

// getUser retrouve un utilisateur par son UserID interne : les réservations ne référencent
// passagers et chauffeurs que par UserID (résolu depuis le Firebase UID par l'intercepteur).
// Retourne (nil, nil) quand aucun profil ne correspond.
func (c *UserServiceClient) getUser(ctx context.Context, userID string) (*userpb.UserProfileResponse, error) {
	resp, err := c.grpcClient.GetUserByUserID(ctx, &userpb.GetUserByUserIDRequest{
		UserID: userID,
	})
	if err == nil {
		return resp, nil
	}
	if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
		return nil, nil
	}
	c.logger.Error("client: GetUserByUserID failed", zap.Error(err), zap.String("userID", userID))
	return nil, fmt.Errorf("user-service: GetUserByUserID failed: %w", err)
}

// GetPassengerInfo retourne le nom complet et le statut de vérification d'un passager.
func (c *UserServiceClient) GetPassengerInfo(ctx context.Context, userID string) (string, bool, error) {
	c.logger.Debug("client: GetPassengerInfo called", zap.String("userID", userID))

	resp, err := c.getUser(ctx, userID)
	if err != nil {
		return "", false, fmt.Errorf("user-service: GetPassengerInfo failed: %w", err)
	}
	if resp == nil {
		return "", false, nil
	}

	fullName := strings.TrimSpace(resp.FirstName + " " + resp.Name)
	return fullName, resp.IsPassengerProfileVerified, nil
}

// GetUserIDByFirebaseID résout le Firebase UID de l'appelant en UserID interne.
// Les réservations référencent passagers et chauffeurs par UserID interne : c'est
// l'identité à utiliser pour tout contrôle d'appartenance.
func (c *UserServiceClient) GetUserIDByFirebaseID(ctx context.Context, firebaseUID string) (string, error) {
	resp, err := c.grpcClient.GetUserByFirebaseID(ctx, &userpb.GetUserByFirebaseIDRequest{
		FirebaseID: firebaseUID,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return "", bookingErrors.ErrorUserNotFound
		}
		c.logger.Error("client: GetUserByFirebaseID failed", zap.Error(err))
		return "", fmt.Errorf("user-service: GetUserByFirebaseID failed: %w", err)
	}
	if resp.UserID == "" {
		return "", bookingErrors.ErrorUserNotFound
	}
	return resp.UserID, nil
}
