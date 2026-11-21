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

type violationRepositoryImpl struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewViolationRepository(pool *pgxpool.Pool, logger *zap.Logger) interfaces.ViolationRepository {
	return &violationRepositoryImpl{pool: pool, logger: logger}
}

func (r *violationRepositoryImpl) CountByUserAndType(ctx context.Context, userID, contentType string) (int, error) {
	query := `SELECT COUNT(*) FROM user_violations WHERE user_id = $1 AND content_type = $2`
	var count int
	err := r.pool.QueryRow(ctx, query, userID, contentType).Scan(&count)
	if err != nil {
		r.logger.Error("failed to count violations", zap.Error(err), zap.String("userID", userID))
		return 0, modErrors.ErrorDataRetrievalFailed
	}
	return count, nil
}

func (r *violationRepositoryImpl) Create(ctx context.Context, v *domain.UserViolation) error {
	if v.ViolationID == "" {
		v.ViolationID = uuid.New().String()
	}

	query := `INSERT INTO user_violations
		(violation_id, user_id, content_type, violation_count, suspension_until, is_permanent_ban)
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := r.pool.Exec(ctx, query,
		v.ViolationID, v.UserID, v.ContentType,
		v.ViolationCount, v.SuspensionUntil, v.IsPermanentBan,
	)
	if err != nil {
		r.logger.Error("failed to create violation record", zap.Error(err))
		return modErrors.ErrorInternalServer
	}
	return nil
}
