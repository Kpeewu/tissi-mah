package client

import (
	"context"
	"fmt"

	"github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	bookingpb "github.com/Kpeewu/tissi-mah/services/booking-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// BookingServiceClient est le client gRPC vers booking-service.
type BookingServiceClient struct {
	conn       *grpc.ClientConn
	grpcClient bookingpb.BookingServiceClient
	logger     *zap.Logger
}

// NewBookingServiceClient établit la connexion gRPC vers booking-service.
func NewBookingServiceClient(address string, logger *zap.Logger) (*BookingServiceClient, error) {
	logger.Debug("connecting to booking-service", zap.String("address", address))

	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(grpcutil.ClientTransportCredentials()),
	)
	if err != nil {
		logger.Error("failed to connect to booking-service", zap.Error(err), zap.String("address", address))
		return nil, fmt.Errorf("booking-service: failed to connect to %s: %w", address, err)
	}

	return &BookingServiceClient{
		conn:       conn,
		grpcClient: bookingpb.NewBookingServiceClient(conn),
		logger:     logger,
	}, nil
}

// Close libère la connexion gRPC.
func (c *BookingServiceClient) Close() error {
	return c.conn.Close()
}

// StartBookingsForWaypoint démarre les réservations approved d'un waypoint.
func (c *BookingServiceClient) StartBookingsForWaypoint(ctx context.Context, tripID, waypointID string) error {
	c.logger.Debug("client: StartBookingsForWaypoint called",
		zap.String("tripID", tripID), zap.String("waypointID", waypointID))

	_, err := c.grpcClient.StartBookingsForWaypoint(ctx, &bookingpb.StartBookingsForWaypointRequest{
		TripId:     tripID,
		WaypointId: waypointID,
	})
	if err != nil {
		c.logger.Warn("client: StartBookingsForWaypoint failed (non-blocking)",
			zap.Error(err), zap.String("tripID", tripID), zap.String("waypointID", waypointID))
		return err
	}

	return nil
}

// CompleteBookingsForWaypoint complète les réservations inProgress d'un waypoint.
func (c *BookingServiceClient) CompleteBookingsForWaypoint(ctx context.Context, tripID, waypointID string) error {
	c.logger.Debug("client: CompleteBookingsForWaypoint called",
		zap.String("tripID", tripID), zap.String("waypointID", waypointID))

	_, err := c.grpcClient.CompleteBookingsForWaypoint(ctx, &bookingpb.CompleteBookingsForWaypointRequest{
		TripId:     tripID,
		WaypointId: waypointID,
	})
	if err != nil {
		c.logger.Warn("client: CompleteBookingsForWaypoint failed (non-blocking)",
			zap.Error(err), zap.String("tripID", tripID), zap.String("waypointID", waypointID))
		return err
	}

	return nil
}

// CancelBookingsForWaypoint annule les réservations actives d'un waypoint supprimé.
func (c *BookingServiceClient) CancelBookingsForWaypoint(ctx context.Context, tripID, waypointID string) error {
	c.logger.Debug("client: CancelBookingsForWaypoint called",
		zap.String("tripID", tripID), zap.String("waypointID", waypointID))

	_, err := c.grpcClient.CancelBookingsForWaypoint(ctx, &bookingpb.CancelBookingsForWaypointRequest{
		TripId:     tripID,
		WaypointId: waypointID,
	})
	if err != nil {
		c.logger.Warn("client: CancelBookingsForWaypoint failed (non-blocking)",
			zap.Error(err), zap.String("tripID", tripID), zap.String("waypointID", waypointID))
		return err
	}

	return nil
}

// CancelBookingsForTrip annule toutes les réservations actives d'un trajet annulé.
func (c *BookingServiceClient) CancelBookingsForTrip(ctx context.Context, tripID string) error {
	c.logger.Debug("client: CancelBookingsForTrip called", zap.String("tripID", tripID))

	_, err := c.grpcClient.CancelBookingsForTrip(ctx, &bookingpb.CancelBookingsForTripRequest{
		TripId: tripID,
	})
	if err != nil {
		c.logger.Warn("client: CancelBookingsForTrip failed (non-blocking)",
			zap.Error(err), zap.String("tripID", tripID))
		return err
	}

	return nil
}

// GetPassengerIDsForTrip retourne les IDs des passagers avec une réservation active sur un trajet.
func (c *BookingServiceClient) GetPassengerIDsForTrip(ctx context.Context, tripID string) ([]string, error) {
	c.logger.Debug("client: GetPassengerIDsForTrip called", zap.String("tripID", tripID))

	resp, err := c.grpcClient.GetActivePassengerIDsForTrip(ctx, &bookingpb.GetActivePassengerIDsForTripRequest{
		TripID: tripID,
	})
	if err != nil {
		c.logger.Error("client: GetPassengerIDsForTrip failed", zap.Error(err), zap.String("tripID", tripID))
		return nil, err
	}

	return resp.PassengerIDs, nil
}

// GetDriverTripBookings retourne les réservations d'un trajet pour le conducteur.
func (c *BookingServiceClient) GetDriverTripBookings(ctx context.Context, driverID, tripID string) ([]BookingPreview, error) {
	c.logger.Debug("client: GetDriverTripBookings called",
		zap.String("driverID", driverID), zap.String("tripID", tripID))

	resp, err := c.grpcClient.GetDriverTripBookings(ctx, &bookingpb.GetDriverTripBookingsRequest{
		DriverId: driverID,
		TripId:   tripID,
		Index:    0,
	})
	if err != nil {
		c.logger.Error("client: GetDriverTripBookings failed",
			zap.Error(err), zap.String("tripID", tripID))
		return nil, err
	}

	bookings := make([]BookingPreview, 0, len(resp.Bookings))
	for _, b := range resp.Bookings {
		bookings = append(bookings, BookingPreview{
			BookingID:           b.BookingId,
			BookingReference:    b.BookingReference,
			TripID:              b.TripId,
			Status:              b.Status,
			SeatsBooked:         int(b.SeatsBooked),
			TotalAmount:         int(b.TotalAmount),
			PickupLocationName:  b.PickupLocationName,
			DropoffLocationName: b.DropoffLocationName,
			DepartureDate:       b.DepartureDate,
			DepartureTime:       b.DepartureTime,
		})
	}

	return bookings, nil
}
