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

type refundWriteRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewRefundWriteRepository(pool *pgxpool.Pool, logger *zap.Logger) repoInterfaces.RefundRepositoryWrite {
	return &refundWriteRepository{pool: pool, logger: logger.Named("refund-write-repo")}
}

func (r *refundWriteRepository) CreateRefund(ctx context.Context, refund *domain.Refund) error {
	query := `INSERT INTO refunds (
		refund_id, refund_reference, payment_id, booking_id,
		refund_reason, refund_rule_applied, original_amount, refund_percentage,
		refund_amount, service_fee_refunded, amount_to_passenger, amount_to_driver,
		amount_to_platform, status, refund_method, processed_at, completed_at, notes,
		payout_destination, payment_provider, payment_provider_reference
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)`

	provider := refund.PaymentProvider
	if provider == "" {
		provider = "fedapay"
	}

	_, err := r.pool.Exec(ctx, query,
		refund.RefundID, refund.RefundReference, refund.PaymentID, refund.BookingID,
		refund.RefundReason, refund.RefundRuleApplied, refund.OriginalAmount, refund.RefundPercentage,
		refund.RefundAmount, refund.ServiceFeeRefunded, refund.AmountToPassenger, refund.AmountToDriver,
		refund.AmountToPlatform, refund.Status, refund.RefundMethod, refund.ProcessedAt, refund.CompletedAt, refund.Notes,
		refund.PayoutDestination, provider, refund.PaymentProviderReference,
	)
	if err != nil {
		r.logger.Error("create refund failed", zap.Error(err), zap.String("refundID", refund.RefundID))
		return paymentErrors.ErrorInternalServer
	}

	return nil
}

func (r *refundWriteRepository) UpdateRefundStatus(ctx context.Context, refundID string, status domain.RefundStatus) error {
	query := `UPDATE refunds SET status = $2 WHERE refund_id = $1`

	tag, err := r.pool.Exec(ctx, query, refundID, status)
	if err != nil {
		r.logger.Error("update refund status failed", zap.Error(err), zap.String("refundID", refundID))
		return paymentErrors.ErrorInternalServer
	}
	if tag.RowsAffected() == 0 {
		return paymentErrors.ErrorRefundNotFound
	}

	return nil
}

func (r *refundWriteRepository) MarkRefundProcessing(ctx context.Context, refundID string) error {
	query := `UPDATE refunds SET status = 'processing', processed_at = COALESCE(processed_at, $2)
		WHERE refund_id = $1 AND status = 'pending'`

	now := time.Now().UTC()
	tag, err := r.pool.Exec(ctx, query, refundID, now)
	if err != nil {
		r.logger.Error("mark refund processing failed", zap.Error(err), zap.String("refundID", refundID))
		return paymentErrors.ErrorInternalServer
	}
	if tag.RowsAffected() == 0 {
		return paymentErrors.ErrorRefundAlreadyProcessed
	}

	return nil
}

func (r *refundWriteRepository) MarkRefundCompleted(ctx context.Context, refundID string, paymentProviderReference string) error {
	query := `UPDATE refunds SET status = 'completed', completed_at = $2,
		payment_provider_reference = COALESCE(NULLIF($3, ''), payment_provider_reference),
		failure_reason = NULL
		WHERE refund_id = $1`

	now := time.Now().UTC()
	tag, err := r.pool.Exec(ctx, query, refundID, now, paymentProviderReference)
	if err != nil {
		r.logger.Error("mark refund completed failed", zap.Error(err), zap.String("refundID", refundID))
		return paymentErrors.ErrorInternalServer
	}
	if tag.RowsAffected() == 0 {
		return paymentErrors.ErrorRefundNotFound
	}

	return nil
}

func (r *refundWriteRepository) MarkRefundFailed(ctx context.Context, refundID string, reason string, incrementRetry bool) error {
	// incrementRetry=true : repasser en pending pour retry, incrémenter retry_count.
	// incrementRetry=false : rester en failed (erreur non récupérable, ex. numéro manquant).
	var query string
	var args []any
	now := time.Now().UTC()

	if incrementRetry {
		query = `UPDATE refunds SET status = 'pending', failure_reason = $2,
			retry_count = retry_count + 1, last_retry_at = $3
			WHERE refund_id = $1`
		args = []any{refundID, reason, now}
	} else {
		query = `UPDATE refunds SET status = 'failed', failure_reason = $2, last_retry_at = $3
			WHERE refund_id = $1`
		args = []any{refundID, reason, now}
	}

	tag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		r.logger.Error("mark refund failed", zap.Error(err), zap.String("refundID", refundID))
		return paymentErrors.ErrorInternalServer
	}
	if tag.RowsAffected() == 0 {
		return paymentErrors.ErrorRefundNotFound
	}

	return nil
}
