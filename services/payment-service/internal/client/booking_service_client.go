package client

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	bookingpb "github.com/Kpeewu/tissi-mah/services/booking-service/proto/gen"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
		grpc.WithTransportCredentials(grpcutil.ClientTransportCredentials()),
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

// ConfirmPayment confirme le paiement auprès de booking-service et retourne le nouveau statut de la réservation.
func (c *BookingServiceClient) ConfirmPayment(ctx context.Context, bookingID string, transactionID string) (string, error) {
	resp, err := c.grpcClient.ConfirmPayment(ctx, &bookingpb.ConfirmPaymentRequest{
		BookingId:     bookingID,
		TransactionId: transactionID,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.FailedPrecondition {
			switch {
			case strings.Contains(st.Message(), "ErrorNoSeatsAvailable"):
				return "", paymentErrors.ErrorNoSeatsAvailable
			case strings.Contains(st.Message(), "ErrorPaymentAlreadyConfirmed"):
				return "", paymentErrors.ErrorPaymentAlreadyProcessed
			case strings.Contains(st.Message(), "ErrorInvalidStatusTransition"):
				// Réservation annulée, rejetée, etc. : le paiement ne peut plus y être rattaché
				return "", paymentErrors.ErrorBookingNotConfirmable
			}
		}
		c.logger.Error("confirm payment failed", zap.Error(err), zap.String("bookingID", bookingID))
		return "", fmt.Errorf("booking-service: ConfirmPayment failed: %w", err)
	}
	if !resp.Success {
		return "", fmt.Errorf("booking-service: ConfirmPayment rejected: %s", resp.ErrorMessage)
	}

	return resp.Status, nil
}

func (c *BookingServiceClient) FailPayment(ctx context.Context, bookingID string, reason string) error {
	resp, err := c.grpcClient.FailPayment(ctx, &bookingpb.FailPaymentRequest{
		BookingId: bookingID,
		Reason:    reason,
	})
	if err != nil {
		c.logger.Error("fail payment failed", zap.Error(err), zap.String("bookingID", bookingID))
		return fmt.Errorf("booking-service: FailPayment failed: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("booking-service: FailPayment rejected: %s", resp.ErrorMessage)
	}

	return nil
}

func (c *BookingServiceClient) GetBookingDetails(ctx context.Context, bookingID string) (*BookingDetails, error) {
	resp, err := c.grpcClient.GetBookingDetailsInternal(ctx, &bookingpb.GetBookingDetailsRequest{
		BookingId: bookingID,
	})
	if err != nil {
		c.logger.Error("get booking details failed", zap.Error(err), zap.String("bookingID", bookingID))
		return nil, fmt.Errorf("booking-service: GetBookingDetails failed: %w", err)
	}

	b := resp.Booking
	return &BookingDetails{
		BookingID:        b.BookingId,
		BookingReference: b.BookingReference,
		TripID:           b.TripId,
		PassengerID:      b.PassengerId,
		DriverID:         b.DriverId,
		Status:           b.Status,
		PaymentMethod:    b.PaymentMethod,
		Subtotal:         int(b.Subtotal),
		TotalAmount:      int(b.TotalAmount),
		ServiceFee:       int(b.ServiceFee),
		CreatedAt:        parseRFC3339(b.CreatedAt),
	}, nil
}

// parseRFC3339 parse une date RFC3339 (zéro si vide ou invalide).
func parseRFC3339(value string) time.Time {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return t
}
