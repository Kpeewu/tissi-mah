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

type documentReviewReadImpl struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewDocumentReviewReadRepository(pool *pgxpool.Pool, logger *zap.Logger) i.DocumentReviewRepositoryRead {
	return &documentReviewReadImpl{pool: pool, logger: logger}
}

func (r *documentReviewReadImpl) GetByID(ctx context.Context, reviewID string) (*domain.DocumentReview, error) {
	r.logger.Debug("récupération de la revue par ID", zap.String("reviewID", reviewID))

	query := `SELECT review_id, user_document_id, vehicle_document_id,
	                 decision, reason_rejection, rejection_details,
	                 reviewed_by, reviewed_by_type, reviewed_at,
	                 notes, extracted_data
	          FROM document_reviews WHERE review_id = $1`

	review := &domain.DocumentReview{}
	err := r.pool.QueryRow(ctx, query, reviewID).Scan(
		&review.ReviewID, &review.UserDocumentID, &review.VehicleDocumentID,
		&review.Decision, &review.ReasonRejection, &review.RejectionDetails,
		&review.ReviewedBy, &review.ReviewedByType, &review.ReviewedAt,
		&review.Notes, &review.ExtractedData,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.logger.Debug("revue non trouvée", zap.String("reviewID", reviewID))
			return nil, fileErrors.ErrorReviewNotFound
		}
		r.logger.Error("erreur récupération revue par ID", zap.String("reviewID", reviewID), zap.Error(err))
		return nil, fileErrors.ErrorDataRetrievalFailed
	}
	return review, nil
}

func (r *documentReviewReadImpl) GetByUserDocumentID(ctx context.Context, userDocumentID string) ([]*domain.DocumentReview, error) {
	r.logger.Debug("récupération des revues par userDocumentID", zap.String("userDocumentID", userDocumentID))

	query := `SELECT review_id, user_document_id, vehicle_document_id,
	                 decision, reason_rejection, rejection_details,
	                 reviewed_by, reviewed_by_type, reviewed_at,
	                 notes, extracted_data
	          FROM document_reviews WHERE user_document_id = $1
	          ORDER BY reviewed_at DESC`

	return r.queryReviews(ctx, query, userDocumentID)
}

func (r *documentReviewReadImpl) GetByVehicleDocumentID(ctx context.Context, vehicleDocumentID string) ([]*domain.DocumentReview, error) {
	r.logger.Debug("récupération des revues par vehicleDocumentID", zap.String("vehicleDocumentID", vehicleDocumentID))

	query := `SELECT review_id, user_document_id, vehicle_document_id,
	                 decision, reason_rejection, rejection_details,
	                 reviewed_by, reviewed_by_type, reviewed_at,
	                 notes, extracted_data
	          FROM document_reviews WHERE vehicle_document_id = $1
	          ORDER BY reviewed_at DESC`

	return r.queryReviews(ctx, query, vehicleDocumentID)
}

func (r *documentReviewReadImpl) queryReviews(ctx context.Context, query string, id string) ([]*domain.DocumentReview, error) {
	r.logger.Debug("exécution de la requête queryReviews", zap.String("id", id))

	rows, err := r.pool.Query(ctx, query, id)
	if err != nil {
		r.logger.Error("erreur requête queryReviews", zap.String("id", id), zap.Error(err))
		return nil, fileErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	var reviews []*domain.DocumentReview
	for rows.Next() {
		review := &domain.DocumentReview{}
		err := rows.Scan(
			&review.ReviewID, &review.UserDocumentID, &review.VehicleDocumentID,
			&review.Decision, &review.ReasonRejection, &review.RejectionDetails,
			&review.ReviewedBy, &review.ReviewedByType, &review.ReviewedAt,
			&review.Notes, &review.ExtractedData,
		)
		if err != nil {
			r.logger.Error("erreur scan revue", zap.String("id", id), zap.Error(err))
			return nil, fileErrors.ErrorDataRetrievalFailed
		}
		reviews = append(reviews, review)
	}
	return reviews, nil
}
