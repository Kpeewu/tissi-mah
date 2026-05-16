package client

import (
	"context"
	"fmt"

	modErrors "github.com/Kpeewu/tissi-mah/services/moderation-service/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	userpb "github.com/Kpeewu/tissi-mah/services/moderation-service/proto/gen/userpb"
)

// UserClient abstrait les appels vers user-service pour obtenir l'auth_id d'un utilisateur.
type UserClient interface {
	GetAuthIDByUserID(ctx context.Context, userID string) (string, error)
	Close() error
}

type userServiceClient struct {
	conn   *grpc.ClientConn
	client userpb.UserServiceClient
	logger *zap.Logger
}

func NewUserServiceClient(address string, logger *zap.Logger) (UserClient, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("user-service client dial: %w", err)
	}
	return &userServiceClient{
		conn:   conn,
		client: userpb.NewUserServiceClient(conn),
		logger: logger,
	}, nil
}

func (c *userServiceClient) GetAuthIDByUserID(ctx context.Context, userID string) (string, error) {
	resp, err := c.client.GetUserByUserID(ctx, &userpb.GetUserByUserIDRequest{UserID: userID})
	if err != nil {
		c.logger.Error("user-service GetUserByUserID failed", zap.Error(err), zap.String("userID", userID))
		return "", modErrors.ErrorUserServiceUnavailable
	}
	if resp.AuthID == "" {
		return "", modErrors.ErrorInvalidInput
	}
	return resp.AuthID, nil
}

func (c *userServiceClient) Close() error {
	return c.conn.Close()
}
