package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
)

// RefundRepositoryWrite définit les opérations d'écriture sur les remboursements.
type RefundRepositoryWrite interface {
	CreateRefund(ctx context.Context, refund *domain.Refund) error
	UpdateRefundStatus(ctx context.Context, refundID string, status domain.RefundStatus) error
	// MarkRefundProcessing passe le statut pending → processing en verrouillage optimiste.
	// Retourne ErrorRefundAlreadyProcessed si le statut n'était pas 'pending'.
	MarkRefundProcessing(ctx context.Context, refundID string) error
	// MarkRefundCompleted finalise le refund après succès FedaPay (processing → completed).
	MarkRefundCompleted(ctx context.Context, refundID string, paymentProviderReference string) error
	// MarkRefundFailed enregistre un échec. Si incrementRetry vaut true, retry_count est incrémenté
	// et last_retry_at mis à now ; le statut repasse à 'pending' pour que le worker puisse réessayer.
	// Si incrementRetry vaut false, le refund reste en 'failed' (échec non récupérable).
	MarkRefundFailed(ctx context.Context, refundID string, reason string, incrementRetry bool) error
}
