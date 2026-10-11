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

// paymentColumns liste les colonnes lues pour un paiement (ordre attendu par scanPaymentRow).
const paymentColumns = `payment_id, booking_id, trip_id, amount, payment_method, payment_provider,
		status, external_transaction_id, payment_reference, passenger_phone_number,
		COALESCE(payment_url, ''), COALESCE(provider_status, ''), expires_at, booking_sync_attempts,
		created_at, completed_at, failed_at, failure_reason, metadata, updated_at`

// scanPaymentRow lit une ligne produite par une requête sélectionnant paymentColumns.
func scanPaymentRow(row rowScanner, p *domain.Payment) error {
	return row.Scan(
		&p.PaymentID, &p.BookingID, &p.TripID, &p.Amount,
		&p.PaymentMethod, &p.PaymentProvider,
		&p.Status, &p.ExternalTransactionID, &p.PaymentReference, &p.PassengerPhoneNumber,
		&p.PaymentURL, &p.ProviderStatus, &p.ExpiresAt, &p.BookingSyncAttempts,
		&p.CreatedAt, &p.CompletedAt, &p.FailedAt, &p.FailureReason,
		&p.Metadata, &p.UpdatedAt,
	)
}

type paymentReadRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewPaymentReadRepository(pool *pgxpool.Pool, logger *zap.Logger) repoInterfaces.PaymentRepositoryRead {
	return &paymentReadRepository{pool: pool, logger: logger.Named("payment-read-repo")}
}

func (r *paymentReadRepository) GetByID(ctx context.Context, paymentID string) (*domain.Payment, error) {
	query := `SELECT ` + paymentColumns + `
		FROM payments WHERE payment_id = $1`

	return r.scanPayment(ctx, query, paymentID)
}

func (r *paymentReadRepository) GetByBookingID(ctx context.Context, bookingID string) (*domain.Payment, error) {
	query := `SELECT ` + paymentColumns + `
		FROM payments WHERE booking_id = $1 ORDER BY created_at DESC LIMIT 1`

	return r.scanPayment(ctx, query, bookingID)
}

func (r *paymentReadRepository) GetByExternalTransactionID(ctx context.Context, externalID string) (*domain.Payment, error) {
	query := `SELECT ` + paymentColumns + `
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

// GetExpiredPendingPayments retourne les paiements pending dont la fenêtre de paiement est dépassée.
func (r *paymentReadRepository) GetExpiredPendingPayments(ctx context.Context, now time.Time) ([]*domain.Payment, error) {
	query := `SELECT ` + paymentColumns + `
		FROM payments WHERE status = 'pending' AND expires_at < $1
		ORDER BY expires_at ASC LIMIT 100`

	return r.queryPayments(ctx, "get expired pending payments", query, now.UTC())
}

// GetUnsyncedHeldPayments retourne les paiements held dont le rattachement à la réservation
// n'a pas abouti, passés en held avant completedBefore.
func (r *paymentReadRepository) GetUnsyncedHeldPayments(
	ctx context.Context, completedBefore time.Time, limit int,
) ([]*domain.Payment, error) {
	query := `SELECT ` + paymentColumns + `
		FROM payments WHERE status = 'held' AND booking_synced_at IS NULL AND completed_at < $1
		ORDER BY completed_at ASC LIMIT $2`

	return r.queryPayments(ctx, "get unsynced held payments", query, completedBefore.UTC(), limit)
}

// queryPayments exécute une requête sélectionnant paymentColumns et retourne les paiements lus.
func (r *paymentReadRepository) queryPayments(
	ctx context.Context, op string, query string, args ...any,
) ([]*domain.Payment, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		r.logger.Error(op+" failed", zap.Error(err))
		return nil, paymentErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	var payments []*domain.Payment
	for rows.Next() {
		var p domain.Payment
		if err := scanPaymentRow(rows, &p); err != nil {
			r.logger.Error(op+": scan failed", zap.Error(err))
			return nil, paymentErrors.ErrorDataRetrievalFailed
		}
		payments = append(payments, &p)
	}
	if err := rows.Err(); err != nil {
		r.logger.Error(op+": iterate failed", zap.Error(err))
		return nil, paymentErrors.ErrorDataRetrievalFailed
	}

	return payments, nil
}

// GetPendingByBookingID retourne le paiement pending d'une réservation (ErrorPaymentNotFound sinon).
func (r *paymentReadRepository) GetPendingByBookingID(ctx context.Context, bookingID string) (*domain.Payment, error) {
	query := `SELECT ` + paymentColumns + `
		FROM payments WHERE booking_id = $1 AND status = 'pending'
		ORDER BY created_at DESC LIMIT 1`

	return r.scanPayment(ctx, query, bookingID)
}

func (r *paymentReadRepository) HasActivePayment(ctx context.Context, bookingID string) (bool, error) {
	query := `SELECT EXISTS (SELECT 1 FROM payments WHERE booking_id = $1 AND status IN ('pending', 'held'))`
	var exists bool
	if err := r.pool.QueryRow(ctx, query, bookingID).Scan(&exists); err != nil {
		r.logger.Error("has active payment check failed", zap.Error(err), zap.String("bookingID", bookingID))
		return false, paymentErrors.ErrorDataRetrievalFailed
	}
	return exists, nil
}

func (r *paymentReadRepository) scanPayment(ctx context.Context, query string, arg interface{}) (*domain.Payment, error) {
	var p domain.Payment
	if err := scanPaymentRow(r.pool.QueryRow(ctx, query, arg), &p); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, paymentErrors.ErrorPaymentNotFound
		}
		r.logger.Error("scan payment failed", zap.Error(err))
		return nil, paymentErrors.ErrorDataRetrievalFailed
	}

	return &p, nil
}
