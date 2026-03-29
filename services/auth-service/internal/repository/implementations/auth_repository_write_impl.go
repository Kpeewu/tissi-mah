package implementations

import (
	"context"
	"errors"

	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/domain"
	i "github.com/Kpeewu/tissi-mah/services/auth-service/internal/repository/interfaces"
	authErrors "github.com/Kpeewu/tissi-mah/services/auth-service/pkg/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type authWriteRepositoryImpl struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewAuthWriteRepository(pool *pgxpool.Pool, logger *zap.Logger) i.AuthRepositoryWrite {
	return &authWriteRepositoryImpl{
		pool:   pool,
		logger: logger,
	}
}

// create user account
func (r *authWriteRepositoryImpl) Create(ctx context.Context, auth *domain.Auth) (string, error) {
	r.logger.Debug("creating auth record",
		zap.String("authID", auth.AuthID),
		zap.String("firebaseID", auth.FirebaseID),
		zap.Boolp("hasEmail", boolPtr(auth.Email != nil)),
		zap.Boolp("hasPhone", boolPtr(auth.PhoneNumber != nil)),
	)

	query := `INSERT INTO auth (auth_id, firebase_id, email, phone_number, suspension_end_date)
				VALUES ($1, $2, $3, $4, $5) RETURNING auth_id`

	var authID string
	err := r.pool.QueryRow(ctx, query, auth.AuthID, auth.FirebaseID, auth.Email, auth.PhoneNumber, auth.SuspensionEndDate).Scan(&authID)

	if err != nil {
		r.logger.Error("insert auth failed", zap.Error(err),
			zap.String("authID", auth.AuthID),
			zap.String("firebaseID", auth.FirebaseID),
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", authErrors.ErrorDataRetrievalFailed
		}
		return "", authErrors.ErrorInternalServer
	}

	r.logger.Info("auth record created", zap.String("authID", authID))
	return authID, nil
}

// update user auth informations
func (r *authWriteRepositoryImpl) Update(ctx context.Context, auth *domain.Auth) (*domain.Auth, error) {
	r.logger.Debug("updating auth record", zap.String("authID", auth.AuthID))

	query := `UPDATE auth SET
					email = $1, phone_number = $2, is_active = $3, is_suspended = $4, suspension_end_date = $5
				WHERE auth_id = $6
				RETURNING auth_id, firebase_id, email, phone_number, is_active, is_suspended, suspension_end_date, created_at, updated_at, deleted_at`

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
		r.logger.Error("update auth failed", zap.Error(err), zap.String("authID", auth.AuthID))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, authErrors.ErrorUserNotFound
		}
		return nil, authErrors.ErrorInternalServer
	}

	r.logger.Info("auth record updated", zap.String("authID", auth.AuthID))
	return auth, nil
}

// delete user auth informations
func (r *authWriteRepositoryImpl) Delete(ctx context.Context, auth *domain.Auth) error {
	r.logger.Debug("deleting auth record (anonymize)", zap.String("authID", auth.AuthID))

	auth.AnonymizeAndDelete()

	query := `UPDATE auth SET
					firebase_id = $1, phone_number = $2, email = $3, is_active = $4, deleted_at = $5 WHERE auth_id = $6`

	// firebase_id doit être NULL (pas string vide) pour ne pas violer la contrainte UNIQUE
	_, err := r.pool.Exec(ctx, query, nil, auth.PhoneNumber, auth.Email, auth.IsActive, auth.DeletedAt, auth.AuthID)

	if err != nil {
		r.logger.Error("delete auth failed", zap.Error(err), zap.String("authID", auth.AuthID))
		return authErrors.ErrorCantDeleteAccount
	}

	r.logger.Info("auth record deleted", zap.String("authID", auth.AuthID))
	return nil
}

func boolPtr(v bool) *bool { return &v }
