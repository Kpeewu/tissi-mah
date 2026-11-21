package implementations

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/domain"
	notifErrors "github.com/Kpeewu/tissi-mah/services/notification-service/pkg/errors"
)

type DeviceTokenRepositoryImpl struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewDeviceTokenRepository(pool *pgxpool.Pool, logger *zap.Logger) *DeviceTokenRepositoryImpl {
	return &DeviceTokenRepositoryImpl{pool: pool, logger: logger}
}

func (r *DeviceTokenRepositoryImpl) GetActiveByUserID(ctx context.Context, userID string) ([]*domain.UserDeviceToken, error) {
	query := `SELECT token_id, user_id, fcm_token, platform, device_name, is_active, last_used_at, created_at, updated_at
		FROM user_device_tokens
		WHERE user_id = $1 AND is_active = true`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		r.logger.Error("failed to get device tokens", zap.String("user_id", userID), zap.Error(err))
		return nil, notifErrors.ErrorDataRetrievalFail
	}
	defer rows.Close()

	var tokens []*domain.UserDeviceToken
	for rows.Next() {
		var t domain.UserDeviceToken
		if err := rows.Scan(
			&t.TokenID, &t.UserID, &t.FCMToken, &t.Platform, &t.DeviceName,
			&t.IsActive, &t.LastUsedAt, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, notifErrors.ErrorDataRetrievalFail
		}
		tokens = append(tokens, &t)
	}

	return tokens, nil
}

func (r *DeviceTokenRepositoryImpl) Upsert(ctx context.Context, token *domain.UserDeviceToken) (string, error) {
	query := `INSERT INTO user_device_tokens (token_id, user_id, fcm_token, platform, device_name, is_active, last_used_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, true, NOW())
		ON CONFLICT (fcm_token) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			platform = EXCLUDED.platform,
			device_name = EXCLUDED.device_name,
			is_active = true,
			last_used_at = NOW(),
			updated_at = NOW()
		RETURNING token_id`

	var tokenID string
	err := r.pool.QueryRow(ctx, query, token.UserID, token.FCMToken, token.Platform, token.DeviceName).Scan(&tokenID)
	if err != nil {
		r.logger.Error("failed to upsert device token", zap.Error(err))
		return "", notifErrors.ErrorDataInsertFail
	}

	return tokenID, nil
}

func (r *DeviceTokenRepositoryImpl) InvalidateByFCMToken(ctx context.Context, fcmToken string) error {
	query := `UPDATE user_device_tokens SET is_active = false, updated_at = NOW() WHERE fcm_token = $1`

	_, err := r.pool.Exec(ctx, query, fcmToken)
	if err != nil {
		r.logger.Error("failed to invalidate device token", zap.String("fcm_token", fcmToken), zap.Error(err))
		return notifErrors.ErrorDataUpdateFail
	}

	return nil
}

func (r *DeviceTokenRepositoryImpl) DeleteByFCMToken(ctx context.Context, fcmToken string) error {
	query := `DELETE FROM user_device_tokens WHERE fcm_token = $1`

	_, err := r.pool.Exec(ctx, query, fcmToken)
	if err != nil {
		r.logger.Error("failed to delete device token", zap.String("fcm_token", fcmToken), zap.Error(err))
		return notifErrors.ErrorDataDeleteFail
	}

	return nil
}
