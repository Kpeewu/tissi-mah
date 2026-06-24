package implementations

import (
	"context"
	"errors"
	"fmt"

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

const reviewSelectColumns = `review_id, user_id, document_type, logical_document_type, user_document_id, second_user_document_id, vehicle_document_id,
	persona_inquiry_id, persona_template_id, persona_session_token, session_expires_at,
	webhook_event_type, webhook_received_at, persona_raw_payload,
	attempt_number, previous_review_id,
	status, decision, reason_rejection, rejection_details,
	reviewed_by, review_type, reviewed_at,
	notes, extracted_data,
	submitted_at, updated_at`

func scanReview(row interface {
	Scan(dest ...any) error
}, review *domain.DocumentReview) error {
	return row.Scan(
		&review.ReviewID, &review.UserID, &review.DocumentType, &review.LogicalDocumentType,
		&review.UserDocumentID, &review.SecondUserDocumentID, &review.VehicleDocumentID,
		&review.PersonaInquiryID, &review.PersonaTemplateID, &review.PersonaSessionToken, &review.SessionExpiresAt,
		&review.WebhookEventType, &review.WebhookReceivedAt, &review.PersonaRawPayload,
		&review.AttemptNumber, &review.PreviousReviewID,
		&review.Status, &review.Decision, &review.ReasonRejection, &review.RejectionDetails,
		&review.ReviewedBy, &review.ReviewType, &review.ReviewedAt,
		&review.Notes, &review.ExtractedData,
		&review.SubmittedAt, &review.UpdatedAt,
	)
}

func (r *documentReviewReadImpl) GetByID(ctx context.Context, reviewID string) (*domain.DocumentReview, error) {
	r.logger.Debug("récupération de la revue par ID", zap.String("reviewID", reviewID))

	query := `SELECT ` + reviewSelectColumns + ` FROM document_reviews WHERE review_id = $1`

	review := &domain.DocumentReview{}
	err := scanReview(r.pool.QueryRow(ctx, query, reviewID), review)
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

	query := `SELECT ` + reviewSelectColumns + `
	          FROM document_reviews WHERE user_document_id = $1
	          ORDER BY COALESCE(reviewed_at, updated_at) DESC`

	return r.queryReviews(ctx, query, userDocumentID)
}

func (r *documentReviewReadImpl) GetByVehicleDocumentID(ctx context.Context, vehicleDocumentID string) ([]*domain.DocumentReview, error) {
	r.logger.Debug("récupération des revues par vehicleDocumentID", zap.String("vehicleDocumentID", vehicleDocumentID))

	query := `SELECT ` + reviewSelectColumns + `
	          FROM document_reviews WHERE vehicle_document_id = $1
	          ORDER BY COALESCE(reviewed_at, updated_at) DESC`

	return r.queryReviews(ctx, query, vehicleDocumentID)
}

func (r *documentReviewReadImpl) GetByPersonaInquiryID(ctx context.Context, personaInquiryID string) (*domain.DocumentReview, error) {
	r.logger.Debug("récupération de la revue par persona_inquiry_id", zap.String("personaInquiryID", personaInquiryID))

	query := `SELECT ` + reviewSelectColumns + ` FROM document_reviews WHERE persona_inquiry_id = $1`

	review := &domain.DocumentReview{}
	err := scanReview(r.pool.QueryRow(ctx, query, personaInquiryID), review)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.logger.Debug("revue non trouvée par persona_inquiry_id", zap.String("personaInquiryID", personaInquiryID))
			return nil, fileErrors.ErrorReviewNotFound
		}
		r.logger.Error("erreur récupération revue par persona_inquiry_id", zap.String("personaInquiryID", personaInquiryID), zap.Error(err))
		return nil, fileErrors.ErrorDataRetrievalFailed
	}
	return review, nil
}

func (r *documentReviewReadImpl) GetByUserID(ctx context.Context, userID string) ([]*domain.DocumentReview, error) {
	r.logger.Debug("récupération des revues par userID", zap.String("userID", userID))

	query := `SELECT ` + reviewSelectColumns + `
	          FROM document_reviews
	          WHERE user_id = $1
	          ORDER BY updated_at DESC`

	return r.queryReviews(ctx, query, userID)
}

func (r *documentReviewReadImpl) List(ctx context.Context, userID string, status string, decision string, offset int32, limit int32) ([]*domain.DocumentReview, error) {
	r.logger.Debug("liste des revues avec filtres",
		zap.String("userID", userID),
		zap.String("status", status),
		zap.String("decision", decision),
		zap.Int32("offset", offset),
		zap.Int32("limit", limit),
	)

	query := `SELECT ` + reviewSelectColumns + ` FROM document_reviews WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if userID != "" {
		query += fmt.Sprintf(` AND user_id = $%d`, argIdx)
		args = append(args, userID)
		argIdx++
	}
	if status != "" {
		query += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, status)
		argIdx++
	}
	if decision != "" {
		query += fmt.Sprintf(` AND decision = $%d`, argIdx)
		args = append(args, decision)
		argIdx++
	}

	query += ` ORDER BY updated_at DESC`
	query += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, limit, offset)

	return r.queryReviewsArgs(ctx, query, args...)
}

func (r *documentReviewReadImpl) GetHistoryByUserIDAndLogicalType(ctx context.Context, userID string, logicalType string) ([]*domain.DocumentReview, error) {
	r.logger.Debug("historique des revues par type logique",
		zap.String("userID", userID),
		zap.String("logicalType", logicalType),
	)

	query := `SELECT ` + reviewSelectColumns + `
	          FROM document_reviews
	          WHERE user_id = $1 AND logical_document_type = $2
	          ORDER BY COALESCE(reviewed_at, updated_at) DESC`

	return r.queryReviewsArgs(ctx, query, userID, logicalType)
}

func (r *documentReviewReadImpl) queryReviews(ctx context.Context, query string, id string) ([]*domain.DocumentReview, error) {
	return r.queryReviewsArgs(ctx, query, id)
}

func (r *documentReviewReadImpl) queryReviewsArgs(ctx context.Context, query string, args ...interface{}) ([]*domain.DocumentReview, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		r.logger.Error("erreur requête reviews", zap.Error(err))
		return nil, fileErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	var reviews []*domain.DocumentReview
	for rows.Next() {
		review := &domain.DocumentReview{}
		if err := scanReview(rows, review); err != nil {
			r.logger.Error("erreur scan revue", zap.Error(err))
			return nil, fileErrors.ErrorDataRetrievalFailed
		}
		reviews = append(reviews, review)
	}
	return reviews, nil
}
