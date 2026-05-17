package implementations

import (
	"context"
	"errors"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/domain"
	i "github.com/Kpeewu/tissi-mah/services/file-service/internal/repository/interfaces"
	fileErrors "github.com/Kpeewu/tissi-mah/services/file-service/pkg/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type documentReviewWriteImpl struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewDocumentReviewWriteRepository(pool *pgxpool.Pool, logger *zap.Logger) i.DocumentReviewRepositoryWrite {
	return &documentReviewWriteImpl{pool: pool, logger: logger}
}

func (r *documentReviewWriteImpl) Create(ctx context.Context, review *domain.DocumentReview) (string, error) {
	r.logger.Debug("création d'une revue de document", zap.String("reviewID", review.ReviewID))

	query := `INSERT INTO document_reviews
	          (review_id, user_id, document_type, user_document_id, vehicle_document_id,
	           persona_inquiry_id, persona_template_id, persona_session_token, session_expires_at,
	           webhook_event_type, webhook_received_at, persona_raw_payload,
	           attempt_number, previous_review_id,
	           status, decision, reason_rejection, rejection_details,
	           reviewed_by, review_type,
	           notes, extracted_data,
	           submitted_at, updated_at)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
	                  $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24)
	          RETURNING review_id`

	var reviewID string
	err := r.pool.QueryRow(ctx, query,
		review.ReviewID, review.UserID, review.DocumentType, review.UserDocumentID, review.VehicleDocumentID,
		review.PersonaInquiryID, review.PersonaTemplateID, review.PersonaSessionToken, review.SessionExpiresAt,
		review.WebhookEventType, review.WebhookReceivedAt, review.PersonaRawPayload,
		review.AttemptNumber, review.PreviousReviewID,
		review.Status, review.Decision, review.ReasonRejection, review.RejectionDetails,
		review.ReviewedBy, review.ReviewType,
		review.Notes, review.ExtractedData,
		review.SubmittedAt, review.UpdatedAt,
	).Scan(&reviewID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.logger.Error("création revue : aucune ligne retournée", zap.String("reviewID", review.ReviewID), zap.Error(err))
			return "", fileErrors.ErrorDataRetrievalFailed
		}
		r.logger.Error("erreur création revue", zap.String("reviewID", review.ReviewID), zap.Error(err))
		return "", fileErrors.ErrorInternalServer
	}

	r.logger.Info("revue créée avec succès", zap.String("reviewID", reviewID))
	return reviewID, nil
}

func (r *documentReviewWriteImpl) Update(ctx context.Context, review *domain.DocumentReview) error {
	r.logger.Debug("mise à jour d'une revue de document", zap.String("reviewID", review.ReviewID))

	query := `UPDATE document_reviews SET
	           persona_session_token = $2, session_expires_at = $3,
	           webhook_event_type = $4, webhook_received_at = $5, persona_raw_payload = $6,
	           status = $7, decision = $8, reason_rejection = $9, rejection_details = $10,
	           reviewed_by = $11, review_type = $12, reviewed_at = $13,
	           notes = $14,
	           submitted_at = $15, updated_at = $16
	          WHERE review_id = $1`

	result, err := r.pool.Exec(ctx, query,
		review.ReviewID,
		review.PersonaSessionToken, review.SessionExpiresAt,
		review.WebhookEventType, review.WebhookReceivedAt, review.PersonaRawPayload,
		review.Status, review.Decision, review.ReasonRejection, review.RejectionDetails,
		review.ReviewedBy, review.ReviewType, review.ReviewedAt,
		review.Notes,
		review.SubmittedAt, review.UpdatedAt,
	)
	if err != nil {
		r.logger.Error("erreur mise à jour revue", zap.String("reviewID", review.ReviewID), zap.Error(err))
		return fileErrors.ErrorInternalServer
	}

	if result.RowsAffected() == 0 {
		r.logger.Debug("revue non trouvée pour mise à jour", zap.String("reviewID", review.ReviewID))
		return fileErrors.ErrorReviewNotFound
	}

	r.logger.Info("revue mise à jour avec succès", zap.String("reviewID", review.ReviewID))
	return nil
}
