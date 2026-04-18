package client

import (
	"context"
	"fmt"

	supportpb "github.com/Kpeewu/tissi-mah/services/support-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
)

// SupportServiceClient est le client gRPC vers support-service.
type SupportServiceClient struct {
	conn       *grpc.ClientConn
	grpcClient supportpb.SupportServiceClient
	logger     *zap.Logger
}

// NewSupportServiceClient établit la connexion gRPC vers support-service.
func NewSupportServiceClient(address string, logger *zap.Logger) (*SupportServiceClient, error) {
	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("support-service: failed to connect to %s: %w", address, err)
	}

	return &SupportServiceClient{
		conn:       conn,
		grpcClient: supportpb.NewSupportServiceClient(conn),
		logger:     logger.Named("support-client"),
	}, nil
}

func (c *SupportServiceClient) Close() error {
	return c.conn.Close()
}

func (c *SupportServiceClient) GetSupportUserByID(ctx context.Context, userID string) (*SupportUserInfo, error) {
	resp, err := c.grpcClient.GetSupportUserByID(ctx, &supportpb.GetSupportUserByIDRequest{
		UserId: userID,
	})
	if err != nil {
		st, _ := status.FromError(err)
		if st.Code() == codes.NotFound {
			return nil, paymentErrors.ErrorUserNotFound
		}
		c.logger.Error("get support user failed", zap.Error(err), zap.String("userID", userID))
		return nil, paymentErrors.ErrorSupportServiceUnavailable
	}

	return &SupportUserInfo{
		UserID:    resp.GetUserId(),
		FirstName: resp.GetFirstName(),
		LastName:  resp.GetLastName(),
		Role:      resp.GetRole(),
		IsActive:  resp.GetIsActive(),
	}, nil
}
