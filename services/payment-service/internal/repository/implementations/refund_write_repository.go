package implementations

import (
	"context"

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
		amount_to_platform, status, refund_method, processed_at, notes
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)`

	_, err := r.pool.Exec(ctx, query,
		refund.RefundID, refund.RefundReference, refund.PaymentID, refund.BookingID,
		refund.RefundReason, refund.RefundRuleApplied, refund.OriginalAmount, refund.RefundPercentage,
		refund.RefundAmount, refund.ServiceFeeRefunded, refund.AmountToPassenger, refund.AmountToDriver,
		refund.AmountToPlatform, refund.Status, refund.RefundMethod, refund.ProcessedAt, refund.Notes,
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
