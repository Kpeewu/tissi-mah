package client

import (
	"context"
	"fmt"

	"github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	bookingpb "github.com/Kpeewu/tissi-mah/services/auth-service/proto/gen/bookingpb"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// BookingClient définit les opérations inter-service vers booking-service.
type BookingClient interface {
	CheckDeletionEligibility(ctx context.Context, userID string) (bool, string, error)
	AnonymizeUserData(ctx context.Context, userID string) error
	GetPassengerBookingIDs(ctx context.Context, userID string) ([]string, error)
	Close() error
}

type bookingServiceClient struct {
	conn       *grpc.ClientConn
	grpcClient bookingpb.BookingServiceClient
	logger     *zap.Logger
}

func NewBookingServiceClient(address string, logger *zap.Logger) (BookingClient, error) {
	conn, err := grpcutil.NewClientConn(address, logger)
	if err != nil {
		return nil, fmt.Errorf("booking-service: failed to connect to %s: %w", address, err)
	}
	return &bookingServiceClient{
		conn:       conn,
		grpcClient: bookingpb.NewBookingServiceClient(conn),
		logger:     logger,
	}, nil
}

func (c *bookingServiceClient) Close() error { return c.conn.Close() }

func (c *bookingServiceClient) CheckDeletionEligibility(ctx context.Context, userID string) (bool, string, error) {
	resp, err := c.grpcClient.CheckDeletionEligibility(ctx, &bookingpb.CheckDeletionEligibilityRequest{UserId: userID})
	if err != nil {
		c.logger.Error("booking: CheckDeletionEligibility failed", zap.Error(err))
		return false, "", fmt.Errorf("booking-service: CheckDeletionEligibility: %w", err)
	}
	if resp.ErrorMessage != "" {
		return false, "", fmt.Errorf("booking-service: %s", resp.ErrorMessage)
	}
	return resp.CanDelete, resp.BlockingReason, nil
}

func (c *bookingServiceClient) AnonymizeUserData(ctx context.Context, userID string) error {
	resp, err := c.grpcClient.AnonymizeUserData(ctx, &bookingpb.AnonymizeUserDataRequest{UserId: userID})
	if err != nil {
		c.logger.Error("booking: AnonymizeUserData failed", zap.Error(err))
		return fmt.Errorf("booking-service: AnonymizeUserData: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("booking-service: AnonymizeUserData: %s", resp.ErrorMessage)
	}
	return nil
}

func (c *bookingServiceClient) GetPassengerBookingIDs(ctx context.Context, userID string) ([]string, error) {
	resp, err := c.grpcClient.GetPassengerBookingIDs(ctx, &bookingpb.GetPassengerBookingIDsRequest{UserId: userID})
	if err != nil {
		c.logger.Error("booking: GetPassengerBookingIDs failed", zap.Error(err))
		return nil, fmt.Errorf("booking-service: GetPassengerBookingIDs: %w", err)
	}
	return resp.BookingIds, nil
}
