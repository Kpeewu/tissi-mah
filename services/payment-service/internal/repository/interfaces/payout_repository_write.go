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
	IncrementRetryCount(ctx context.Context, payoutID string) error
	CreateBatch(ctx context.Context, batch *domain.PayoutBatch) error
	UpdateBatch(ctx context.Context, batch *domain.PayoutBatch) error
	MarkPaymentsAsPaidOut(ctx context.Context, tripID string) error

	// AnonymizeDriverRefs pseudonymise driver_id, payout_destination et destination_name
	// dans payouts pour l'utilisateur donné.
	AnonymizeDriverRefs(ctx context.Context, driverID string) error
}
