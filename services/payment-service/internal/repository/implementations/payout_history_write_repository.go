package implementations

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/payment-service/internal/repository/interfaces"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
)

type payoutHistoryWriteRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewPayoutHistoryWriteRepository(pool *pgxpool.Pool, logger *zap.Logger) repoInterfaces.PayoutHistoryRepositoryWrite {
	return &payoutHistoryWriteRepository{pool: pool, logger: logger.Named("payout-history-write-repo")}
}

func (r *payoutHistoryWriteRepository) CreateHistoryEntry(ctx context.Context, entry *domain.PayoutStatusHistory) error {
	query := `INSERT INTO payout_status_history (
		history_id, payout_id, status, initiated_by,
		support_user_id, support_first_name, support_last_name,
		occurred_at, notes
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	var supportUserID, supportFirstName, supportLastName *string
	if entry.SupportUserID != "" {
		supportUserID = &entry.SupportUserID
	}
	if entry.SupportFirstName != "" {
		supportFirstName = &entry.SupportFirstName
	}
	if entry.SupportLastName != "" {
		supportLastName = &entry.SupportLastName
	}

	_, err := r.pool.Exec(ctx, query,
		entry.HistoryID, entry.PayoutID, entry.Status, entry.InitiatedBy,
		supportUserID, supportFirstName, supportLastName,
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

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
