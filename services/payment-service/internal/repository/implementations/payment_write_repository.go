package implementations

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
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
		status, external_transaction_id, payment_reference, passenger_phone_number, metadata,
		payment_url, provider_status, provider_status_updated_at, expires_at
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NULLIF($12, ''), NULLIF($13, ''), NOW(), $14)`

	_, err := r.pool.Exec(ctx, query,
		payment.PaymentID, payment.BookingID, payment.TripID, payment.Amount,
		payment.PaymentMethod, payment.PaymentProvider,
		payment.Status, payment.ExternalTransactionID, payment.PaymentReference,
		payment.PassengerPhoneNumber, payment.Metadata,
		payment.PaymentURL, payment.ProviderStatus, payment.ExpiresAt,
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
	var allowedCurrent []string
	switch status {
	case domain.PaymentStatusHeld:
		allowedCurrent = []string{string(domain.PaymentStatusPending)}
	case domain.PaymentStatusReleased:
		allowedCurrent = []string{string(domain.PaymentStatusHeld)}
	case domain.PaymentStatusRefunded:
		allowedCurrent = []string{string(domain.PaymentStatusHeld), string(domain.PaymentStatusReleased)}
	default:
		allowedCurrent = []string{string(domain.PaymentStatusPending)}
	}

	var query string
	var args []any
	if status == domain.PaymentStatusHeld {
		query = `UPDATE payments SET status = $2, external_transaction_id = COALESCE(NULLIF($3, ''), external_transaction_id), completed_at = $4
			WHERE payment_id = $1 AND status = ANY($5)`
		args = []any{paymentID, status, externalTransactionID, time.Now().UTC(), allowedCurrent}
	} else {
		query = `UPDATE payments SET status = $2, external_transaction_id = COALESCE(NULLIF($3, ''), external_transaction_id)
			WHERE payment_id = $1 AND status = ANY($4)`
		args = []any{paymentID, status, externalTransactionID, allowedCurrent}
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

// UpdateProviderStatus enregistre le dernier statut FedaPay observé sur un paiement.
// N'affecte pas le statut interne (declined/canceled ne sont pas finaux en mode redirection).
func (r *paymentWriteRepository) UpdateProviderStatus(ctx context.Context, paymentID string, providerStatus string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE payments SET provider_status = $2, provider_status_updated_at = NOW()
		 WHERE payment_id = $1 AND provider_status IS DISTINCT FROM $2`,
		paymentID, providerStatus,
	)
	if err != nil {
		r.logger.Error("update provider status failed", zap.Error(err), zap.String("paymentID", paymentID))
		return paymentErrors.ErrorInternalServer
	}
	return nil
}

// UpdatePaymentURL met à jour le lien de paiement FedaPay d'un paiement pending (token regénéré).
func (r *paymentWriteRepository) UpdatePaymentURL(ctx context.Context, paymentID string, paymentURL string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE payments SET payment_url = $2 WHERE payment_id = $1 AND status = 'pending'`,
		paymentID, paymentURL,
	)
	if err != nil {
		r.logger.Error("update payment url failed", zap.Error(err), zap.String("paymentID", paymentID))
		return paymentErrors.ErrorInternalServer
	}
	return nil
}

// MarkFailedPaymentHeld fait passer un paiement 'failed' en 'held' lorsque FedaPay confirme
// une approbation tardive (après expiration ou annulation côté TissiMah). L'argent ayant été
// encaissé, il doit être soit rattaché à la réservation, soit remboursé.
func (r *paymentWriteRepository) MarkFailedPaymentHeld(ctx context.Context, paymentID string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE payments SET status = 'held', completed_at = NOW(), booking_synced_at = NULL, booking_sync_attempts = 0,
			provider_status = 'approved', provider_status_updated_at = NOW()
		 WHERE payment_id = $1 AND status = 'failed'`,
		paymentID,
	)
	if err != nil {
		if isDuplicateKeyError(err) {
			// Un autre paiement actif existe pour la réservation (index unique pending/held)
			return paymentErrors.ErrorDuplicatePayment
		}
		r.logger.Error("mark failed payment held failed", zap.Error(err), zap.String("paymentID", paymentID))
		return paymentErrors.ErrorInternalServer
	}
	if tag.RowsAffected() == 0 {
		return paymentErrors.ErrorPaymentAlreadyProcessed
	}
	return nil
}

// MarkBookingSynced enregistre que le paiement held est rattaché à sa réservation :
// il n'est plus repris par le worker de rattachement. Idempotent.
func (r *paymentWriteRepository) MarkBookingSynced(ctx context.Context, paymentID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE payments SET booking_synced_at = NOW()
		 WHERE payment_id = $1 AND booking_synced_at IS NULL`,
		paymentID,
	)
	if err != nil {
		r.logger.Error("mark booking synced failed", zap.Error(err), zap.String("paymentID", paymentID))
		return paymentErrors.ErrorInternalServer
	}
	return nil
}

// IncrementBookingSyncAttempts incrémente le compteur de reprises de rattachement échouées
// d'un paiement et retourne sa nouvelle valeur.
func (r *paymentWriteRepository) IncrementBookingSyncAttempts(ctx context.Context, paymentID string) (int, error) {
	var attempts int
	err := r.pool.QueryRow(ctx,
		`UPDATE payments SET booking_sync_attempts = booking_sync_attempts + 1
		 WHERE payment_id = $1 RETURNING booking_sync_attempts`,
		paymentID,
	).Scan(&attempts)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, paymentErrors.ErrorPaymentNotFound
		}
		r.logger.Error("increment booking sync attempts failed", zap.Error(err), zap.String("paymentID", paymentID))
		return 0, paymentErrors.ErrorInternalServer
	}
	return attempts, nil
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

// AnonymizePassengerPhoneNumbers efface passenger_phone_number pour les paiements des bookings donnés.
func (r *paymentWriteRepository) AnonymizePassengerPhoneNumbers(ctx context.Context, bookingIDs []string) error {
	if len(bookingIDs) == 0 {
		return nil
	}
	_, err := r.pool.Exec(ctx,
		`UPDATE payments SET passenger_phone_number = '', updated_at = NOW()
		 WHERE booking_id = ANY($1)`,
		bookingIDs,
	)
	if err != nil {
		r.logger.Error("AnonymizePassengerPhoneNumbers failed", zap.Error(err))
		return paymentErrors.ErrorInternalServer
	}
	return nil
}
