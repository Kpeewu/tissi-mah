package implementations

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/payment-service/internal/repository/interfaces"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
)

type paymentHistoryRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

const paymentHistorySelectColumns = `h.history_id, h.payment_id, h.status, COALESCE(h.provider_status, ''),
	h.initiated_by, h.source, h.occurred_at, COALESCE(h.notes, '')`

// NewPaymentHistoryRepository crée le repository de payment_status_history.
func NewPaymentHistoryRepository(pool *pgxpool.Pool, logger *zap.Logger) repoInterfaces.PaymentHistoryRepository {
	return &paymentHistoryRepository{pool: pool, logger: logger.Named("payment-history-repo")}
}

func (r *paymentHistoryRepository) CreateHistoryEntry(ctx context.Context, entry *domain.PaymentStatusHistory) error {
	query := `INSERT INTO payment_status_history (
		history_id, payment_id, status, provider_status,
		initiated_by, source, occurred_at, notes
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := r.pool.Exec(ctx, query,
		entry.HistoryID, entry.PaymentID, entry.Status, nullableString(entry.ProviderStatus),
		entry.InitiatedBy, entry.Source, entry.OccurredAt, nullableString(entry.Notes),
	)
	if err != nil {
		r.logger.Error("create payment history entry failed",
			zap.Error(err),
			zap.String("paymentID", entry.PaymentID),
			zap.String("status", entry.Status),
		)
		return paymentErrors.ErrorInternalServer
	}
	return nil
}

func (r *paymentHistoryRepository) ListByPaymentID(ctx context.Context, paymentID string) ([]*domain.PaymentStatusHistory, error) {
	return r.list(ctx, `SELECT `+paymentHistorySelectColumns+`
		FROM payment_status_history h
		WHERE h.payment_id = $1
		ORDER BY h.occurred_at ASC, h.history_id ASC`, paymentID)
}

func (r *paymentHistoryRepository) ListByBookingID(ctx context.Context, bookingID string) ([]*domain.PaymentStatusHistory, error) {
	return r.list(ctx, `SELECT `+paymentHistorySelectColumns+`
		FROM payment_status_history h
		JOIN payments p ON p.payment_id = h.payment_id
		WHERE p.booking_id = $1
		ORDER BY h.occurred_at ASC, h.history_id ASC`, bookingID)
}

func (r *paymentHistoryRepository) list(ctx context.Context, query string, arg string) ([]*domain.PaymentStatusHistory, error) {
	rows, err := r.pool.Query(ctx, query, arg)
	if err != nil {
		r.logger.Error("list payment history failed", zap.Error(err), zap.String("arg", arg))
		return nil, paymentErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	entries := []*domain.PaymentStatusHistory{}
	for rows.Next() {
		var e domain.PaymentStatusHistory
		if err := rows.Scan(
			&e.HistoryID, &e.PaymentID, &e.Status, &e.ProviderStatus,
			&e.InitiatedBy, &e.Source, &e.OccurredAt, &e.Notes,
		); err != nil {
			r.logger.Error("scan payment history failed", zap.Error(err))
			return nil, paymentErrors.ErrorDataRetrievalFailed
		}
		entries = append(entries, &e)
	}
	if err := rows.Err(); err != nil {
		return nil, paymentErrors.ErrorDataRetrievalFailed
	}
	return entries, nil
}
