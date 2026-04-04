package implementations

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/domain"
	notifErrors "github.com/Kpeewu/tissi-mah/services/notification-service/pkg/errors"
)

type InboxRepositoryImpl struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewInboxRepository(pool *pgxpool.Pool, logger *zap.Logger) *InboxRepositoryImpl {
	return &InboxRepositoryImpl{pool: pool, logger: logger}
}

func (r *InboxRepositoryImpl) Create(ctx context.Context, entry *domain.InboxEntry) error {
	query := `INSERT INTO notification_inbox (
		inbox_id, user_id, event_type, title, body, action_type, action_id,
		is_read, notification_id, created_at
	) VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, false, $7, NOW())
	RETURNING inbox_id`

	var notifID interface{}
	if entry.NotificationID != "" {
		notifID = entry.NotificationID
	}

	err := r.pool.QueryRow(ctx, query,
		entry.UserID, entry.EventType, entry.Title, entry.Body,
		entry.ActionType, entry.ActionID, notifID,
	).Scan(&entry.InboxID)
	if err != nil {
		r.logger.Error("failed to create inbox entry", zap.Error(err))
		return notifErrors.ErrorDataInsertFail
	}

	return nil
}

func (r *InboxRepositoryImpl) GetByUserID(ctx context.Context, userID string, page, pageSize int) ([]*domain.InboxEntry, int, error) {
	// Compter le total
	countQuery := `SELECT COUNT(*) FROM notification_inbox WHERE user_id = $1`
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, userID).Scan(&total); err != nil {
		return nil, 0, notifErrors.ErrorDataRetrievalFail
	}

	offset := (page - 1) * pageSize
	query := `SELECT inbox_id, user_id, event_type, title, body, action_type, action_id,
		is_read, read_at, notification_id, created_at
		FROM notification_inbox
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, query, userID, pageSize, offset)
	if err != nil {
		r.logger.Error("failed to get inbox", zap.String("user_id", userID), zap.Error(err))
		return nil, 0, notifErrors.ErrorDataRetrievalFail
	}
	defer rows.Close()

	var entries []*domain.InboxEntry
	for rows.Next() {
		var e domain.InboxEntry
		var notifID *string
		if err := rows.Scan(
			&e.InboxID, &e.UserID, &e.EventType, &e.Title, &e.Body,
			&e.ActionType, &e.ActionID, &e.IsRead, &e.ReadAt,
			&notifID, &e.CreatedAt,
		); err != nil {
			return nil, 0, notifErrors.ErrorDataRetrievalFail
		}
		if notifID != nil {
			e.NotificationID = *notifID
		}
		entries = append(entries, &e)
	}

	return entries, total, nil
}

func (r *InboxRepositoryImpl) MarkAsRead(ctx context.Context, inboxID, userID string) error {
	query := `UPDATE notification_inbox SET is_read = true, read_at = NOW() WHERE inbox_id = $1 AND user_id = $2`

	tag, err := r.pool.Exec(ctx, query, inboxID, userID)
	if err != nil {
		r.logger.Error("failed to mark as read", zap.String("inbox_id", inboxID), zap.Error(err))
		return notifErrors.ErrorDataUpdateFail
	}

	if tag.RowsAffected() == 0 {
		return notifErrors.ErrorNotFound
	}

	return nil
}

func (r *InboxRepositoryImpl) MarkAllAsRead(ctx context.Context, userID string) (int, error) {
	query := `UPDATE notification_inbox SET is_read = true, read_at = NOW()
		WHERE user_id = $1 AND is_read = false`

	tag, err := r.pool.Exec(ctx, query, userID)
	if err != nil {
		r.logger.Error("failed to mark all as read", zap.String("user_id", userID), zap.Error(err))
		return 0, notifErrors.ErrorDataUpdateFail
	}

	return int(tag.RowsAffected()), nil
}

func (r *InboxRepositoryImpl) GetUnreadCount(ctx context.Context, userID string) (int, error) {
	query := `SELECT COUNT(*) FROM notification_inbox WHERE user_id = $1 AND is_read = false`

	var count int
	if err := r.pool.QueryRow(ctx, query, userID).Scan(&count); err != nil {
		r.logger.Error("failed to get unread count", zap.String("user_id", userID), zap.Error(err))
		return 0, notifErrors.ErrorDataRetrievalFail
	}

	return count, nil
}

func (r *InboxRepositoryImpl) PurgeOldRead(ctx context.Context, days int) (int64, error) {
	query := `DELETE FROM notification_inbox
		WHERE is_read = true
		AND created_at < NOW() - $1::interval`

	interval := time.Duration(days) * 24 * time.Hour
	intervalStr := interval.String()

	tag, err := r.pool.Exec(ctx, query, intervalStr)
	if err != nil {
		r.logger.Error("failed to purge old inbox entries", zap.Error(err))
		return 0, notifErrors.ErrorDataDeleteFail
	}

	return tag.RowsAffected(), nil
}
