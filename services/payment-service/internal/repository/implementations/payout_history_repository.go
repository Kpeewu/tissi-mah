package implementations

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/payment-service/internal/repository/interfaces"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
)

type payoutHistoryRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

// NewPayoutHistoryRepository crée le repository de payout_status_history.
func NewPayoutHistoryRepository(pool *pgxpool.Pool, logger *zap.Logger) repoInterfaces.PayoutHistoryRepository {
	return &payoutHistoryRepository{pool: pool, logger: logger.Named("payout-history-repo")}
}

func (r *payoutHistoryRepository) CreateHistoryEntry(ctx context.Context, entry *domain.PayoutStatusHistory) error {
	query := `INSERT INTO payout_status_history (
		history_id, payout_id, status, initiated_by,
		support_user_id, support_first_name, support_last_name,
		occurred_at, notes
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := r.pool.Exec(ctx, query,
		entry.HistoryID, entry.PayoutID, entry.Status, entry.InitiatedBy,
		nullableString(entry.SupportUserID), nullableString(entry.SupportFirstName), nullableString(entry.SupportLastName),
		entry.OccurredAt, nullableString(entry.Notes),
	)
	if err != nil {
		r.logger.Error("create payout history entry failed",
			zap.Error(err),
			zap.String("payoutID", entry.PayoutID),
			zap.String("status", entry.Status),
		)
		return paymentErrors.ErrorInternalServer
	}
	return nil
}

func (r *payoutHistoryRepository) ListByPayoutID(ctx context.Context, payoutID string) ([]*domain.PayoutStatusHistory, error) {
	query := `SELECT history_id, payout_id, status, initiated_by,
		COALESCE(support_user_id, ''), COALESCE(support_first_name, ''), COALESCE(support_last_name, ''),
		occurred_at, COALESCE(notes, '')
		FROM payout_status_history
		WHERE payout_id = $1
		ORDER BY occurred_at ASC, history_id ASC`

	rows, err := r.pool.Query(ctx, query, payoutID)
	if err != nil {
		r.logger.Error("list payout history failed", zap.Error(err), zap.String("payoutID", payoutID))
		return nil, paymentErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	entries := []*domain.PayoutStatusHistory{}
	for rows.Next() {
		var e domain.PayoutStatusHistory
		if err := rows.Scan(
			&e.HistoryID, &e.PayoutID, &e.Status, &e.InitiatedBy,
			&e.SupportUserID, &e.SupportFirstName, &e.SupportLastName,
			&e.OccurredAt, &e.Notes,
		); err != nil {
			r.logger.Error("scan payout history failed", zap.Error(err))
			return nil, paymentErrors.ErrorDataRetrievalFailed
		}
		entries = append(entries, &e)
	}
	if err := rows.Err(); err != nil {
		return nil, paymentErrors.ErrorDataRetrievalFailed
	}
	return entries, nil
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
