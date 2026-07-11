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

type UserServiceClient struct {
	conn   *grpc.ClientConn
	client userpb.UserServiceClient
	logger *zap.Logger
}

func NewUserServiceClient(address string, logger *zap.Logger) (*UserServiceClient, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(grpcutil.ClientTransportCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to user-service at %s: %w", address, err)
	}

	return &UserServiceClient{
		conn:   conn,
		client: userpb.NewUserServiceClient(conn),
		logger: logger,
	}, nil
}

// GetUserIDByFirebaseID résout un Firebase UID en UserID interne via user-service.
// Le notification-service reçoit le Firebase UID depuis le contexte (injecté par
// l'api-gateway), mais device tokens et inbox sont keyés sur l'UserID interne (comme
// le fait le dispatcher). Cette méthode fait le pont entre les deux.
func (c *UserServiceClient) GetUserIDByFirebaseID(ctx context.Context, firebaseUID string) (string, error) {
	resp, err := c.client.GetUserByFirebaseID(ctx, &userpb.GetUserByFirebaseIDRequest{FirebaseID: firebaseUID})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			c.logger.Debug("user not found by firebaseUID", zap.String("firebase_uid", firebaseUID))
			return "", fmt.Errorf("user-service: user not found for firebaseUID %s", firebaseUID)
		}
		c.logger.Error("failed to resolve firebaseUID", zap.String("firebase_uid", firebaseUID), zap.Error(err))
		return "", fmt.Errorf("user-service: GetUserByFirebaseID failed: %w", err)
	}

	return resp.UserID, nil
}

func (c *UserServiceClient) GetUserByUserID(ctx context.Context, userID string) (*UserInfo, error) {
	resp, err := c.client.GetUserByUserID(ctx, &userpb.GetUserByUserIDRequest{UserID: userID})
	if err != nil {
		c.logger.Error("failed to get user", zap.String("user_id", userID), zap.Error(err))
		return nil, err
	}

	return &UserInfo{
		UserID:       resp.UserID,
		Name:         resp.Name,
		FirstName:    resp.FirstName,
		Email:        resp.Email,
		PhoneNumber:  resp.PhoneNumber,
		LanguageCode: resp.LanguageCode,
	}, nil
}

func (c *UserServiceClient) Close() error {
	return c.conn.Close()
}
