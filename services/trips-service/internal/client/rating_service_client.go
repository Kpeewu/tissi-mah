package client

import (
	"context"
	"fmt"

	"github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	ratingpb "github.com/Kpeewu/tissi-mah/services/rating-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// RatingServiceClient est le client gRPC vers rating-service.
type RatingServiceClient struct {
	conn       *grpc.ClientConn
	grpcClient ratingpb.RatingServiceClient
	logger     *zap.Logger
}

// NewRatingServiceClient établit la connexion gRPC vers rating-service.
func NewRatingServiceClient(address string, logger *zap.Logger) (*RatingServiceClient, error) {
	logger.Debug("connecting to rating-service", zap.String("address", address))

	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(grpcutil.ClientTransportCredentials()),
	)
	if err != nil {
		logger.Error("failed to connect to rating-service", zap.Error(err), zap.String("address", address))
		return nil, fmt.Errorf("rating-service: failed to connect to %s: %w", address, err)
	}

	return &RatingServiceClient{
		conn:       conn,
		grpcClient: ratingpb.NewRatingServiceClient(conn),
		logger:     logger,
	}, nil
}

// Close libère la connexion gRPC.
func (c *RatingServiceClient) Close() error {
	return c.conn.Close()
}

// GetDriverRatingAverage retourne la note moyenne du conducteur.
// Retourne 0 si aucune note ou si le service est indisponible.
func (c *RatingServiceClient) GetDriverRatingAverage(ctx context.Context, driverID string) (float64, error) {
	c.logger.Debug("client: GetDriverRatingAverage called", zap.String("driverID", driverID))

	resp, err := c.grpcClient.GetUserRatingsAverage(ctx, &ratingpb.GetUserRatingsAverageRequest{
		UserRatedId: driverID,
	})
	if err != nil {
		c.logger.Error("client: GetUserRatingsAverage failed", zap.Error(err), zap.String("driverID", driverID))
		return 0, fmt.Errorf("rating-service: GetUserRatingsAverage failed: %w", err)
	}

	if resp.ErrorMessage != "" {
		c.logger.Warn("client: GetUserRatingsAverage returned error", zap.String("error", resp.ErrorMessage))
		return 0, nil
	}

	return resp.Average, nil
}
