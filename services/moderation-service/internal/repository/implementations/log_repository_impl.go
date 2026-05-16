package implementations

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/moderation-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/moderation-service/internal/repository/interfaces"
	modErrors "github.com/Kpeewu/tissi-mah/services/moderation-service/pkg/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type moderationLogRepositoryImpl struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewModerationLogRepository(pool *pgxpool.Pool, logger *zap.Logger) interfaces.ModerationLogRepository {
	return &moderationLogRepositoryImpl{pool: pool, logger: logger}
}

func (r *moderationLogRepositoryImpl) Create(ctx context.Context, log *domain.ModerationLog) error {
	if log.LogID == "" {
		log.LogID = uuid.New().String()
	}

	query := `INSERT INTO moderation_logs
		(log_id, content_id, content_type, author_id, decision, category, score, reason, used_fallback)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := r.pool.Exec(ctx, query,
		log.LogID, log.ContentID, log.ContentType,
		log.AuthorID, string(log.Decision), string(log.Category),
		log.Score, log.Reason, log.UsedFallback,
	)
	if err != nil {
		r.logger.Error("failed to create moderation log", zap.Error(err))
		return modErrors.ErrorInternalServer
	}
	return nil
}
