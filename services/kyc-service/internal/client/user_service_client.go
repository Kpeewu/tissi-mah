package client

import (
	"context"
	"fmt"

	"github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	"github.com/Kpeewu/tissi-mah/services/kyc-service/internal/domain"
	userpb "github.com/Kpeewu/tissi-mah/services/user-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UserServiceClient est le client gRPC vers user-service.
// Utilise TLS avec skip-verify pour la communication intra-cluster.
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

// GetUserIDByFirebaseID résout un Firebase UID en UserID interne MongoDB via user-service.
// Le kyc-service reçoit le Firebase UID depuis le contexte (injecté par l'api-gateway),
// mais le file-service stocke les documents avec l'UserID interne MongoDB.
// Cette méthode fait le pont entre les deux.
func (c *UserServiceClient) GetUserIDByFirebaseID(ctx context.Context, firebaseUID string) (string, error) {
	c.logger.Debug("client: GetUserIDByFirebaseID called", zap.String("firebaseUID", firebaseUID))

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

// GetUserByUserID récupère les infos profil d'un utilisateur par son UserID interne.
func (c *UserServiceClient) GetUserByUserID(ctx context.Context, userID string) (*domain.UserInfo, error) {
	c.logger.Debug("client: GetUserByUserID called", zap.String("userID", userID))

	resp, err := c.grpcClient.GetUserByUserID(ctx, &userpb.GetUserByUserIDRequest{UserID: userID})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return nil, fmt.Errorf("user-service: user not found for userID %s", userID)
		}
		c.logger.Error("client: GetUserByUserID failed", zap.Error(err), zap.String("userID", userID))
		return nil, fmt.Errorf("user-service: GetUserByUserID failed: %w", err)
	}

	return &domain.UserInfo{
		UserID:          resp.UserID,
		Name:            resp.Name,
		FirstName:       resp.FirstName,
		Email:           resp.Email,
		PhoneNumber:     resp.PhoneNumber,
		ProfileImageURL: resp.ProfileImageURL,
	}, nil
}

// GetUsersByUserIDs récupère en batch les infos profil de plusieurs utilisateurs
// (nom/prénom/photo, sans email/phone). Retourne une map indexée par userID.
func (c *UserServiceClient) GetUsersByUserIDs(ctx context.Context, userIDs []string) (map[string]*domain.UserInfo, error) {
	c.logger.Debug("client: GetUsersByUserIDs called", zap.Int("count", len(userIDs)))

	out := make(map[string]*domain.UserInfo, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}

	resp, err := c.grpcClient.GetUsersByUserIDs(ctx, &userpb.GetUsersByUserIDsRequest{UserIDs: userIDs})
	if err != nil {
		c.logger.Error("client: GetUsersByUserIDs failed", zap.Error(err))
		return nil, fmt.Errorf("user-service: GetUsersByUserIDs failed: %w", err)
	}

	for _, u := range resp.Users {
		out[u.UserID] = &domain.UserInfo{
			UserID:          u.UserID,
			Name:            u.Name,
			FirstName:       u.FirstName,
			ProfileImageURL: u.ProfileImageURL,
		}
	}
	return out, nil
}
