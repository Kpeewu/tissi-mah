package client

import (
	"context"
	"fmt"

	"github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	userpb "github.com/Kpeewu/tissi-mah/services/user-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// UserServiceClient est le client gRPC vers user-service.
type UserServiceClient struct {
	conn       *grpc.ClientConn
	grpcClient userpb.UserServiceClient
	logger     *zap.Logger
}

// NewUserServiceClient établit la connexion gRPC vers user-service.
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
		logger:     logger.Named("user-client"),
	}, nil
}

func (c *UserServiceClient) Close() error {
	return c.conn.Close()
}

func (c *UserServiceClient) GetUserByUserID(ctx context.Context, userID string) (*UserInfo, error) {
	resp, err := c.grpcClient.GetUserByUserID(ctx, &userpb.GetUserByUserIDRequest{
		UserID: userID,
	})
	if err != nil {
		c.logger.Error("get user failed", zap.Error(err), zap.String("userID", userID))
		return nil, fmt.Errorf("user-service: GetUserByUserID failed: %w", err)
	}

	return &UserInfo{
		UserID:         resp.UserID,
		Name:           resp.Name,
		FirstName:      resp.FirstName,
		WithdrawNumber: resp.WithdrawNumber,
	}, nil
}
