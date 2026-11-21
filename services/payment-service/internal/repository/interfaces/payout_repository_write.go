package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
)

// PayoutRepositoryWrite définit les opérations d'écriture sur les payouts.
type PayoutRepositoryWrite interface {
	CreatePayout(ctx context.Context, payout *domain.Payout) error
	UpdatePayoutStatus(ctx context.Context, payoutID string, status domain.PayoutStatus, providerReference string) error
	MarkPayoutFailed(ctx context.Context, payoutID string, reason string) error
	// ConfirmPayoutCompleted passe un payout processing en completed après confirmation FedaPay.
	// Retourne ErrorPayoutAlreadyProcessed si le payout n'était plus 'processing'.
	ConfirmPayoutCompleted(ctx context.Context, payoutID string) error
	// FailProcessingPayout enregistre l'échec FedaPay d'un payout processing et remet les paiements
	// paidOut du trajet en released (nouvelle tentative au batch suivant), dans une même transaction.
	// Retourne les paiements remis en released, ou ErrorPayoutAlreadyProcessed si le payout
	// n'était plus 'processing'.
	FailProcessingPayout(ctx context.Context, payoutID string, tripID string, reason string) ([]string, error)
	IncrementRetryCount(ctx context.Context, payoutID string) error
	CreateBatch(ctx context.Context, batch *domain.PayoutBatch) error
	UpdateBatch(ctx context.Context, batch *domain.PayoutBatch) error
	// MarkPaymentsAsPaidOut passe les paiements released du trip en paidOut et retourne leurs identifiants.
	MarkPaymentsAsPaidOut(ctx context.Context, tripID string) ([]string, error)

	// AnonymizeDriverRefs pseudonymise driver_id, payout_destination et destination_name
	// dans payouts pour l'utilisateur donné.
	AnonymizeDriverRefs(ctx context.Context, driverID string) error
}
