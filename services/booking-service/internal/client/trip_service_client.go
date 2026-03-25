package client

import (
	"context"
	"fmt"

	trippb "github.com/Kpeewu/tissi-mah/services/trips-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// TripServiceClient est le client gRPC vers trips-service.
type TripServiceClient struct {
	conn       *grpc.ClientConn
	grpcClient trippb.TripServiceClient
	logger     *zap.Logger
}

// NewTripServiceClient établit la connexion gRPC vers trips-service.
func NewTripServiceClient(address string, logger *zap.Logger) (*TripServiceClient, error) {
	logger.Debug("connecting to trips-service", zap.String("address", address))

	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		logger.Error("failed to connect to trips-service", zap.Error(err), zap.String("address", address))
		return nil, fmt.Errorf("trips-service: failed to connect to %s: %w", address, err)
	}

	return &TripServiceClient{
		conn:       conn,
		grpcClient: trippb.NewTripServiceClient(conn),
		logger:     logger,
	}, nil
}

// Close libère la connexion gRPC.
func (c *TripServiceClient) Close() error {
	return c.conn.Close()
}

// GetTripDetails retourne les détails d'un trajet.
func (c *TripServiceClient) GetTripDetails(ctx context.Context, tripID string) (*TripDetails, error) {
	c.logger.Debug("client: GetTripDetails called", zap.String("tripID", tripID))

	resp, err := c.grpcClient.GetTripByID(ctx, &trippb.GetTripByIDRequest{
		TripId: tripID,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return nil, nil
		}
		c.logger.Error("client: GetTripByID failed", zap.Error(err), zap.String("tripID", tripID))
		return nil, fmt.Errorf("trips-service: GetTripByID failed: %w", err)
	}

	details := &TripDetails{
		TripID:             resp.TripId,
		DriverID:           resp.DriverId,
		Status:             resp.Status,
		AvailableSeats:     int(resp.AvailableSeats),
		TotalSeats:         int(resp.TotalSeats),
		PricePerSeat:       int(resp.PricePerSeat),
		AutoApproveEnabled: resp.AutoApproveEnabled,
	}

	for _, wp := range resp.Waypoints {
		details.Waypoints = append(details.Waypoints, TripWaypoint{
			WaypointID:              wp.WaypointId,
			WaypointType:            wp.WaypointType,
			LocationName:            wp.LocationName,
			City:                    wp.City,
			ScheduledPickupDatetime: wp.ScheduledPickupDatetime,
		})
	}

	return details, nil
}

// UpdateAvailableSeats met à jour le nombre de places disponibles.
func (c *TripServiceClient) UpdateAvailableSeats(ctx context.Context, tripID string, newAvailableSeats int) error {
	c.logger.Debug("client: UpdateAvailableSeats called", zap.String("tripID", tripID), zap.Int("newSeats", newAvailableSeats))

	_, err := c.grpcClient.UpdateAvailableSeats(ctx, &trippb.UpdateAvailableSeatsRequest{
		TripId:            tripID,
		NewAvailableSeats: int32(newAvailableSeats),
	})
	if err != nil {
		c.logger.Error("client: UpdateAvailableSeats failed", zap.Error(err), zap.String("tripID", tripID))
		return fmt.Errorf("trips-service: UpdateAvailableSeats failed: %w", err)
	}

	return nil
}
