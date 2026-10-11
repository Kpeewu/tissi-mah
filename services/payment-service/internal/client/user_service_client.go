package client

import (
	"context"
	"fmt"

	"github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
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

// GetUserIDByFirebaseID résout le UserID interne à partir du Firebase UID (ErrorUserNotFound si inconnu).
func (c *UserServiceClient) GetUserIDByFirebaseID(ctx context.Context, firebaseID string) (string, error) {
	resp, err := c.grpcClient.GetUserByFirebaseID(ctx, &userpb.GetUserByFirebaseIDRequest{
		FirebaseID: firebaseID,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return "", paymentErrors.ErrorUserNotFound
		}
		c.logger.Error("get user by firebase id failed", zap.Error(err))
		return "", fmt.Errorf("user-service: GetUserByFirebaseID failed: %w", err)
	}

	return resp.UserID, nil
}
