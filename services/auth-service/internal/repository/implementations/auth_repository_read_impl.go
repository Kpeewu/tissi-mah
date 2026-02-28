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

type authReadRepositoryImpl struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewAuthReadRepository(pool *pgxpool.Pool, logger *zap.Logger) i.AuthRepositoryRead {
	return &authReadRepositoryImpl{
		pool:   pool,
		logger: logger,
	}
}

func (r *authReadRepositoryImpl) GetByAuthID(ctx context.Context, authID string) (*domain.Auth, error) {
	r.logger.Debug("get auth by authID", zap.String("authID", authID))

	query := `SELECT auth_id, firebase_id, email, phone_number,
            		 is_active, is_suspended, suspension_end_date,
                	 created_at, updated_at, deleted_at
			  FROM auth WHERE auth_id = $1 AND deleted_at IS NULL`

	auth := &domain.Auth{}

	err := r.pool.QueryRow(ctx, query, authID).Scan(
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
			r.logger.Debug("auth not found by authID", zap.String("authID", authID))
			return nil, authErrors.ErrorUserNotFound
		}
		r.logger.Error("get auth by authID failed", zap.Error(err), zap.String("authID", authID))
		return nil, authErrors.ErrorDataRetrievalFailed
	}

	return auth, nil
}

// get user by his firebaase id
func (r *authReadRepositoryImpl) GetByFirebaseID(ctx context.Context, firebaseID string) (*domain.Auth, error) {
	r.logger.Debug("get auth by firebaseID", zap.String("firebaseID", firebaseID))

	query := `SELECT auth_id, firebase_id, email, phone_number,
            		 is_active, is_suspended, suspension_end_date,
                	 created_at, updated_at, deleted_at
			  FROM auth WHERE firebase_id = $1 AND deleted_at IS NULL`

	auth := &domain.Auth{}

	err := r.pool.QueryRow(ctx, query, firebaseID).Scan(
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
			r.logger.Debug("auth not found by firebaseID", zap.String("firebaseID", firebaseID))
			return nil, authErrors.ErrorUserNotFound
		}
		r.logger.Error("get auth by firebaseID failed", zap.Error(err), zap.String("firebaseID", firebaseID))
		return nil, authErrors.ErrorDataRetrievalFailed
	}

	return auth, nil
}

// get user by his email
func (r *authReadRepositoryImpl) GetByEmail(ctx context.Context, email string) (*domain.Auth, error) {
	r.logger.Debug("get auth by email", zap.String("email", email))

	query := `SELECT auth_id, firebase_id, email, phone_number,
            		 is_active, is_suspended, suspension_end_date,
                	 created_at, updated_at, deleted_at
			  FROM auth WHERE email = $1 AND deleted_at IS NULL`

	auth := &domain.Auth{}

	err := r.pool.QueryRow(ctx, query, email).Scan(
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
			r.logger.Debug("auth not found by email", zap.String("email", email))
			return nil, authErrors.ErrorUserNotFound
		}
		r.logger.Error("get auth by email failed", zap.Error(err), zap.String("email", email))
		return nil, authErrors.ErrorDataRetrievalFailed
	}

	return auth, nil
}

// get user by his phone number
func (r *authReadRepositoryImpl) GetByPhoneNumber(ctx context.Context, phoneNumber string) (*domain.Auth, error) {
	r.logger.Debug("get auth by phone", zap.String("phone", phoneNumber))

	query := `SELECT auth_id, firebase_id, email, phone_number,
            		 is_active, is_suspended, suspension_end_date,
                	 created_at, updated_at, deleted_at
			  FROM auth WHERE phone_number = $1 AND deleted_at IS NULL`

	auth := &domain.Auth{}

	err := r.pool.QueryRow(ctx, query, phoneNumber).Scan(
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
			r.logger.Debug("auth not found by phone", zap.String("phone", phoneNumber))
			return nil, authErrors.ErrorUserNotFound
		}
		r.logger.Error("get auth by phone failed", zap.Error(err), zap.String("phone", phoneNumber))
		return nil, authErrors.ErrorDataRetrievalFailed
	}

	return auth, nil
}

func (r *authReadRepositoryImpl) EmailExists(ctx context.Context, email string) (bool, error) {
	r.logger.Debug("checking email exists", zap.String("email", email))

	query := `SELECT EXISTS(SELECT 1 FROM auth WHERE email = $1 AND deleted_at IS NULL)`

	var exists bool
	err := r.pool.QueryRow(ctx, query, email).Scan(&exists)

	if err != nil {
		r.logger.Error("email exists check failed", zap.Error(err), zap.String("email", email))
		return false, authErrors.ErrorDataRetrievalFailed
	}

	if exists {
		r.logger.Debug("email already taken", zap.String("email", email))
		return true, authErrors.ErrorEmailNotAvailable
	}

	return false, nil
}

// check if phone number already used
func (r *authReadRepositoryImpl) PhoneNumberExists(ctx context.Context, phoneNumber string) (bool, error) {
	r.logger.Debug("checking phone exists", zap.String("phone", phoneNumber))

	query := `SELECT EXISTS(SELECT 1 FROM auth WHERE phone_number = $1 AND deleted_at IS NULL)`

	var exists bool
	err := r.pool.QueryRow(ctx, query, phoneNumber).Scan(&exists)

	if err != nil {
		r.logger.Error("phone exists check failed", zap.Error(err), zap.String("phone", phoneNumber))
		return false, authErrors.ErrorDataRetrievalFailed
	}

	if exists {
		r.logger.Debug("phone already taken", zap.String("phone", phoneNumber))
		return true, authErrors.ErrorPhoneNumberNotAvailable
	}

	return false, nil
}
