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
	// SetRefundProviderReference enregistre l'ID du payout FedaPay d'un refund processing, avant son démarrage.
	SetRefundProviderReference(ctx context.Context, refundID string, paymentProviderReference string) error
	// MarkRefundCompleted finalise le refund après confirmation FedaPay (processing → completed).
	// Retourne ErrorRefundAlreadyProcessed si le refund n'était plus 'processing'.
	MarkRefundCompleted(ctx context.Context, refundID string, paymentProviderReference string) error
	// MarkRefundPayoutFailed enregistre l'échec FedaPay d'un virement démarré : processing → pending,
	// retry_count incrémenté. Retourne ErrorRefundAlreadyProcessed si le refund n'était plus 'processing'.
	MarkRefundPayoutFailed(ctx context.Context, refundID string, reason string) error
	// MarkRefundFailed enregistre un échec. Si incrementRetry vaut true, retry_count est incrémenté
	// et last_retry_at mis à now ; le statut repasse à 'pending' pour que le worker puisse réessayer.
	// Si incrementRetry vaut false, le refund reste en 'failed' (échec non récupérable).
	MarkRefundFailed(ctx context.Context, refundID string, reason string, incrementRetry bool) error
	// SetDriverPayoutProviderReference enregistre l'ID FedaPay du virement chauffeur avant son démarrage.
	SetDriverPayoutProviderReference(ctx context.Context, refundID, providerRef string) error
	// MarkDriverPayoutProcessing passe driver_payout_status pending → processing (verrouillage optimiste).
	// Retourne ErrorRefundAlreadyProcessed si le statut n'était pas 'pending'.
	MarkDriverPayoutProcessing(ctx context.Context, refundID string) error
	// MarkDriverPayoutCompleted finalise le payout chauffeur après confirmation FedaPay (processing → completed).
	// Retourne ErrorRefundAlreadyProcessed si driver_payout_status n'était plus 'processing'.
	MarkDriverPayoutCompleted(ctx context.Context, refundID string) error
	// MarkDriverPayoutFailed enregistre l'échec du payout chauffeur.
	// incrementRetry=true → driver_payout_status repasse à 'pending', retry_count incrémenté.
	// incrementRetry=false → driver_payout_status = 'failed' (non récupérable).
	MarkDriverPayoutFailed(ctx context.Context, refundID, reason string, incrementRetry bool) error
}
