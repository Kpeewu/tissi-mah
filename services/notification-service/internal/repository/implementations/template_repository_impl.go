package implementations

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/domain"
	notifErrors "github.com/Kpeewu/tissi-mah/services/notification-service/pkg/errors"
)

type TemplateRepositoryImpl struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewTemplateRepository(pool *pgxpool.Pool, logger *zap.Logger) *TemplateRepositoryImpl {
	return &TemplateRepositoryImpl{pool: pool, logger: logger}
}

// GetByEventTypeAndChannel cherche le template pour la langue demandée, avec fallback sur "fr".
func (r *TemplateRepositoryImpl) GetByEventTypeAndChannel(ctx context.Context, eventType, channel, languageCode string) (*domain.Template, error) {
	query := `SELECT template_id, event_type, channel, language_code, title, subject, body, body_html, is_active, created_at, updated_at
		FROM notification_templates
		WHERE event_type = $1 AND channel = $2 AND language_code = $3 AND is_active = true`

	var t domain.Template
	err := r.pool.QueryRow(ctx, query, eventType, channel, languageCode).Scan(
		&t.TemplateID, &t.EventType, &t.Channel, &t.LanguageCode,
		&t.Title, &t.Subject, &t.Body, &t.BodyHTML,
		&t.IsActive, &t.CreatedAt, &t.UpdatedAt,
	)
	if err == nil {
		return &t, nil
	}

	// Fallback sur "fr" si la langue demandée n'est pas trouvée
	if languageCode != "fr" {
		r.logger.Debug("template fallback to fr",
			zap.String("event_type", eventType),
			zap.String("channel", channel),
			zap.String("requested_lang", languageCode),
		)
		return r.GetByEventTypeAndChannel(ctx, eventType, channel, "fr")
	}

	r.logger.Error("template not found",
		zap.String("event_type", eventType),
		zap.String("channel", channel),
		zap.Error(err),
	)
	return nil, notifErrors.ErrorTemplateNotFound
}
