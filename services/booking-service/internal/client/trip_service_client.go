package client

import (
	"context"
	"fmt"

	"github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	trippb "github.com/Kpeewu/tissi-mah/services/trips-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
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
		grpc.WithTransportCredentials(grpcutil.ClientTransportCredentials()),
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
		RoutePolyline:      resp.RoutePolyline,
	}

	for _, wp := range resp.Waypoints {
		details.Waypoints = append(details.Waypoints, TripWaypoint{
			WaypointID:              wp.WaypointId,
			WaypointType:            wp.WaypointType,
			SequencerOrder:          int(wp.SequencerOrder),
			LocationName:            wp.LocationName,
			City:                    wp.City,
			ScheduledPickupDatetime: wp.ScheduledPickupDatetime,
			PriceFromPrevious:       int(wp.PriceFromPrevious),
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

// IncrementLegBookedSeats incrémente/décrémente booked_seats sur les legs [fromOrder, toOrder).
func (c *TripServiceClient) IncrementLegBookedSeats(ctx context.Context, tripID string, fromOrder, toOrder, delta int) error {
	c.logger.Debug("client: IncrementLegBookedSeats called",
		zap.String("tripID", tripID),
		zap.Int("fromOrder", fromOrder),
		zap.Int("toOrder", toOrder),
		zap.Int("delta", delta),
	)

	_, err := c.grpcClient.IncrementLegBookedSeats(ctx, &trippb.IncrementLegBookedSeatsRequest{
		TripId:    tripID,
		FromOrder: int32(fromOrder),
		ToOrder:   int32(toOrder),
		Delta:     int32(delta),
	})
	if err != nil {
		c.logger.Error("client: IncrementLegBookedSeats failed", zap.Error(err), zap.String("tripID", tripID))
		return fmt.Errorf("trips-service: IncrementLegBookedSeats failed: %w", err)
	}

	return nil
}

// SyncLegBookedSeats force la valeur de booked_seats pour chaque leg (réconciliation).
func (c *TripServiceClient) SyncLegBookedSeats(ctx context.Context, tripID string, legs []LegBookedSeats) error {
	c.logger.Debug("client: SyncLegBookedSeats called", zap.String("tripID", tripID), zap.Int("legsCount", len(legs)))

	pbLegs := make([]*trippb.LegBookedSeatsEntry, len(legs))
	for i, l := range legs {
		pbLegs[i] = &trippb.LegBookedSeatsEntry{
			SequencerOrder: int32(l.SequencerOrder),
			BookedSeats:    int32(l.BookedSeats),
		}
	}

	_, err := c.grpcClient.SyncLegBookedSeats(ctx, &trippb.SyncLegBookedSeatsRequest{
		TripId: tripID,
		Legs:   pbLegs,
	})
	if err != nil {
		c.logger.Error("client: SyncLegBookedSeats failed", zap.Error(err), zap.String("tripID", tripID))
		return fmt.Errorf("trips-service: SyncLegBookedSeats failed: %w", err)
	}

	return nil
}
