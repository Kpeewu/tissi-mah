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
	          (review_id, user_document_id, vehicle_document_id,
	           decision, reason_rejection, rejection_details,
	           reviewed_by, reviewed_by_type,
	           notes, extracted_data)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	          RETURNING review_id`

	var reviewID string
	err := r.pool.QueryRow(ctx, query,
		review.ReviewID, review.UserDocumentID, review.VehicleDocumentID,
		review.Decision, review.ReasonRejection, review.RejectionDetails,
		review.ReviewedBy, review.ReviewedByType,
		review.Notes, review.ExtractedData,
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
