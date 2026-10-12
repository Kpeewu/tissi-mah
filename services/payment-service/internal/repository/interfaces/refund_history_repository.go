package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
)

// RefundHistoryRepository définit les opérations sur l'historique des remboursements.
type RefundHistoryRepository interface {
	CreateHistoryEntry(ctx context.Context, entry *domain.RefundStatusHistory) error
	// ListByRefundID retourne l'historique d'un remboursement, du plus ancien au plus récent.
	ListByRefundID(ctx context.Context, refundID string) ([]*domain.RefundStatusHistory, error)
	// ListByBookingID retourne l'historique des remboursements d'une réservation.
	ListByBookingID(ctx context.Context, bookingID string) ([]*domain.RefundStatusHistory, error)
}
