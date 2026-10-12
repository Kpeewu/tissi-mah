package implementations

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/payment-service/internal/repository/interfaces"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
)

type refundReadRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

const refundSelectColumns = `refund_id, refund_reference, payment_id, booking_id,
	refund_reason, refund_rule_applied, original_amount, refund_percentage,
	refund_amount, service_fee_refunded, amount_to_passenger, amount_to_driver,
	amount_to_platform, status, refund_method, processed_at, completed_at,
	estimated_completion, passenger_notified, notification_sent_at, notes,
	payout_destination, payment_provider, payment_provider_reference,
	failure_reason, retry_count, last_retry_at, updated_at,
	driver_payout_status, driver_payout_provider_ref, driver_payout_retry_count,
	driver_payout_last_retry_at, driver_payout_completed_at`

func NewRefundReadRepository(pool *pgxpool.Pool, logger *zap.Logger) repoInterfaces.RefundRepositoryRead {
	return &refundReadRepository{pool: pool, logger: logger.Named("refund-read-repo")}
}

func (r *refundReadRepository) GetByID(ctx context.Context, refundID string) (*domain.Refund, error) {
	return r.scanRefund(ctx, `SELECT `+refundSelectColumns+` FROM refunds WHERE refund_id = $1`, refundID)
}

func (r *refundReadRepository) GetByBookingID(ctx context.Context, bookingID string) (*domain.Refund, error) {
	return r.scanRefund(ctx, `SELECT `+refundSelectColumns+` FROM refunds WHERE booking_id = $1 ORDER BY updated_at DESC LIMIT 1`, bookingID)
}

func (r *refundReadRepository) GetByPaymentID(ctx context.Context, paymentID string) (*domain.Refund, error) {
	return r.scanRefund(ctx, `SELECT `+refundSelectColumns+` FROM refunds WHERE payment_id = $1`, paymentID)
}

func (r *refundReadRepository) GetPendingRefundsForPayout(ctx context.Context, limit int) ([]*domain.Refund, error) {
	if limit <= 0 {
		limit = 20
	}
	query := `SELECT ` + refundSelectColumns + `
		FROM refunds
		WHERE status = 'pending' AND amount_to_passenger > 0
		ORDER BY (SELECT created_at FROM payments p WHERE p.payment_id = refunds.payment_id) ASC, refund_id ASC
		LIMIT $1`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		r.logger.Error("get pending refunds for payout failed", zap.Error(err))
		return nil, paymentErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	var refunds []*domain.Refund
	for rows.Next() {
		var ref domain.Refund
		if err := scanRefundRow(rows, &ref); err != nil {
			r.logger.Error("scan pending refund failed", zap.Error(err))
			return nil, paymentErrors.ErrorDataRetrievalFailed
		}
		refunds = append(refunds, &ref)
	}
	if err := rows.Err(); err != nil {
		return nil, paymentErrors.ErrorDataRetrievalFailed
	}

	return refunds, nil
}

func (r *refundReadRepository) GetByProviderReference(ctx context.Context, providerReference string) (*domain.Refund, error) {
	return r.scanRefund(ctx, `SELECT `+refundSelectColumns+` FROM refunds WHERE payment_provider_reference = $1`, providerReference)
}

