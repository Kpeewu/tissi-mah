package implementations

import (
	"context"
	"errors"

	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/domain"
	i "github.com/Kpeewu/tissi-mah/services/auth-service/internal/repository/interfaces"
	authErrors "github.com/Kpeewu/tissi-mah/services/auth-service/pkg/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type authWriteRepositoryImpl struct {
	pool *pgxpool.Pool
}

func NewAuthWriteRepository(pool *pgxpool.Pool) i.AuthRepositoryWrite {
	return &authWriteRepositoryImpl{
		pool: pool,
	}
}

// create user account
func (r *authWriteRepositoryImpl) Create(ctx context.Context, auth *domain.Auth) (string, error) {

	query := `INSERT INTO auth (auth_id, firebase_id, email, phone_number, suspension_end_date) 
				VALUES ($1, $2, $3, $4, $5) RETURNING auth_id`

	var authID string
	err := r.pool.QueryRow(ctx, query, auth.AuthID, auth.FirebaseID, auth.Email, auth.PhoneNumber, auth.SuspensionEndDate).Scan(&authID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", authErrors.ErrorDataRetrievalFailed
		}
		return "", authErrors.ErrorInternalServer
	}

	return authID, nil
}

// update user auth informations
func (r *authWriteRepositoryImpl) Update(ctx context.Context, auth *domain.Auth) (*domain.Auth, error) {
	query := `UPDATE auth SET 
					email = $1, phone_number = $2, is_active = $3, is_suspended = $4, suspension_end_date = $5
				WHERE auth_id = $6
				RETURNING auth_id, firebase_id, phone_number, is_active, is_suspended, suspension_end_date, created_at, updated_at, deleted_at`

	err := r.pool.QueryRow(ctx, query, auth.Email, auth.PhoneNumber, auth.IsActive, auth.IsSuspended, auth.SuspensionEndDate, auth.AuthID).Scan(
		&auth.AuthID,
		&auth.FirebaseID,
		&auth.Email,
		&auth.PhoneNumber,
		&auth.IsActive,
		&auth.IsSuspended,
		&auth.SuspensionEndDate,
		&auth.CreatedAt,
		&auth.UpdatedAt,
		&auth.DeletedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, authErrors.ErrorUserNotFound
		}
		return nil, authErrors.ErrorInternalServer
	}

	return auth, nil
}

// delete user auth informations
func (r *authWriteRepositoryImpl) Delete(ctx context.Context, auth *domain.Auth) error {

	auth.AnonymizeAndDelete()

	query := `UPDATE auth SET
					firebase_id = $1, phone_number = $2, email = $3, is_active = $4, deleted_at = $5 WHERE auth_id = $6`

	_, err := r.pool.Exec(ctx, query, auth.FirebaseID, auth.PhoneNumber, auth.Email, auth.IsActive, auth.DeletedAt, auth.AuthID)

	if err != nil {
		return authErrors.ErrorCantDeleteAccount
	}
	return nil
}
