package implementations

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/payment-service/internal/repository/interfaces"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
)

type payoutWriteRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewPayoutWriteRepository(pool *pgxpool.Pool, logger *zap.Logger) repoInterfaces.PayoutRepositoryWrite {
	return &payoutWriteRepository{pool: pool, logger: logger.Named("payout-write-repo")}
}

func (r *payoutWriteRepository) CreatePayout(ctx context.Context, payout *domain.Payout) error {
	query := `INSERT INTO payouts (
		payout_id, payout_reference, driver_id, trip_id,
		gross_amount, platform_fee, net_amount, payout_method, payout_destination,
		destination_name, status, scheduled_at, payment_provider, payment_provider_reference
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`

	_, err := r.pool.Exec(ctx, query,
		payout.PayoutID, payout.PayoutReference, payout.DriverID, payout.TripID,
		payout.GrossAmount, payout.PlatformFee, payout.NetAmount,
		payout.PayoutMethod, payout.PayoutDestination,
		payout.DestinationName, payout.Status, payout.ScheduledAt, payout.PaymentProvider,
		payout.PaymentProviderReference,
	)
	if err != nil {
		r.logger.Error("create payout failed", zap.Error(err), zap.String("payoutID", payout.PayoutID))
		return paymentErrors.ErrorInternalServer
	}
	return nil
}

func (r *payoutWriteRepository) UpdatePayoutStatus(ctx context.Context, payoutID string, status domain.PayoutStatus, providerReference string) error {
	var query string
	var args []interface{}

	switch status {
	case domain.PayoutStatusCompleted:
		now := time.Now().UTC()
		query = `UPDATE payouts SET status = $2, payment_provider_reference = $3, completed_at = $4 WHERE payout_id = $1`
		args = []interface{}{payoutID, status, providerReference, now}
	default:
		query = `UPDATE payouts SET status = $2, payment_provider_reference = COALESCE(NULLIF($3, ''), payment_provider_reference) WHERE payout_id = $1`
		args = []interface{}{payoutID, status, providerReference}
	}

	tag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		r.logger.Error("update payout status failed", zap.Error(err))
		return paymentErrors.ErrorInternalServer
	}
	if tag.RowsAffected() == 0 {
		return paymentErrors.ErrorPayoutNotFound
	}
	return nil
}

func (r *payoutWriteRepository) MarkPayoutFailed(ctx context.Context, payoutID string, reason string) error {
	now := time.Now().UTC()
	query := `UPDATE payouts SET status = 'failed', failed_at = $2, failure_reason = $3 WHERE payout_id = $1`

	tag, err := r.pool.Exec(ctx, query, payoutID, now, reason)
	if err != nil {
		r.logger.Error("mark payout failed", zap.Error(err))
		return paymentErrors.ErrorInternalServer
	}
	if tag.RowsAffected() == 0 {
		return paymentErrors.ErrorPayoutNotFound
	}
	return nil
}

func (r *payoutWriteRepository) IncrementRetryCount(ctx context.Context, payoutID string) error {
	now := time.Now().UTC()
	query := `UPDATE payouts SET retry_count = retry_count + 1, last_retry_at = $2 WHERE payout_id = $1`

	_, err := r.pool.Exec(ctx, query, payoutID, now)
	if err != nil {
		r.logger.Error("increment retry count failed", zap.Error(err))
		return paymentErrors.ErrorInternalServer
	}
	return nil
}

func (r *payoutWriteRepository) CreateBatch(ctx context.Context, batch *domain.PayoutBatch) error {
	query := `INSERT INTO payout_batches (batch_id, status, total_payouts, total_amount, started_at)
		VALUES ($1, $2, $3, $4, $5)`

	_, err := r.pool.Exec(ctx, query,
		batch.BatchID, batch.Status, batch.TotalPayouts, batch.TotalAmount, batch.StartedAt,
	)
	if err != nil {
		r.logger.Error("create batch failed", zap.Error(err))
		return paymentErrors.ErrorInternalServer
	}
	return nil
}

func (r *payoutWriteRepository) UpdateBatch(ctx context.Context, batch *domain.PayoutBatch) error {
	query := `UPDATE payout_batches SET status = $2, successful_count = $3, failed_count = $4, completed_at = $5
		WHERE batch_id = $1`

	_, err := r.pool.Exec(ctx, query,
		batch.BatchID, batch.Status, batch.SuccessfulCount, batch.FailedCount, batch.CompletedAt,
	)
	if err != nil {
		r.logger.Error("update batch failed", zap.Error(err))
		return paymentErrors.ErrorInternalServer
	}
	return nil
}

// MarkPaymentsAsPaidOut met à jour tous les paiements released d'un trip → paid_out.
func (r *payoutWriteRepository) MarkPaymentsAsPaidOut(ctx context.Context, tripID string) error {
	query := `UPDATE payments SET status = 'paidOut' WHERE trip_id = $1 AND status = 'released'`

	_, err := r.pool.Exec(ctx, query, tripID)
	if err != nil {
		r.logger.Error("mark payments as paid out failed", zap.Error(err))
		return paymentErrors.ErrorInternalServer
	}
	return nil
}

// AnonymizeDriverRefs pseudonymise les références du chauffeur dans payouts.
func (r *payoutWriteRepository) AnonymizeDriverRefs(ctx context.Context, driverID string) error {
	anon := "deleted_" + driverID[:8]
	_, err := r.pool.Exec(ctx,
		`UPDATE payouts
		 SET driver_id          = $1,
		     payout_destination = $1,
		     destination_name   = $1,
		     updated_at         = NOW()
		 WHERE driver_id = $2`,
		anon, driverID,
	)
	if err != nil {
		r.logger.Error("AnonymizeDriverRefs failed", zap.Error(err), zap.String("driverID", driverID))
		return paymentErrors.ErrorInternalServer
	}
	return nil
}
