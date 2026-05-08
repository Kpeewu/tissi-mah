package client

import (
	"context"
	"fmt"

	userpb "github.com/Kpeewu/tissi-mah/services/user-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// UserClient résout un Firebase UID (injecté par l'api-gateway dans le
// header x-firebase-uid) en UserID interne. Toutes les tables chat (sender_id,
// driver_id, passenger_id) utilisent l'UserID interne, jamais le Firebase UID.
type UserClient interface {
	GetUserIDByFirebaseID(ctx context.Context, firebaseUID string) (string, error)
	Close() error
}

type userClientImpl struct {
	grpcClient userpb.UserServiceClient
	conn       *grpc.ClientConn
	logger     *zap.Logger
}

func NewUserClient(addr string, logger *zap.Logger) (UserClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("chat: dial user-service %s: %w", addr, err)
	}
	return &userClientImpl{
		grpcClient: userpb.NewUserServiceClient(conn),
		conn:       conn,
		logger:     logger,
	}, nil
}

func (c *userClientImpl) GetUserIDByFirebaseID(ctx context.Context, firebaseUID string) (string, error) {
	resp, err := c.grpcClient.GetUserByFirebaseID(ctx, &userpb.GetUserByFirebaseIDRequest{
		FirebaseID: firebaseUID,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return "", fmt.Errorf("user-service: user not found for firebaseUID %s", firebaseUID)
		}
		return "", fmt.Errorf("user-service: GetUserByFirebaseID: %w", err)
	}
	return resp.UserID, nil
}

func (c *userClientImpl) Close() error { return c.conn.Close() }
