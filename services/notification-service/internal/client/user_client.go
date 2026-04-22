package client

import (
	"context"
	"fmt"

	"github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	userpb "github.com/Kpeewu/tissi-mah/services/user-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
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
