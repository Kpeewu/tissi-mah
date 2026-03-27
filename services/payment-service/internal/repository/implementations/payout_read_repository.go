package implementations

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/payment-service/internal/repository/interfaces"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
)

const payoutsPageSize = 20

type payoutReadRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewPayoutReadRepository(pool *pgxpool.Pool, logger *zap.Logger) repoInterfaces.PayoutRepositoryRead {
	return &payoutReadRepository{pool: pool, logger: logger.Named("payout-read-repo")}
}

func (r *payoutReadRepository) GetByID(ctx context.Context, payoutID string) (*domain.Payout, error) {
	query := `SELECT payout_id, payout_reference, driver_id, trip_id,
		gross_amount, platform_fee, net_amount, payout_method, payout_destination,
		destination_name, status, scheduled_at, completed_at, failed_at, cancelled_at,
		payment_provider, payment_provider_reference, failure_reason,
		retry_count, last_retry_at, created_at, updated_at
		FROM payouts WHERE payout_id = $1`

	return r.scanPayout(ctx, query, payoutID)
}

func (r *payoutReadRepository) GetByTripID(ctx context.Context, tripID string) (*domain.Payout, error) {
	query := `SELECT payout_id, payout_reference, driver_id, trip_id,
		gross_amount, platform_fee, net_amount, payout_method, payout_destination,
		destination_name, status, scheduled_at, completed_at, failed_at, cancelled_at,
		payment_provider, payment_provider_reference, failure_reason,
		retry_count, last_retry_at, created_at, updated_at
		FROM payouts WHERE trip_id = $1 ORDER BY created_at DESC LIMIT 1`

	return r.scanPayout(ctx, query, tripID)
}

func (r *payoutReadRepository) GetDriverPayouts(ctx context.Context, driverID string, pageIndex int) ([]*domain.Payout, error) {
	offset := pageIndex * payoutsPageSize
	query := `SELECT payout_id, payout_reference, driver_id, trip_id,
		gross_amount, platform_fee, net_amount, payout_method, payout_destination,
		destination_name, status, scheduled_at, completed_at, failed_at, cancelled_at,
		payment_provider, payment_provider_reference, failure_reason,
		retry_count, last_retry_at, created_at, updated_at
		FROM payouts WHERE driver_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, query, driverID, payoutsPageSize, offset)
	if err != nil {
		r.logger.Error("get driver payouts failed", zap.Error(err))
		return nil, paymentErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	var payouts []*domain.Payout
	for rows.Next() {
		p := &domain.Payout{}
		if err := rows.Scan(
			&p.PayoutID, &p.PayoutReference, &p.DriverID, &p.TripID,
			&p.GrossAmount, &p.PlatformFee, &p.NetAmount, &p.PayoutMethod, &p.PayoutDestination,
			&p.DestinationName, &p.Status, &p.ScheduledAt, &p.CompletedAt, &p.FailedAt, &p.CancelledAt,
			&p.PaymentProvider, &p.PaymentProviderReference, &p.FailureReason,
			&p.RetryCount, &p.LastRetryAt, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			r.logger.Error("scan payout failed", zap.Error(err))
			return nil, paymentErrors.ErrorDataRetrievalFailed
		}
		payouts = append(payouts, p)
	}

	return payouts, nil
}

func (r *payoutReadRepository) GetBatchByID(ctx context.Context, batchID string) (*domain.PayoutBatch, error) {
	query := `SELECT batch_id, status, total_payouts, successful_count, failed_count,
		total_amount, started_at, completed_at, created_at
		FROM payout_batches WHERE batch_id = $1`

	var b domain.PayoutBatch
	err := r.pool.QueryRow(ctx, query, batchID).Scan(
		&b.BatchID, &b.Status, &b.TotalPayouts, &b.SuccessfulCount, &b.FailedCount,
		&b.TotalAmount, &b.StartedAt, &b.CompletedAt, &b.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, paymentErrors.ErrorPayoutNotFound
		}
		return nil, paymentErrors.ErrorDataRetrievalFailed
	}
	return &b, nil
}

// GetTripsReadyForPayout retourne les trip_id dont tous les paiements sont en status released
// et pour lesquels aucun payout n'existe encore.
func (r *payoutReadRepository) GetTripsReadyForPayout(ctx context.Context) ([]string, error) {
	query := `SELECT DISTINCT p.trip_id
		FROM payments p
		WHERE p.status = 'released'
		AND NOT EXISTS (
			SELECT 1 FROM payments p2
			WHERE p2.trip_id = p.trip_id AND p2.status NOT IN ('released', 'paidOut', 'refunded')
		)
		AND NOT EXISTS (
			SELECT 1 FROM payouts po
			WHERE po.trip_id = p.trip_id AND po.status NOT IN ('failed', 'cancelled')
		)`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		r.logger.Error("get trips ready for payout failed", zap.Error(err))
		return nil, paymentErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	var tripIDs []string
	for rows.Next() {
		var tripID string
		if err := rows.Scan(&tripID); err != nil {
			return nil, paymentErrors.ErrorDataRetrievalFailed
		}
		tripIDs = append(tripIDs, tripID)
	}

	return tripIDs, nil
}

// GetReleasedPaymentsForTrip retourne les paiements released d'un trip.
func (r *payoutReadRepository) GetReleasedPaymentsForTrip(ctx context.Context, tripID string) ([]*domain.Payment, error) {
	query := `SELECT payment_id, booking_id, trip_id, amount, payment_method, payment_provider,
		status, external_transaction_id, payment_reference,
		created_at, completed_at, failed_at, failure_reason, metadata, updated_at
		FROM payments WHERE trip_id = $1 AND status = 'released'`

	rows, err := r.pool.Query(ctx, query, tripID)
	if err != nil {
		r.logger.Error("get released payments for trip failed", zap.Error(err))
		return nil, paymentErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	var payments []*domain.Payment
	for rows.Next() {
		p := &domain.Payment{}
		if err := rows.Scan(
			&p.PaymentID, &p.BookingID, &p.TripID, &p.Amount,
			&p.PaymentMethod, &p.PaymentProvider,
			&p.Status, &p.ExternalTransactionID, &p.PaymentReference,
			&p.CreatedAt, &p.CompletedAt, &p.FailedAt, &p.FailureReason,
			&p.Metadata, &p.UpdatedAt,
		); err != nil {
			return nil, paymentErrors.ErrorDataRetrievalFailed
		}
		payments = append(payments, p)
	}

	return payments, nil
}

func (r *payoutReadRepository) scanPayout(ctx context.Context, query string, arg interface{}) (*domain.Payout, error) {
	p := &domain.Payout{}
	err := r.pool.QueryRow(ctx, query, arg).Scan(
		&p.PayoutID, &p.PayoutReference, &p.DriverID, &p.TripID,
		&p.GrossAmount, &p.PlatformFee, &p.NetAmount, &p.PayoutMethod, &p.PayoutDestination,
		&p.DestinationName, &p.Status, &p.ScheduledAt, &p.CompletedAt, &p.FailedAt, &p.CancelledAt,
		&p.PaymentProvider, &p.PaymentProviderReference, &p.FailureReason,
		&p.RetryCount, &p.LastRetryAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, paymentErrors.ErrorPayoutNotFound
		}
		r.logger.Error("scan payout failed", zap.Error(err))
		return nil, paymentErrors.ErrorDataRetrievalFailed
	}
	return p, nil
}
