package client

import (
	"context"
	"fmt"

	"github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	ratingErrors "github.com/Kpeewu/tissi-mah/services/rating-service/pkg/errors"
	userpb "github.com/Kpeewu/tissi-mah/services/rating-service/proto/gen/userpb"
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

// GetUserIDByFirebaseID résout le Firebase UID de l'appelant en UserID interne (utilisé par
// l'intercepteur). Retourne ErrorUserNotFound si aucun profil ne correspond.
func (c *UserServiceClient) GetUserIDByFirebaseID(ctx context.Context, firebaseUID string) (string, error) {
	resp, err := c.grpcClient.GetUserByFirebaseID(ctx, &userpb.GetUserByFirebaseIDRequest{
		FirebaseID: firebaseUID,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return "", ratingErrors.ErrorUserNotFound
		}
		c.logger.Error("client: GetUserByFirebaseID failed", zap.Error(err))
		return "", fmt.Errorf("user-service: GetUserByFirebaseID failed: %w", err)
	}
	if resp.UserID == "" {
		return "", ratingErrors.ErrorUserNotFound
	}
	return resp.UserID, nil
}

// UserExists vérifie qu'un utilisateur existe dans user-service via son UserID.
// Retourne true si l'utilisateur existe, false si NOT_FOUND.
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
		c.logger.Error("client: UserExists failed", zap.Error(err), zap.String("userID", userID))
		return false, fmt.Errorf("user-service: GetUserByUserID failed: %w", err)
	}

	return true, nil
}

// GetUsersByUserIDs récupère en batch les profils (nom/prénom/photo) de plusieurs utilisateurs.
// Retourne une map indexée par UserID. En cas d'erreur, retourne une map vide (dégradation gracieuse).
func (c *UserServiceClient) GetUsersByUserIDs(ctx context.Context, userIDs []string) (map[string]*UserProfile, error) {
	c.logger.Debug("client: GetUsersByUserIDs called", zap.Int("count", len(userIDs)))

	out := make(map[string]*UserProfile, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}

	resp, err := c.grpcClient.GetUsersByUserIDs(ctx, &userpb.GetUsersByUserIDsRequest{UserIDs: userIDs})
	if err != nil {
		c.logger.Error("client: GetUsersByUserIDs failed", zap.Error(err))
		return nil, fmt.Errorf("user-service: GetUsersByUserIDs failed: %w", err)
	}

	for _, u := range resp.Users {
		out[u.UserID] = &UserProfile{
			FirstName:       u.FirstName,
			LastName:        u.Name,
			ProfileImageURL: u.ProfileImageURL,
		}
	}
	return out, nil
}
