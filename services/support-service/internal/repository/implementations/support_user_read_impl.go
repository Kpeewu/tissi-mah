package implementations

import (
	"context"
	"errors"

	"github.com/Kpeewu/tissi-mah/services/support-service/internal/domain"
	i "github.com/Kpeewu/tissi-mah/services/support-service/internal/repository/interfaces"
	supportErrors "github.com/Kpeewu/tissi-mah/services/support-service/pkg/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type supportUserReadRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewSupportUserReadRepository(pool *pgxpool.Pool, logger *zap.Logger) i.SupportUserReadRepository {
	return &supportUserReadRepository{pool: pool, logger: logger}
}

const supportUserCols = `
    user_id, email, password_hash, first_name, last_name, role,
    is_active, must_change_password, email_changed_at, password_changed_at,
    password_reset_requested_at, created_at, updated_at, deleted_at
`

func scanSupportUser(row pgx.Row) (*domain.SupportUser, error) {
	var u domain.SupportUser
	err := row.Scan(
		&u.UserID, &u.Email, &u.PasswordHash, &u.FirstName, &u.LastName, &u.Role,
		&u.IsActive, &u.MustChangePassword, &u.EmailChangedAt, &u.PasswordChangedAt,
		&u.PasswordResetRequestedAt, &u.CreatedAt, &u.UpdatedAt, &u.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *supportUserReadRepository) GetByID(ctx context.Context, userID string) (*domain.SupportUser, error) {
	q := `SELECT ` + supportUserCols + ` FROM support_users WHERE user_id = $1 AND deleted_at IS NULL`
	u, err := scanSupportUser(r.pool.QueryRow(ctx, q, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, supportErrors.ErrUserNotFound
		}
		r.logger.Error("GetByID failed", zap.Error(err))
		return nil, supportErrors.ErrInternal
	}
	return u, nil
}

func (r *supportUserReadRepository) GetByEmail(ctx context.Context, email string) (*domain.SupportUser, error) {
	q := `SELECT ` + supportUserCols + ` FROM support_users WHERE email = $1 AND deleted_at IS NULL`
	u, err := scanSupportUser(r.pool.QueryRow(ctx, q, email))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, supportErrors.ErrUserNotFound
		}
		r.logger.Error("GetByEmail failed", zap.Error(err))
		return nil, supportErrors.ErrInternal
	}
	return u, nil
}

func (r *supportUserReadRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM support_users WHERE email = $1 AND deleted_at IS NULL)`,
		email,
	).Scan(&exists)
	if err != nil {
		return false, supportErrors.ErrInternal
	}
	return exists, nil
}

func (r *supportUserReadRepository) List(ctx context.Context, limit, offset int) ([]*domain.SupportUser, int, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM support_users WHERE deleted_at IS NULL`,
	).Scan(&total); err != nil {
		return nil, 0, supportErrors.ErrInternal
	}

	rows, err := r.pool.Query(ctx,
		`SELECT `+supportUserCols+` FROM support_users WHERE deleted_at IS NULL
         ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, 0, supportErrors.ErrInternal
	}
	defer rows.Close()

	out := make([]*domain.SupportUser, 0)
	for rows.Next() {
		u, err := scanSupportUser(rows)
		if err != nil {
			r.logger.Error("List scan failed", zap.Error(err))
			return nil, 0, supportErrors.ErrInternal
		}
		out = append(out, u)
	}
	return out, total, nil
}

// ListPendingPasswordResets retourne les agents ayant une demande de réinitialisation en attente.
func (r *supportUserReadRepository) ListPendingPasswordResets(ctx context.Context) ([]*domain.SupportUser, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+supportUserCols+` FROM support_users
         WHERE password_reset_requested_at IS NOT NULL AND deleted_at IS NULL
         ORDER BY password_reset_requested_at ASC`,
	)
	if err != nil {
		return nil, supportErrors.ErrInternal
	}
	defer rows.Close()

	out := make([]*domain.SupportUser, 0)
	for rows.Next() {
		u, err := scanSupportUser(rows)
		if err != nil {
			r.logger.Error("ListPendingPasswordResets scan failed", zap.Error(err))
			return nil, supportErrors.ErrInternal
		}
		out = append(out, u)
	}
	return out, nil
}

// ListAdminEmails retourne les emails de tous les admins actifs (pour notification).
func (r *supportUserReadRepository) ListAdminEmails(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT email FROM support_users
         WHERE role = 'admin' AND is_active AND deleted_at IS NULL`,
	)
	if err != nil {
		return nil, supportErrors.ErrInternal
	}
	defer rows.Close()

	out := make([]string, 0)
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			r.logger.Error("ListAdminEmails scan failed", zap.Error(err))
			return nil, supportErrors.ErrInternal
		}
		out = append(out, email)
	}
	return out, nil
}
