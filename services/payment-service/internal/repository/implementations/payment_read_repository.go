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

type paymentReadRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewPaymentReadRepository(pool *pgxpool.Pool, logger *zap.Logger) repoInterfaces.PaymentRepositoryRead {
	return &paymentReadRepository{pool: pool, logger: logger.Named("payment-read-repo")}
}

func (r *paymentReadRepository) GetByID(ctx context.Context, paymentID string) (*domain.Payment, error) {
	query := `SELECT payment_id, booking_id, trip_id, amount, payment_method, payment_provider,
		status, external_transaction_id, payment_reference,
		created_at, completed_at, failed_at, failure_reason, metadata, updated_at
		FROM payments WHERE payment_id = $1`

	return r.scanPayment(ctx, query, paymentID)
}

func (r *paymentReadRepository) GetByBookingID(ctx context.Context, bookingID string) (*domain.Payment, error) {
	query := `SELECT payment_id, booking_id, trip_id, amount, payment_method, payment_provider,
		status, external_transaction_id, payment_reference,
		created_at, completed_at, failed_at, failure_reason, metadata, updated_at
		FROM payments WHERE booking_id = $1 ORDER BY created_at DESC LIMIT 1`

	return r.scanPayment(ctx, query, bookingID)
}

func (r *paymentReadRepository) GetByExternalTransactionID(ctx context.Context, externalID string) (*domain.Payment, error) {
	query := `SELECT payment_id, booking_id, trip_id, amount, payment_method, payment_provider,
		status, external_transaction_id, payment_reference,
		created_at, completed_at, failed_at, failure_reason, metadata, updated_at
		FROM payments WHERE external_transaction_id = $1`

	return r.scanPayment(ctx, query, externalID)
}

func (r *paymentReadRepository) GetWebhookEvent(ctx context.Context, fedapayEventID string) (*domain.WebhookEvent, error) {
	query := `SELECT event_id, fedapay_event_id, event_type, payload, processed_at, created_at
		FROM webhook_events WHERE fedapay_event_id = $1`

	var event domain.WebhookEvent
	err := r.pool.QueryRow(ctx, query, fedapayEventID).Scan(
		&event.EventID, &event.FedapayEventID, &event.EventType,
		&event.Payload, &event.ProcessedAt, &event.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		r.logger.Error("get webhook event failed", zap.Error(err))
		return nil, paymentErrors.ErrorDataRetrievalFailed
	}

	return &event, nil
}

func (r *paymentReadRepository) scanPayment(ctx context.Context, query string, arg interface{}) (*domain.Payment, error) {
	var p domain.Payment
	err := r.pool.QueryRow(ctx, query, arg).Scan(
		&p.PaymentID, &p.BookingID, &p.TripID, &p.Amount,
		&p.PaymentMethod, &p.PaymentProvider,
		&p.Status, &p.ExternalTransactionID, &p.PaymentReference,
		&p.CreatedAt, &p.CompletedAt, &p.FailedAt, &p.FailureReason,
		&p.Metadata, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, paymentErrors.ErrorPaymentNotFound
		}
		r.logger.Error("scan payment failed", zap.Error(err))
		return nil, paymentErrors.ErrorDataRetrievalFailed
	}

	return &p, nil
}
