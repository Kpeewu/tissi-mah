package client

import (
	"context"
	"fmt"

	"github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	ratingpb "github.com/Kpeewu/tissi-mah/services/rating-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

// GetUserRatingsAverage retourne la moyenne et le nombre total de notes d'un utilisateur.
// Retourne (0, 0, nil) si l'utilisateur n'a aucune note (codes.NotFound).
func (c *RatingServiceClient) GetUserRatingsAverage(ctx context.Context, userID string) (float64, int32, error) {
	c.logger.Debug("client: GetUserRatingsAverage called", zap.String("userID", userID))

	resp, err := c.grpcClient.GetUserRatingsAverage(ctx, &ratingpb.GetUserRatingsAverageRequest{
		UserRatedId: userID,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return 0, 0, nil
		}
		c.logger.Error("client: GetUserRatingsAverage failed", zap.Error(err), zap.String("userID", userID))
		return 0, 0, fmt.Errorf("rating-service: GetUserRatingsAverage failed: %w", err)
	}

	if resp.ErrorMessage != "" {
		c.logger.Warn("client: GetUserRatingsAverage returned error message",
			zap.String("userID", userID), zap.String("error", resp.ErrorMessage))
		return 0, 0, nil
	}

	return resp.Average, resp.TotalRatings, nil
}
