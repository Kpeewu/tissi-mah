package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
)

// RefundRepositoryRead définit les opérations de lecture sur les remboursements.
type RefundRepositoryRead interface {
	GetByID(ctx context.Context, refundID string) (*domain.Refund, error)
	GetByBookingID(ctx context.Context, bookingID string) (*domain.Refund, error)
	GetByPaymentID(ctx context.Context, paymentID string) (*domain.Refund, error)
	// GetPendingRefundsForPayout retourne les refunds status='pending' avec amount_to_passenger > 0,
	// triés par created_at ASC (FIFO), limités à `limit` entrées.
	GetPendingRefundsForPayout(ctx context.Context, limit int) ([]*domain.Refund, error)
}
