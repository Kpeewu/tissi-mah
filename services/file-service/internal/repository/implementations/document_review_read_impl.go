package implementations

import (
	"context"
	"errors"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/domain"
	i "github.com/Kpeewu/tissi-mah/services/file-service/internal/repository/interfaces"
	fileErrors "github.com/Kpeewu/tissi-mah/services/file-service/pkg/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type documentReviewReadImpl struct {
	pool *pgxpool.Pool
}

func NewDocumentReviewReadRepository(pool *pgxpool.Pool) i.DocumentReviewRepositoryRead {
	return &documentReviewReadImpl{pool: pool}
}

func (r *documentReviewReadImpl) GetByID(ctx context.Context, reviewID string) (*domain.DocumentReview, error) {
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
			return nil, fileErrors.ErrorReviewNotFound
		}
		return nil, fileErrors.ErrorDataRetrievalFailed
	}
	return review, nil
}

func (r *documentReviewReadImpl) GetByUserDocumentID(ctx context.Context, userDocumentID string) ([]*domain.DocumentReview, error) {
	query := `SELECT review_id, user_document_id, vehicle_document_id,
	                 decision, reason_rejection, rejection_details,
	                 reviewed_by, reviewed_by_type, reviewed_at,
	                 notes, extracted_data
	          FROM document_reviews WHERE user_document_id = $1
	          ORDER BY reviewed_at DESC`

	return r.queryReviews(ctx, query, userDocumentID)
}

func (r *documentReviewReadImpl) GetByVehicleDocumentID(ctx context.Context, vehicleDocumentID string) ([]*domain.DocumentReview, error) {
	query := `SELECT review_id, user_document_id, vehicle_document_id,
	                 decision, reason_rejection, rejection_details,
	                 reviewed_by, reviewed_by_type, reviewed_at,
	                 notes, extracted_data
	          FROM document_reviews WHERE vehicle_document_id = $1
	          ORDER BY reviewed_at DESC`

	return r.queryReviews(ctx, query, vehicleDocumentID)
}

func (r *documentReviewReadImpl) queryReviews(ctx context.Context, query string, id string) ([]*domain.DocumentReview, error) {
	rows, err := r.pool.Query(ctx, query, id)
	if err != nil {
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
			return nil, fileErrors.ErrorDataRetrievalFailed
		}
		reviews = append(reviews, review)
	}
	return reviews, nil
}
