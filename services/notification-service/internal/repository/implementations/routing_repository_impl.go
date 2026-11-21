package implementations

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/domain"
	notifErrors "github.com/Kpeewu/tissi-mah/services/notification-service/pkg/errors"
)

type RoutingRepositoryImpl struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewRoutingRepository(pool *pgxpool.Pool, logger *zap.Logger) *RoutingRepositoryImpl {
	return &RoutingRepositoryImpl{pool: pool, logger: logger}
}

func (r *RoutingRepositoryImpl) GetByEventType(ctx context.Context, eventType string) (*domain.EventRouting, error) {
	query := `SELECT routing_id, event_type, send_push, send_email, create_inbox_entry, priority, is_active, created_at, updated_at
		FROM notification_event_routing
		WHERE event_type = $1 AND is_active = true`

	var routing domain.EventRouting
	err := r.pool.QueryRow(ctx, query, eventType).Scan(
		&routing.RoutingID, &routing.EventType, &routing.SendPush, &routing.SendEmail,
		&routing.CreateInboxEntry, &routing.Priority, &routing.IsActive,
		&routing.CreatedAt, &routing.UpdatedAt,
	)
	if err != nil {
		r.logger.Error("routing not found", zap.String("event_type", eventType), zap.Error(err))
		return nil, notifErrors.ErrorRoutingNotFound
	}

	return &routing, nil
}

func (r *RoutingRepositoryImpl) GetAll(ctx context.Context) ([]*domain.EventRouting, error) {
	query := `SELECT routing_id, event_type, send_push, send_email, create_inbox_entry, priority, is_active, created_at, updated_at
		FROM notification_event_routing
		WHERE is_active = true
		ORDER BY event_type`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		r.logger.Error("failed to get all routings", zap.Error(err))
		return nil, notifErrors.ErrorDataRetrievalFail
	}
	defer rows.Close()

	var routings []*domain.EventRouting
	for rows.Next() {
		var routing domain.EventRouting
		if err := rows.Scan(
			&routing.RoutingID, &routing.EventType, &routing.SendPush, &routing.SendEmail,
			&routing.CreateInboxEntry, &routing.Priority, &routing.IsActive,
			&routing.CreatedAt, &routing.UpdatedAt,
		); err != nil {
			return nil, notifErrors.ErrorDataRetrievalFail
		}
		routings = append(routings, &routing)
	}

	return routings, nil
}
