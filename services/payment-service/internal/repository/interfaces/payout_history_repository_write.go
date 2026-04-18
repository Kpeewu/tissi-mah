package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
)

// PayoutHistoryRepositoryWrite définit les opérations d'écriture sur l'historique des payouts.
type PayoutHistoryRepositoryWrite interface {
	CreateHistoryEntry(ctx context.Context, entry *domain.PayoutStatusHistory) error
}
