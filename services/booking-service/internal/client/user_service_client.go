package client

import (
	"context"
	"fmt"
	"strings"

	"github.com/Kpeewu/tissi-mah/pkg/grpcutil"
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

// UserExists vérifie qu'un utilisateur existe dans user-service.
func (c *UserServiceClient) UserExists(ctx context.Context, userID string) (bool, error) {
	c.logger.Debug("client: UserExists called", zap.String("userID", userID))

	_, err := c.grpcClient.GetUserByUserID(ctx, &userpb.GetUserByUserIDRequest{
		UserID: userID,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			c.logger.Debug("client: user not found", zap.String("userID", userID))
			return false, nil
		}
		c.logger.Error("client: GetUserByUserID failed", zap.Error(err), zap.String("userID", userID))
		return false, fmt.Errorf("user-service: GetUserByUserID failed: %w", err)
	}

	return true, nil
}

// IsPassengerVerified vérifie qu'un utilisateur existe et que son profil passager est vérifié.
func (c *UserServiceClient) IsPassengerVerified(ctx context.Context, userID string) (bool, error) {
	c.logger.Debug("client: IsPassengerVerified called", zap.String("userID", userID))

	resp, err := c.resolveUser(ctx, userID)
	if err != nil {
		return false, err
	}
	if resp == nil {
		return false, nil
	}

	return resp.IsPassengerProfileVerified, nil
}

// resolveUser retrouve un utilisateur à partir de l'identifiant porté par la réservation.
// Les clients transmettent aujourd'hui le Firebase UID, alors que user-service indexe les
// profils par UserID : la recherche par UserID est donc complétée par une recherche par
// Firebase UID. Sans cela, toute réservation était refusée avec ErrorPassengerNotVerified,
// le passager étant simplement introuvable.
// Retourne (nil, nil) quand aucun profil ne correspond.
func (c *UserServiceClient) resolveUser(ctx context.Context, passengerID string) (*userpb.UserProfileResponse, error) {
	resp, err := c.grpcClient.GetUserByUserID(ctx, &userpb.GetUserByUserIDRequest{
		UserID: passengerID,
	})
	if err == nil {
		return resp, nil
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.NotFound {
		c.logger.Error("client: GetUserByUserID failed", zap.Error(err), zap.String("passengerID", passengerID))
		return nil, fmt.Errorf("user-service: GetUserByUserID failed: %w", err)
	}

	resp, err = c.grpcClient.GetUserByFirebaseID(ctx, &userpb.GetUserByFirebaseIDRequest{
		FirebaseID: passengerID,
	})
	if err == nil {
		return resp, nil
	}
	if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
		return nil, nil
	}
	c.logger.Error("client: GetUserByFirebaseID failed", zap.Error(err), zap.String("passengerID", passengerID))
	return nil, fmt.Errorf("user-service: GetUserByFirebaseID failed: %w", err)
}

// GetPassengerInfo retourne le nom complet et le statut de vérification d'un passager.
func (c *UserServiceClient) GetPassengerInfo(ctx context.Context, userID string) (string, bool, error) {
	c.logger.Debug("client: GetPassengerInfo called", zap.String("userID", userID))

	resp, err := c.resolveUser(ctx, userID)
	if err != nil {
		return "", false, fmt.Errorf("user-service: GetPassengerInfo failed: %w", err)
	}
	if resp == nil {
		return "", false, nil
	}

	fullName := strings.TrimSpace(resp.FirstName + " " + resp.Name)
	return fullName, resp.IsPassengerProfileVerified, nil
}
