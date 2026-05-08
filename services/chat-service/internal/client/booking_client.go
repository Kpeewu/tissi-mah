package client

import (
	"context"
	"fmt"

	bookingpb "github.com/Kpeewu/tissi-mah/services/booking-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// BookingInfo contient les infos nécessaires du côté chat.
type BookingInfo struct {
	BookingID   string
	TripID      string
	DriverID    string
	PassengerID string
	Status      string // "approved", "pending", "cancelled", etc.
}

// BookingClient vérifie auprès du booking-service qu'une réservation existe
// et est dans un état autorisant le chat.
type BookingClient interface {
	GetBookingDetails(ctx context.Context, bookingID string) (*BookingInfo, error)
	Close() error
}

type bookingClientImpl struct {
	grpcClient bookingpb.BookingServiceClient
	conn       *grpc.ClientConn
	logger     *zap.Logger
}

func NewBookingClient(addr string, logger *zap.Logger) (BookingClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("chat: dial booking-service %s: %w", addr, err)
	}
	return &bookingClientImpl{
		grpcClient: bookingpb.NewBookingServiceClient(conn),
		conn:       conn,
		logger:     logger,
	}, nil
}

func (c *bookingClientImpl) GetBookingDetails(ctx context.Context, bookingID string) (*BookingInfo, error) {
	resp, err := c.grpcClient.GetBookingDetails(ctx, &bookingpb.GetBookingDetailsRequest{
		BookingId: bookingID,
	})
	if err != nil {
		return nil, fmt.Errorf("booking-service: GetBookingDetails: %w", err)
	}
	if resp.Booking == nil {
		return nil, nil
	}
	return &BookingInfo{
		BookingID:   resp.Booking.BookingId,
		TripID:      resp.Booking.TripId,
		DriverID:    resp.Booking.DriverId,
		PassengerID: resp.Booking.PassengerId,
		Status:      resp.Booking.Status,
	}, nil
}

func (c *bookingClientImpl) Close() error { return c.conn.Close() }
