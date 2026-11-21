package client

import (
	"context"
	"fmt"

	"github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	tripspb "github.com/Kpeewu/tissi-mah/services/auth-service/proto/gen/tripspb"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// TripsClient définit les opérations inter-service vers trips-service.
type TripsClient interface {
	CheckDeletionEligibility(ctx context.Context, userID string) (bool, string, error)
	AnonymizeUserData(ctx context.Context, userID string) error
	Close() error
}

type tripsServiceClient struct {
	conn       *grpc.ClientConn
	grpcClient tripspb.TripServiceClient
	logger     *zap.Logger
}

func NewTripsServiceClient(address string, logger *zap.Logger) (TripsClient, error) {
	conn, err := grpcutil.NewClientConn(address, logger)
	if err != nil {
		return nil, fmt.Errorf("trips-service: failed to connect to %s: %w", address, err)
	}
	return &tripsServiceClient{
		conn:       conn,
		grpcClient: tripspb.NewTripServiceClient(conn),
		logger:     logger,
	}, nil
}

func (c *tripsServiceClient) Close() error { return c.conn.Close() }

func (c *tripsServiceClient) CheckDeletionEligibility(ctx context.Context, userID string) (bool, string, error) {
	resp, err := c.grpcClient.CheckDeletionEligibility(ctx, &tripspb.CheckDeletionEligibilityRequest{UserId: userID})
	if err != nil {
		c.logger.Error("trips: CheckDeletionEligibility failed", zap.Error(err))
		return false, "", fmt.Errorf("trips-service: CheckDeletionEligibility: %w", err)
	}
	if resp.ErrorMessage != "" {
		return false, "", fmt.Errorf("trips-service: %s", resp.ErrorMessage)
	}
	return resp.CanDelete, resp.BlockingReason, nil
}

func (c *tripsServiceClient) AnonymizeUserData(ctx context.Context, userID string) error {
	resp, err := c.grpcClient.AnonymizeUserData(ctx, &tripspb.AnonymizeUserDataRequest{UserId: userID})
	if err != nil {
		c.logger.Error("trips: AnonymizeUserData failed", zap.Error(err))
		return fmt.Errorf("trips-service: AnonymizeUserData: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("trips-service: AnonymizeUserData: %s", resp.ErrorMessage)
	}
	return nil
}