func (r *refundReadRepository) GetProcessingRefundsToReconcile(
	ctx context.Context, updatedBefore time.Time, limit int,
) ([]*domain.Refund, error) {
	query := `SELECT ` + refundSelectColumns + `
		FROM refunds
		WHERE status = 'processing' AND COALESCE(payment_provider_reference, '') <> '' AND updated_at < $1
		ORDER BY updated_at ASC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, query, updatedBefore, limit)
	if err != nil {
		r.logger.Error("get processing refunds to reconcile failed", zap.Error(err))
		return nil, paymentErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	var refunds []*domain.Refund
	for rows.Next() {
		var ref domain.Refund
		if err := scanRefundRow(rows, &ref); err != nil {
			r.logger.Error("scan processing refund failed", zap.Error(err))
			return nil, paymentErrors.ErrorDataRetrievalFailed
		}
		refunds = append(refunds, &ref)
	}
	if err := rows.Err(); err != nil {
		return nil, paymentErrors.ErrorDataRetrievalFailed
	}

	return refunds, nil
}

func (r *refundReadRepository) scanRefund(ctx context.Context, query string, arg interface{}) (*domain.Refund, error) {
	var ref domain.Refund
	row := r.pool.QueryRow(ctx, query, arg)
	if err := scanRefundRow(row, &ref); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, paymentErrors.ErrorRefundNotFound
		}
		r.logger.Error("scan refund failed", zap.Error(err))
		return nil, paymentErrors.ErrorDataRetrievalFailed
	}

	return &ref, nil
}

// rowScanner permet de partager le scan entre QueryRow et Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanRefundRow(row rowScanner, ref *domain.Refund) error {
	return row.Scan(
		&ref.RefundID, &ref.RefundReference, &ref.PaymentID, &ref.BookingID,
		&ref.RefundReason, &ref.RefundRuleApplied, &ref.OriginalAmount, &ref.RefundPercentage,
		&ref.RefundAmount, &ref.ServiceFeeRefunded, &ref.AmountToPassenger, &ref.AmountToDriver,
		&ref.AmountToPlatform, &ref.Status, &ref.RefundMethod, &ref.ProcessedAt, &ref.CompletedAt,
		&ref.EstimatedCompletion, &ref.PassengerNotified, &ref.NotificationSentAt, &ref.Notes,
		&ref.PayoutDestination, &ref.PaymentProvider, &ref.PaymentProviderReference,
		&ref.FailureReason, &ref.RetryCount, &ref.LastRetryAt, &ref.UpdatedAt,
		&ref.DriverPayoutStatus, &ref.DriverPayoutProviderRef, &ref.DriverPayoutRetryCount,
		&ref.DriverPayoutLastRetryAt, &ref.DriverPayoutCompletedAt,
	)
}

func (r *refundReadRepository) GetPendingDriverRefundPayouts(ctx context.Context, limit int) ([]*domain.Refund, error) {
	if limit <= 0 {
		limit = 20
	}
	query := `SELECT ` + refundSelectColumns + `
		FROM refunds
		WHERE amount_to_driver > 0 AND driver_payout_status = 'pending'
		ORDER BY updated_at ASC
		LIMIT $1`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		r.logger.Error("get pending driver refund payouts failed", zap.Error(err))
		return nil, paymentErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	var refunds []*domain.Refund
	for rows.Next() {
		var ref domain.Refund
		if err := scanRefundRow(rows, &ref); err != nil {
			r.logger.Error("scan pending driver refund payout failed", zap.Error(err))
			return nil, paymentErrors.ErrorDataRetrievalFailed
		}
		refunds = append(refunds, &ref)
	}
	if err := rows.Err(); err != nil {
		return nil, paymentErrors.ErrorDataRetrievalFailed
	}

	return refunds, nil
}

func (r *refundReadRepository) GetByDriverPayoutProviderReference(ctx context.Context, providerRef string) (*domain.Refund, error) {
	return r.scanRefund(ctx, `SELECT `+refundSelectColumns+` FROM refunds WHERE driver_payout_provider_ref = $1`, providerRef)
}

func (r *refundReadRepository) GetProcessingDriverRefundPayoutsToReconcile(
	ctx context.Context, updatedBefore time.Time, limit int,
) ([]*domain.Refund, error) {
	query := `SELECT ` + refundSelectColumns + `
		FROM refunds
		WHERE driver_payout_status = 'processing' AND COALESCE(driver_payout_provider_ref, '') <> '' AND updated_at < $1
		ORDER BY updated_at ASC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, query, updatedBefore, limit)
	if err != nil {
		r.logger.Error("get processing driver refund payouts to reconcile failed", zap.Error(err))
		return nil, paymentErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	var refunds []*domain.Refund
	for rows.Next() {
		var ref domain.Refund
		if err := scanRefundRow(rows, &ref); err != nil {
			r.logger.Error("scan processing driver refund payout failed", zap.Error(err))
			return nil, paymentErrors.ErrorDataRetrievalFailed
		}
		refunds = append(refunds, &ref)
	}
	if err := rows.Err(); err != nil {
		return nil, paymentErrors.ErrorDataRetrievalFailed
	}

	return refunds, nil
}
