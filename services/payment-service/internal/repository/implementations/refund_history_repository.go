package implementations

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/payment-service/internal/repository/interfaces"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
)

type refundHistoryRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

const refundHistorySelectColumns = `h.history_id, h.refund_id, h.status, h.retry_count,
	h.initiated_by, h.source, h.occurred_at, COALESCE(h.notes, '')`

// NewRefundHistoryRepository crée le repository de refund_status_history.
func NewRefundHistoryRepository(pool *pgxpool.Pool, logger *zap.Logger) repoInterfaces.RefundHistoryRepository {
	return &refundHistoryRepository{pool: pool, logger: logger.Named("refund-history-repo")}
}

func (r *refundHistoryRepository) CreateHistoryEntry(ctx context.Context, entry *domain.RefundStatusHistory) error {
	query := `INSERT INTO refund_status_history (
		history_id, refund_id, status, retry_count,
		initiated_by, source, occurred_at, notes
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := r.pool.Exec(ctx, query,
		entry.HistoryID, entry.RefundID, entry.Status, entry.RetryCount,
		entry.InitiatedBy, entry.Source, entry.OccurredAt, nullableString(entry.Notes),
	)
	if err != nil {
		r.logger.Error("create refund history entry failed",
			zap.Error(err),
			zap.String("refundID", entry.RefundID),
			zap.String("status", entry.Status),
		)
		return paymentErrors.ErrorInternalServer
	}
	return nil
}

func (r *refundHistoryRepository) ListByRefundID(ctx context.Context, refundID string) ([]*domain.RefundStatusHistory, error) {
	return r.list(ctx, `SELECT `+refundHistorySelectColumns+`
		FROM refund_status_history h
		WHERE h.refund_id = $1
		ORDER BY h.occurred_at ASC, h.history_id ASC`, refundID)
}

func (r *refundHistoryRepository) ListByBookingID(ctx context.Context, bookingID string) ([]*domain.RefundStatusHistory, error) {
	return r.list(ctx, `SELECT `+refundHistorySelectColumns+`
		FROM refund_status_history h
		JOIN refunds rf ON rf.refund_id = h.refund_id
		WHERE rf.booking_id = $1
		ORDER BY h.occurred_at ASC, h.history_id ASC`, bookingID)
}

func (r *refundHistoryRepository) list(ctx context.Context, query string, arg string) ([]*domain.RefundStatusHistory, error) {
	rows, err := r.pool.Query(ctx, query, arg)
	if err != nil {
		r.logger.Error("list refund history failed", zap.Error(err), zap.String("arg", arg))
		return nil, paymentErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	entries := []*domain.RefundStatusHistory{}
	for rows.Next() {
		var e domain.RefundStatusHistory
		if err := rows.Scan(
			&e.HistoryID, &e.RefundID, &e.Status, &e.RetryCount,
			&e.InitiatedBy, &e.Source, &e.OccurredAt, &e.Notes,
		); err != nil {
			r.logger.Error("scan refund history failed", zap.Error(err))
			return nil, paymentErrors.ErrorDataRetrievalFailed
		}
		entries = append(entries, &e)
	}
	if err := rows.Err(); err != nil {
		return nil, paymentErrors.ErrorDataRetrievalFailed
	}
	return entries, nil
}
