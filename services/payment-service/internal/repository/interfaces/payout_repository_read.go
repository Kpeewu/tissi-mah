package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
)

// PayoutRepositoryRead définit les opérations de lecture sur les payouts.
type PayoutRepositoryRead interface {
	GetByID(ctx context.Context, payoutID string) (*domain.Payout, error)
	GetByTripID(ctx context.Context, tripID string) (*domain.Payout, error)
	GetDriverPayouts(ctx context.Context, driverID string, pageIndex int) ([]*domain.Payout, error)
	GetBatchByID(ctx context.Context, batchID string) (*domain.PayoutBatch, error)
	GetTripsReadyForPayout(ctx context.Context) ([]string, error)
	GetReleasedPaymentsForTrip(ctx context.Context, tripID string) ([]*domain.Payment, error)
	IsTripReadyForPayout(ctx context.Context, tripID string) (bool, error)
	GetByProviderReference(ctx context.Context, providerReference string) (*domain.Payout, error)

	// HasActivePayout vérifie si un chauffeur a un payout en attente (pending/scheduled/processing).
	HasActivePayout(ctx context.Context, driverID string) (bool, error)
}
