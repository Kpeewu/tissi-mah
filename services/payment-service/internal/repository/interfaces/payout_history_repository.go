package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
)

// PayoutHistoryRepository définit les opérations sur l'historique des payouts.
type PayoutHistoryRepository interface {
	CreateHistoryEntry(ctx context.Context, entry *domain.PayoutStatusHistory) error
	// ListByPayoutID retourne l'historique d'un payout, du plus ancien au plus récent.
	ListByPayoutID(ctx context.Context, payoutID string) ([]*domain.PayoutStatusHistory, error)
}
