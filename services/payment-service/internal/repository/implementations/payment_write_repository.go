package implementations

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/payment-service/internal/repository/interfaces"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
)

type paymentWriteRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewPaymentWriteRepository(pool *pgxpool.Pool, logger *zap.Logger) repoInterfaces.PaymentRepositoryWrite {
	return &paymentWriteRepository{pool: pool, logger: logger.Named("payment-write-repo")}
}

func (r *paymentWriteRepository) CreatePayment(ctx context.Context, payment *domain.Payment) error {
	query := `INSERT INTO payments (
		payment_id, booking_id, trip_id, amount, payment_method, payment_provider,
		status, external_transaction_id, payment_reference, metadata
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := r.pool.Exec(ctx, query,
		payment.PaymentID, payment.BookingID, payment.TripID, payment.Amount,
		payment.PaymentMethod, payment.PaymentProvider,
		payment.Status, payment.ExternalTransactionID, payment.PaymentReference,
		payment.Metadata,
	)
	if err != nil {
		if isDuplicateKeyError(err) {
			return paymentErrors.ErrorDuplicatePayment
		}
		r.logger.Error("create payment failed", zap.Error(err), zap.String("paymentID", payment.PaymentID))
		return paymentErrors.ErrorInternalServer
	}

	return nil
}

// isDuplicateKeyError vérifie si l'erreur PostgreSQL est une violation de contrainte unique (23505).
func isDuplicateKeyError(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (r *paymentWriteRepository) UpdatePaymentStatus(ctx context.Context, paymentID string, status domain.PaymentStatus, externalTransactionID string) error {
	var expectedCurrent domain.PaymentStatus
	switch status {
	case domain.PaymentStatusHeld:
		expectedCurrent = domain.PaymentStatusPending
	case domain.PaymentStatusReleased, domain.PaymentStatusRefunded:
		expectedCurrent = domain.PaymentStatusHeld
	default:
		expectedCurrent = domain.PaymentStatusPending
	}

	var query string
	var args []any
	if status == domain.PaymentStatusHeld {
		query = `UPDATE payments SET status = $2, external_transaction_id = COALESCE(NULLIF($3, ''), external_transaction_id), completed_at = $4
			WHERE payment_id = $1 AND status = $5`
		args = []any{paymentID, status, externalTransactionID, time.Now().UTC(), expectedCurrent}
	} else {
		query = `UPDATE payments SET status = $2, external_transaction_id = COALESCE(NULLIF($3, ''), external_transaction_id)
			WHERE payment_id = $1 AND status = $4`
		args = []any{paymentID, status, externalTransactionID, expectedCurrent}
	}

	tag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		r.logger.Error("update payment status failed", zap.Error(err), zap.String("paymentID", paymentID))
		return paymentErrors.ErrorInternalServer
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM payments WHERE payment_id = $1)`, paymentID).Scan(&exists); err != nil {
			r.logger.Error("check payment existence failed", zap.Error(err), zap.String("paymentID", paymentID))
			return paymentErrors.ErrorInternalServer
		}
		if !exists {
			return paymentErrors.ErrorPaymentNotFound
		}
		return paymentErrors.ErrorPaymentAlreadyProcessed
	}

	return nil
}

func (r *paymentWriteRepository) MarkPaymentFailed(ctx context.Context, paymentID string, reason string) error {
	query := `UPDATE payments SET status = 'failed', failed_at = $2, failure_reason = $3
		WHERE payment_id = $1 AND status = 'pending'`

	now := time.Now().UTC()
	tag, err := r.pool.Exec(ctx, query, paymentID, now, reason)
	if err != nil {
		r.logger.Error("mark payment failed", zap.Error(err), zap.String("paymentID", paymentID))
		return paymentErrors.ErrorInternalServer
	}
	if tag.RowsAffected() == 0 {
		return paymentErrors.ErrorPaymentAlreadyProcessed
	}

	return nil
}

func (r *paymentWriteRepository) SaveWebhookEvent(ctx context.Context, event *domain.WebhookEvent) error {
	// INSERT ON CONFLICT DO NOTHING — idempotence
	query := `INSERT INTO webhook_events (event_id, fedapay_event_id, event_type, payload, processed_at)
		VALUES ($1, $2, $3, $4, $5) ON CONFLICT (fedapay_event_id) DO NOTHING`

	tag, err := r.pool.Exec(ctx, query,
		event.EventID, event.FedapayEventID, event.EventType,
		event.Payload, event.ProcessedAt,
	)
	if err != nil {
		r.logger.Error("save webhook event failed", zap.Error(err))
		return paymentErrors.ErrorInternalServer
	}

	if tag.RowsAffected() == 0 {
		return paymentErrors.ErrorDuplicateWebhookEvent
	}

	return nil
}
