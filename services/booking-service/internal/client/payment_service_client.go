package client

import (
	"context"
	"fmt"

	"github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	paymentpb "github.com/Kpeewu/tissi-mah/services/payment-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// PaymentServiceClient est le client gRPC vers payment-service.
type PaymentServiceClient struct {
	conn       *grpc.ClientConn
	grpcClient paymentpb.PaymentServiceClient
	logger     *zap.Logger
}

// NewPaymentServiceClient établit la connexion gRPC vers payment-service.
func NewPaymentServiceClient(address string, logger *zap.Logger) (*PaymentServiceClient, error) {
	logger.Debug("connecting to payment-service", zap.String("address", address))

	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(grpcutil.ClientTransportCredentials()),
	)
	if err != nil {
		logger.Error("failed to connect to payment-service", zap.Error(err), zap.String("address", address))
		return nil, fmt.Errorf("payment-service: failed to connect to %s: %w", address, err)
	}

	return &PaymentServiceClient{
		conn:       conn,
		grpcClient: paymentpb.NewPaymentServiceClient(conn),
		logger:     logger,
	}, nil
}

// Close libère la connexion gRPC.
func (c *PaymentServiceClient) Close() error {
	return c.conn.Close()
}

// RequestRefund demande un remboursement au payment-service.
func (c *PaymentServiceClient) RequestRefund(ctx context.Context, input *RefundInput) error {
	c.logger.Debug("client: RequestRefund called",
		zap.String("bookingID", input.BookingID),
		zap.String("reason", input.RefundReason),
	)

	resp, err := c.grpcClient.RequestRefund(ctx, &paymentpb.RequestRefundRequest{
		BookingId:         input.BookingID,
		RefundReason:      input.RefundReason,
		OriginalAmount:    int32(input.OriginalAmount),
		ServiceFee:        int32(input.ServiceFee),
		DepartureDatetime: input.DepartureDatetime,
		ApprovedAt:        input.ApprovedAt,
		CancelledAt:       input.CancelledAt,
	})
	if err != nil {
		c.logger.Error("client: RequestRefund failed", zap.Error(err), zap.String("bookingID", input.BookingID))
		return fmt.Errorf("payment-service: RequestRefund failed: %w", err)
	}

	if resp.ErrorMessage != "" {
		return fmt.Errorf("payment-service: RequestRefund error: %s", resp.ErrorMessage)
	}

	c.logger.Info("refund requested",
		zap.String("bookingID", input.BookingID),
		zap.String("refundID", resp.RefundId),
		zap.Int32("refundAmount", resp.RefundAmount),
	)

	return nil
}

// ReleasePayment libère un paiement held → released après le délai de contestation.
func (c *PaymentServiceClient) ReleasePayment(ctx context.Context, bookingID string) error {
	c.logger.Debug("client: ReleasePayment called", zap.String("bookingID", bookingID))

	resp, err := c.grpcClient.ReleasePayment(ctx, &paymentpb.ReleasePaymentRequest{
		BookingId: bookingID,
	})
	if err != nil {
		c.logger.Error("client: ReleasePayment failed", zap.Error(err), zap.String("bookingID", bookingID))
		return fmt.Errorf("payment-service: ReleasePayment failed: %w", err)
	}

	if resp.ErrorMessage != "" {
		return fmt.Errorf("payment-service: ReleasePayment error: %s", resp.ErrorMessage)
	}

	return nil
}

// GetPaymentStatusByBooking retourne le statut du dernier paiement d'une réservation ("" si aucun).
func (c *PaymentServiceClient) GetPaymentStatusByBooking(ctx context.Context, bookingID string) (string, error) {
	resp, err := c.grpcClient.GetPaymentByBooking(ctx, &paymentpb.GetPaymentByBookingRequest{
		BookingId: bookingID,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return "", nil
		}
		c.logger.Error("client: GetPaymentByBooking failed", zap.Error(err), zap.String("bookingID", bookingID))
		return "", fmt.Errorf("payment-service: GetPaymentByBooking failed: %w", err)
	}

	return resp.Status, nil
}

// CancelPaymentForBooking supprime la collecte FedaPay pending d'une réservation annulée.
func (c *PaymentServiceClient) CancelPaymentForBooking(ctx context.Context, bookingID string, reason string) (string, error) {
	resp, err := c.grpcClient.CancelPaymentForBooking(ctx, &paymentpb.CancelPaymentForBookingRequest{
		BookingId: bookingID,
		Reason:    reason,
	})
	if err != nil {
		c.logger.Error("client: CancelPaymentForBooking failed", zap.Error(err), zap.String("bookingID", bookingID))
		return "", fmt.Errorf("payment-service: CancelPaymentForBooking failed: %w", err)
	}

	return resp.Status, nil
}
