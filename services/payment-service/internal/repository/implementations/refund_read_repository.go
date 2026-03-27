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

type refundReadRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewRefundReadRepository(pool *pgxpool.Pool, logger *zap.Logger) repoInterfaces.RefundRepositoryRead {
	return &refundReadRepository{pool: pool, logger: logger.Named("refund-read-repo")}
}

func (r *refundReadRepository) GetByID(ctx context.Context, refundID string) (*domain.Refund, error) {
	return r.scanRefund(ctx, `SELECT refund_id, refund_reference, payment_id, booking_id,
		refund_reason, refund_rule_applied, original_amount, refund_percentage,
		refund_amount, service_fee_refunded, amount_to_passenger, amount_to_driver,
		amount_to_platform, status, refund_method, processed_at, completed_at,
		estimated_completion, passenger_notified, notification_sent_at, notes, updated_at
		FROM refunds WHERE refund_id = $1`, refundID)
}

func (r *refundReadRepository) GetByBookingID(ctx context.Context, bookingID string) (*domain.Refund, error) {
	return r.scanRefund(ctx, `SELECT refund_id, refund_reference, payment_id, booking_id,
		refund_reason, refund_rule_applied, original_amount, refund_percentage,
		refund_amount, service_fee_refunded, amount_to_passenger, amount_to_driver,
		amount_to_platform, status, refund_method, processed_at, completed_at,
		estimated_completion, passenger_notified, notification_sent_at, notes, updated_at
		FROM refunds WHERE booking_id = $1 ORDER BY updated_at DESC LIMIT 1`, bookingID)
}

func (r *refundReadRepository) GetByPaymentID(ctx context.Context, paymentID string) (*domain.Refund, error) {
	return r.scanRefund(ctx, `SELECT refund_id, refund_reference, payment_id, booking_id,
		refund_reason, refund_rule_applied, original_amount, refund_percentage,
		refund_amount, service_fee_refunded, amount_to_passenger, amount_to_driver,
		amount_to_platform, status, refund_method, processed_at, completed_at,
		estimated_completion, passenger_notified, notification_sent_at, notes, updated_at
		FROM refunds WHERE payment_id = $1`, paymentID)
}

func (r *refundReadRepository) scanRefund(ctx context.Context, query string, arg interface{}) (*domain.Refund, error) {
	var ref domain.Refund
	err := r.pool.QueryRow(ctx, query, arg).Scan(
		&ref.RefundID, &ref.RefundReference, &ref.PaymentID, &ref.BookingID,
		&ref.RefundReason, &ref.RefundRuleApplied, &ref.OriginalAmount, &ref.RefundPercentage,
		&ref.RefundAmount, &ref.ServiceFeeRefunded, &ref.AmountToPassenger, &ref.AmountToDriver,
		&ref.AmountToPlatform, &ref.Status, &ref.RefundMethod, &ref.ProcessedAt, &ref.CompletedAt,
		&ref.EstimatedCompletion, &ref.PassengerNotified, &ref.NotificationSentAt, &ref.Notes, &ref.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, paymentErrors.ErrorRefundNotFound
		}
		r.logger.Error("scan refund failed", zap.Error(err))
		return nil, paymentErrors.ErrorDataRetrievalFailed
	}

	return &ref, nil
}
