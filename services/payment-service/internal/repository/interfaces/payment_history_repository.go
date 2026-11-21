package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
)

// PaymentHistoryRepository définit les opérations sur l'historique des paiements.
type PaymentHistoryRepository interface {
	CreateHistoryEntry(ctx context.Context, entry *domain.PaymentStatusHistory) error
	// ListByPaymentID retourne l'historique d'un paiement, du plus ancien au plus récent.
	ListByPaymentID(ctx context.Context, paymentID string) ([]*domain.PaymentStatusHistory, error)
	// ListByBookingID retourne l'historique de tous les paiements d'une réservation
	// (une réservation peut avoir plusieurs paiements, ex. un échec puis une nouvelle tentative).
	ListByBookingID(ctx context.Context, bookingID string) ([]*domain.PaymentStatusHistory, error)
}
