package implementations

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/domain"
	notifErrors "github.com/Kpeewu/tissi-mah/services/notification-service/pkg/errors"
)

type PreferenceRepositoryImpl struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewPreferenceRepository(pool *pgxpool.Pool, logger *zap.Logger) *PreferenceRepositoryImpl {
	return &PreferenceRepositoryImpl{pool: pool, logger: logger}
}

// GetByUserID retourne les préférences d'un utilisateur.
// Si aucune entrée n'existe, retourne les valeurs par défaut (tout activé).
func (r *PreferenceRepositoryImpl) GetByUserID(ctx context.Context, userID string) (*domain.UserNotificationPreference, error) {
	query := `SELECT preference_id, user_id, push_enabled, email_enabled, created_at, updated_at
		FROM user_notification_preferences
		WHERE user_id = $1`

	var pref domain.UserNotificationPreference
	err := r.pool.QueryRow(ctx, query, userID).Scan(
		&pref.PreferenceID, &pref.UserID, &pref.PushEnabled, &pref.EmailEnabled,
		&pref.CreatedAt, &pref.UpdatedAt,
	)
	if err != nil {
		// Pas d'entrée → valeurs par défaut
		return &domain.UserNotificationPreference{
			UserID:       userID,
			PushEnabled:  true,
			EmailEnabled: true,
		}, nil
	}

	return &pref, nil
}

func (r *PreferenceRepositoryImpl) Upsert(ctx context.Context, pref *domain.UserNotificationPreference) error {
	query := `INSERT INTO user_notification_preferences (preference_id, user_id, push_enabled, email_enabled)
		VALUES (gen_random_uuid(), $1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE SET
			push_enabled = EXCLUDED.push_enabled,
			email_enabled = EXCLUDED.email_enabled,
			updated_at = NOW()`

	_, err := r.pool.Exec(ctx, query, pref.UserID, pref.PushEnabled, pref.EmailEnabled)
	if err != nil {
		r.logger.Error("failed to upsert preference", zap.String("user_id", pref.UserID), zap.Error(err))
		return notifErrors.ErrorDataInsertFail
	}

	return nil
}
