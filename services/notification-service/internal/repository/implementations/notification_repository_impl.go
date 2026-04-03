package implementations

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/domain"
	notifErrors "github.com/Kpeewu/tissi-mah/services/notification-service/pkg/errors"
)

type NotificationRepositoryImpl struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewNotificationRepository(pool *pgxpool.Pool, logger *zap.Logger) *NotificationRepositoryImpl {
	return &NotificationRepositoryImpl{pool: pool, logger: logger}
}

func (r *NotificationRepositoryImpl) Create(ctx context.Context, n *domain.Notification) error {
	query := `INSERT INTO notifications (
		notification_id, event_id, user_id, event_type, channel, template_id,
		resolved_title, resolved_subject, resolved_body, recipient_address,
		status, reference_id, reference_type, attempt_count, max_attempts,
		provider_name, created_at, updated_at
	) VALUES (
		gen_random_uuid(), $1, $2, $3, $4, $5,
		$6, $7, $8, $9,
		$10, $11, $12, $13, $14,
		$15, NOW(), NOW()
	) RETURNING notification_id`

	err := r.pool.QueryRow(ctx, query,
		n.EventID, n.UserID, n.EventType, n.Channel, n.TemplateID,
		n.ResolvedTitle, n.ResolvedSubject, n.ResolvedBody, n.RecipientAddress,
		n.Status, n.ReferenceID, n.ReferenceType, n.AttemptCount, n.MaxAttempts,
		n.ProviderName,
	).Scan(&n.NotificationID)
	if err != nil {
		r.logger.Error("failed to create notification", zap.Error(err))
		return notifErrors.ErrorDataInsertFail
	}

	return nil
}

func (r *NotificationRepositoryImpl) UpdateStatus(ctx context.Context, notificationID, status, failureReason, providerMessageID string) error {
	query := `UPDATE notifications SET
		status = $2,
		failure_reason = $3,
		provider_message_id = $4,
		last_attempt_at = NOW(),
		sent_at = CASE WHEN $2 = 'sent' THEN NOW() ELSE sent_at END,
		updated_at = NOW()
		WHERE notification_id = $1`

	_, err := r.pool.Exec(ctx, query, notificationID, status, failureReason, providerMessageID)
	if err != nil {
		r.logger.Error("failed to update notification status", zap.String("id", notificationID), zap.Error(err))
		return notifErrors.ErrorDataUpdateFail
	}

	return nil
}

func (r *NotificationRepositoryImpl) UpdateRetry(ctx context.Context, n *domain.Notification) error {
	query := `UPDATE notifications SET
		status = $2,
		attempt_count = $3,
		next_attempt_at = $4,
		last_attempt_at = NOW(),
		failure_reason = $5,
		updated_at = NOW()
		WHERE notification_id = $1`

	_, err := r.pool.Exec(ctx, query, n.NotificationID, n.Status, n.AttemptCount, n.NextAttemptAt, n.FailureReason)
	if err != nil {
		r.logger.Error("failed to update notification retry", zap.String("id", n.NotificationID), zap.Error(err))
		return notifErrors.ErrorDataUpdateFail
	}

	return nil
}

func (r *NotificationRepositoryImpl) ExistsByEventIDAndChannel(ctx context.Context, eventID, channel string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM notifications WHERE event_id = $1 AND channel = $2)`

	var exists bool
	err := r.pool.QueryRow(ctx, query, eventID, channel).Scan(&exists)
	if err != nil {
		r.logger.Error("failed to check notification existence", zap.Error(err))
		return false, notifErrors.ErrorDataRetrievalFail
	}

	return exists, nil
}

func (r *NotificationRepositoryImpl) GetPendingForRetry(ctx context.Context, limit int) ([]*domain.Notification, error) {
	query := `SELECT notification_id, event_id, user_id, event_type, channel, template_id,
		resolved_title, resolved_subject, resolved_body, recipient_address,
		status, failure_reason, reference_id, reference_type,
		attempt_count, max_attempts, next_attempt_at, last_attempt_at,
		provider_name, provider_message_id, sent_at, delivered_at,
		created_at, updated_at
		FROM notifications
		WHERE status IN ('pending', 'failed')
		AND attempt_count < max_attempts
		AND (next_attempt_at IS NULL OR next_attempt_at <= NOW())
		ORDER BY
			CASE priority_cache WHEN 'critical' THEN 1 WHEN 'standard' THEN 2 ELSE 3 END,
			created_at ASC
		LIMIT $1`

	// Note : on utilise un JOIN ou une sous-requête pour la priorité.
	// Alternative simplifiée : stocker la priorité directement dans notifications.
	// Pour l'instant, on utilise une version sans la colonne priority_cache.
	query = `SELECT notification_id, event_id, user_id, event_type, channel, template_id,
		resolved_title, resolved_subject, resolved_body, recipient_address,
		status, failure_reason, reference_id, reference_type,
		attempt_count, max_attempts, next_attempt_at, last_attempt_at,
		provider_name, provider_message_id, sent_at, delivered_at,
		created_at, updated_at
		FROM notifications
		WHERE status IN ('pending', 'failed')
		AND attempt_count < max_attempts
		AND (next_attempt_at IS NULL OR next_attempt_at <= NOW())
		ORDER BY created_at ASC
		LIMIT $1`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		r.logger.Error("failed to get pending notifications", zap.Error(err))
		return nil, notifErrors.ErrorDataRetrievalFail
	}
	defer rows.Close()

	var notifications []*domain.Notification
	for rows.Next() {
		var n domain.Notification
		if err := rows.Scan(
			&n.NotificationID, &n.EventID, &n.UserID, &n.EventType, &n.Channel, &n.TemplateID,
			&n.ResolvedTitle, &n.ResolvedSubject, &n.ResolvedBody, &n.RecipientAddress,
			&n.Status, &n.FailureReason, &n.ReferenceID, &n.ReferenceType,
			&n.AttemptCount, &n.MaxAttempts, &n.NextAttemptAt, &n.LastAttemptAt,
			&n.ProviderName, &n.ProviderMessageID, &n.SentAt, &n.DeliveredAt,
			&n.CreatedAt, &n.UpdatedAt,
		); err != nil {
			return nil, notifErrors.ErrorDataRetrievalFail
		}
		notifications = append(notifications, &n)
	}

	return notifications, nil
}

func (r *NotificationRepositoryImpl) PurgeOldSent(ctx context.Context, days int) (int64, error) {
	query := `DELETE FROM notifications
		WHERE status IN ('sent', 'delivered', 'cancelled')
		AND created_at < NOW() - $1::interval`

	interval := time.Duration(days) * 24 * time.Hour
	intervalStr := interval.String()

	tag, err := r.pool.Exec(ctx, query, intervalStr)
	if err != nil {
		r.logger.Error("failed to purge old notifications", zap.Error(err))
		return 0, notifErrors.ErrorDataDeleteFail
	}

	return tag.RowsAffected(), nil
}
