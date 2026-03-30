package client

import (
	"context"
	"fmt"

	bookingpb "github.com/Kpeewu/tissi-mah/services/booking-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// BookingServiceClient est le client gRPC vers booking-service.
type BookingServiceClient struct {
	conn       *grpc.ClientConn
	grpcClient bookingpb.BookingServiceClient
	logger     *zap.Logger
}

// NewBookingServiceClient établit la connexion gRPC vers booking-service.
func NewBookingServiceClient(address string, logger *zap.Logger) (*BookingServiceClient, error) {
	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("booking-service: failed to connect to %s: %w", address, err)
	}

	return &BookingServiceClient{
		conn:       conn,
		grpcClient: bookingpb.NewBookingServiceClient(conn),
		logger:     logger.Named("booking-client"),
	}, nil
}

func (c *BookingServiceClient) Close() error {
	return c.conn.Close()
}

func (c *BookingServiceClient) ConfirmPayment(ctx context.Context, bookingID string, transactionID string) error {
	resp, err := c.grpcClient.ConfirmPayment(ctx, &bookingpb.ConfirmPaymentRequest{
		BookingId:     bookingID,
		TransactionId: transactionID,
	})
	if err != nil {
		c.logger.Error("confirm payment failed", zap.Error(err), zap.String("bookingID", bookingID))
		return fmt.Errorf("booking-service: ConfirmPayment failed: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("booking-service: ConfirmPayment rejected: %s", resp.ErrorMessage)
	}

	return nil
}

func (c *BookingServiceClient) GetBookingDetails(ctx context.Context, bookingID string) (*BookingDetails, error) {
	resp, err := c.grpcClient.GetBookingDetails(ctx, &bookingpb.GetBookingDetailsRequest{
		BookingId: bookingID,
	})
	if err != nil {
		c.logger.Error("get booking details failed", zap.Error(err), zap.String("bookingID", bookingID))
		return nil, fmt.Errorf("booking-service: GetBookingDetails failed: %w", err)
	}

	b := resp.Booking
	return &BookingDetails{
		BookingID:   b.BookingId,
		TripID:      b.TripId,
		PassengerID: b.PassengerId,
		DriverID:    b.DriverId,
		Status:      b.Status,
		TotalAmount: int(b.TotalAmount),
		ServiceFee:  int(b.ServiceFee),
	}, nil
}
