package implementations

import (
	"context"
	"errors"

	"github.com/Kpeewu/tissi-mah/services/support-service/internal/domain"
	i "github.com/Kpeewu/tissi-mah/services/support-service/internal/repository/interfaces"
	supportErrors "github.com/Kpeewu/tissi-mah/services/support-service/pkg/errors"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type supportUserWriteRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewSupportUserWriteRepository(pool *pgxpool.Pool, logger *zap.Logger) i.SupportUserWriteRepository {
	return &supportUserWriteRepository{pool: pool, logger: logger}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

func (r *supportUserWriteRepository) Create(ctx context.Context, u *domain.SupportUser) error {
	_, err := r.pool.Exec(ctx, `
        INSERT INTO support_users (
            user_id, email, password_hash, first_name, last_name, role,
            is_active, must_change_password, password_changed_at
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8, NOW())
    `, u.UserID, u.Email, u.PasswordHash, u.FirstName, u.LastName, u.Role,
		u.IsActive, u.MustChangePassword)
	if err != nil {
		if isUniqueViolation(err) {
			return supportErrors.ErrEmailAlreadyExists
		}
		r.logger.Error("Create support user failed", zap.Error(err))
		return supportErrors.ErrInternal
	}
	return nil
}

func (r *supportUserWriteRepository) UpdatePassword(ctx context.Context, userID, hash string, mustChange bool) error {
	tag, err := r.pool.Exec(ctx, `
        UPDATE support_users
        SET password_hash = $1, must_change_password = $2, password_changed_at = NOW()
        WHERE user_id = $3 AND deleted_at IS NULL
    `, hash, mustChange, userID)
	if err != nil {
		r.logger.Error("UpdatePassword failed", zap.Error(err))
		return supportErrors.ErrInternal
	}
	if tag.RowsAffected() == 0 {
		return supportErrors.ErrUserNotFound
	}
	return nil
}

func (r *supportUserWriteRepository) UpdateEmail(ctx context.Context, userID, newEmail string) error {
	tag, err := r.pool.Exec(ctx, `
        UPDATE support_users
        SET email = $1, email_changed_at = NOW()
        WHERE user_id = $2 AND deleted_at IS NULL
    `, newEmail, userID)
	if err != nil {
		if isUniqueViolation(err) {
			return supportErrors.ErrEmailAlreadyExists
		}
		r.logger.Error("UpdateEmail failed", zap.Error(err))
		return supportErrors.ErrInternal
	}
	if tag.RowsAffected() == 0 {
		return supportErrors.ErrUserNotFound
	}
	return nil
}

func (r *supportUserWriteRepository) UpdateName(ctx context.Context, userID, firstName, lastName string) error {
	tag, err := r.pool.Exec(ctx, `
        UPDATE support_users
        SET first_name = $1, last_name = $2
        WHERE user_id = $3 AND deleted_at IS NULL
    `, firstName, lastName, userID)
	if err != nil {
		r.logger.Error("UpdateName failed", zap.Error(err))
		return supportErrors.ErrInternal
	}
	if tag.RowsAffected() == 0 {
		return supportErrors.ErrUserNotFound
	}
	return nil
}

func (r *supportUserWriteRepository) UpdateRole(ctx context.Context, userID, role string) error {
	tag, err := r.pool.Exec(ctx, `
        UPDATE support_users
        SET role = $1
        WHERE user_id = $2 AND deleted_at IS NULL
    `, role, userID)
	if err != nil {
		r.logger.Error("UpdateRole failed", zap.Error(err))
		return supportErrors.ErrInternal
	}
	if tag.RowsAffected() == 0 {
		return supportErrors.ErrUserNotFound
	}
	return nil
}

func (r *supportUserWriteRepository) Deactivate(ctx context.Context, userID string) error {
	tag, err := r.pool.Exec(ctx, `
        UPDATE support_users
        SET is_active = FALSE
        WHERE user_id = $1 AND deleted_at IS NULL
    `, userID)
	if err != nil {
		r.logger.Error("Deactivate failed", zap.Error(err))
		return supportErrors.ErrInternal
	}
	if tag.RowsAffected() == 0 {
		return supportErrors.ErrUserNotFound
	}
	return nil
}

func (r *supportUserWriteRepository) Activate(ctx context.Context, userID string) error {
	tag, err := r.pool.Exec(ctx, `
        UPDATE support_users
        SET is_active = TRUE
        WHERE user_id = $1 AND deleted_at IS NULL
    `, userID)
	if err != nil {
		r.logger.Error("Activate failed", zap.Error(err))
		return supportErrors.ErrInternal
	}
	if tag.RowsAffected() == 0 {
		return supportErrors.ErrUserNotFound
	}
	return nil
}

func (r *supportUserWriteRepository) SoftDelete(ctx context.Context, userID string) error {
	tag, err := r.pool.Exec(ctx, `
        UPDATE support_users
        SET deleted_at = NOW()
        WHERE user_id = $1 AND deleted_at IS NULL
    `, userID)
	if err != nil {
		r.logger.Error("SoftDelete failed", zap.Error(err))
		return supportErrors.ErrInternal
	}
	if tag.RowsAffected() == 0 {
		return supportErrors.ErrUserNotFound
	}
	return nil
}

func (r *supportUserWriteRepository) SetPasswordResetRequested(ctx context.Context, userID string) error {
	tag, err := r.pool.Exec(ctx, `
        UPDATE support_users
        SET password_reset_requested_at = NOW()
        WHERE user_id = $1 AND deleted_at IS NULL
    `, userID)
	if err != nil {
		r.logger.Error("SetPasswordResetRequested failed", zap.Error(err))
		return supportErrors.ErrInternal
	}
	if tag.RowsAffected() == 0 {
		return supportErrors.ErrUserNotFound
	}
	return nil
}

func (r *supportUserWriteRepository) ClearPasswordResetRequested(ctx context.Context, userID string) error {
	tag, err := r.pool.Exec(ctx, `
        UPDATE support_users
        SET password_reset_requested_at = NULL
        WHERE user_id = $1 AND deleted_at IS NULL
    `, userID)
	if err != nil {
		r.logger.Error("ClearPasswordResetRequested failed", zap.Error(err))
		return supportErrors.ErrInternal
	}
	if tag.RowsAffected() == 0 {
		return supportErrors.ErrUserNotFound
	}
	return nil
}

// ClaimPasswordResetRequest n'efface le flag que s'il est présent (UPDATE conditionnel
// atomique). 0 ligne affectée = demande déjà traitée par un autre admin.
func (r *supportUserWriteRepository) ClaimPasswordResetRequest(ctx context.Context, userID string) error {
	tag, err := r.pool.Exec(ctx, `
        UPDATE support_users
        SET password_reset_requested_at = NULL
        WHERE user_id = $1 AND password_reset_requested_at IS NOT NULL AND deleted_at IS NULL
    `, userID)
	if err != nil {
		r.logger.Error("ClaimPasswordResetRequest failed", zap.Error(err))
		return supportErrors.ErrInternal
	}
	if tag.RowsAffected() == 0 {
		return supportErrors.ErrResetAlreadyProcessed
	}
	return nil
}
