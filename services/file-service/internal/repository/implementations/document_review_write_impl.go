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
	          (review_id, user_id, document_type, logical_document_type,
	           user_document_id, second_user_document_id, vehicle_document_id,
	           attempt_number, previous_review_id,
	           status, decision, reason_rejection, rejection_details,
	           reviewed_by, review_type, reviewed_at,
	           notes, extracted_data,
	           submitted_at, created_at, updated_at)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11,
	                  $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
	          RETURNING review_id`

	var reviewID string
	err := r.pool.QueryRow(ctx, query,
		review.ReviewID, review.UserID, review.DocumentType, review.LogicalDocumentType,
		review.UserDocumentID, review.SecondUserDocumentID, review.VehicleDocumentID,
		review.AttemptNumber, review.PreviousReviewID,
		review.Status, review.Decision, review.ReasonRejection, review.RejectionDetails,
		review.ReviewedBy, review.ReviewType, review.ReviewedAt,
		review.Notes, review.ExtractedData,
		review.SubmittedAt, review.CreatedAt, review.UpdatedAt,
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
	           status = $2, decision = $3, reason_rejection = $4, rejection_details = $5,
	           reviewed_by = $6, review_type = $7, reviewed_at = $8,
	           notes = $9,
	           submitted_at = $10, updated_at = $11,
	           logical_document_type = $12, second_user_document_id = $13
	          WHERE review_id = $1`
	args := []interface{}{
		review.ReviewID,
		review.Status, review.Decision, review.ReasonRejection, review.RejectionDetails,
		review.ReviewedBy, review.ReviewType, review.ReviewedAt,
		review.Notes,
		review.SubmittedAt, review.UpdatedAt,
		review.LogicalDocumentType, review.SecondUserDocumentID,
	}

	// Compare-and-set : une review ne peut être complétée qu'une seule fois.
	// Deux agents support validant le même document en concurrence → le second
	// échoue proprement (ErrorReviewAlreadyCompleted) au lieu d'écraser la décision.
	completing := review.Status == "completed"
	if completing {
		query += ` AND status <> 'completed'`
	}

	result, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		r.logger.Error("erreur mise à jour revue", zap.String("reviewID", review.ReviewID), zap.Error(err))
		return fileErrors.ErrorInternalServer
	}

	if result.RowsAffected() == 0 {
		if completing {
			// Distinguer "introuvable" de "déjà complétée"
			var exists bool
			if scanErr := r.pool.QueryRow(ctx,
				`SELECT EXISTS(SELECT 1 FROM document_reviews WHERE review_id = $1)`,
				review.ReviewID,
			).Scan(&exists); scanErr == nil && exists {
				r.logger.Warn("revue déjà complétée (validation concurrente)", zap.String("reviewID", review.ReviewID))
				return fileErrors.ErrorReviewAlreadyCompleted
			}
		}
		r.logger.Debug("revue non trouvée pour mise à jour", zap.String("reviewID", review.ReviewID))
		return fileErrors.ErrorReviewNotFound
	}

	r.logger.Info("revue mise à jour avec succès", zap.String("reviewID", review.ReviewID))
	return nil
}
