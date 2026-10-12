package interfaces

import (
	"context"
	"time"

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
	// GetByProviderReference retourne le refund dont le virement FedaPay passager a cet ID.
	GetByProviderReference(ctx context.Context, providerReference string) (*domain.Refund, error)
	// GetProcessingRefundsToReconcile retourne les refunds processing avec un virement FedaPay
	// non modifiés depuis updatedBefore (confirmation à vérifier auprès de FedaPay).
	GetProcessingRefundsToReconcile(ctx context.Context, updatedBefore time.Time, limit int) ([]*domain.Refund, error)
	// GetPendingDriverRefundPayouts retourne les refunds avec driver_payout_status='pending',
	// triés par updated_at ASC (FIFO), limités à `limit` entrées.
	GetPendingDriverRefundPayouts(ctx context.Context, limit int) ([]*domain.Refund, error)
	// GetByDriverPayoutProviderReference retourne le refund dont le virement FedaPay chauffeur a cet ID.
	GetByDriverPayoutProviderReference(ctx context.Context, providerRef string) (*domain.Refund, error)
	// GetProcessingDriverRefundPayoutsToReconcile retourne les refunds avec driver_payout_status='processing'
	// non modifiés depuis updatedBefore.
	GetProcessingDriverRefundPayoutsToReconcile(ctx context.Context, updatedBefore time.Time, limit int) ([]*domain.Refund, error)
}
