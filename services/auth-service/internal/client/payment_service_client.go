package client

import (
	"context"
	"fmt"

	"github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	paymentpb "github.com/Kpeewu/tissi-mah/services/auth-service/proto/gen/paymentpb"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// PaymentClient définit les opérations inter-service vers payment-service.
type PaymentClient interface {
	CheckDeletionEligibility(ctx context.Context, userID string) (bool, string, error)
	AnonymizeUserData(ctx context.Context, userID string, bookingIDs []string) error
	Close() error
}

type paymentServiceClient struct {
	conn       *grpc.ClientConn
	grpcClient paymentpb.PaymentServiceClient
	logger     *zap.Logger
}

func NewPaymentServiceClient(address string, logger *zap.Logger) (PaymentClient, error) {
	conn, err := grpcutil.NewClientConn(address, logger)
	if err != nil {
		return nil, fmt.Errorf("payment-service: failed to connect to %s: %w", address, err)
	}
	return &paymentServiceClient{
		conn:       conn,
		grpcClient: paymentpb.NewPaymentServiceClient(conn),
		logger:     logger,
	}, nil
}

func (c *paymentServiceClient) Close() error { return c.conn.Close() }

func (c *paymentServiceClient) CheckDeletionEligibility(ctx context.Context, userID string) (bool, string, error) {
	resp, err := c.grpcClient.CheckDeletionEligibility(ctx, &paymentpb.CheckDeletionEligibilityRequest{UserId: userID})
	if err != nil {
		c.logger.Error("payment: CheckDeletionEligibility failed", zap.Error(err))
		return false, "", fmt.Errorf("payment-service: CheckDeletionEligibility: %w", err)
	}
	if resp.ErrorMessage != "" {
		return false, "", fmt.Errorf("payment-service: %s", resp.ErrorMessage)
	}
	return resp.CanDelete, resp.BlockingReason, nil
}

func (c *paymentServiceClient) AnonymizeUserData(ctx context.Context, userID string, bookingIDs []string) error {
	resp, err := c.grpcClient.AnonymizeUserData(ctx, &paymentpb.AnonymizeUserDataRequest{
		UserId:     userID,
		BookingIds: bookingIDs,
	})
	if err != nil {
		c.logger.Error("payment: AnonymizeUserData failed", zap.Error(err))
		return fmt.Errorf("payment-service: AnonymizeUserData: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("payment-service: AnonymizeUserData: %s", resp.ErrorMessage)
	}
	return nil
}
