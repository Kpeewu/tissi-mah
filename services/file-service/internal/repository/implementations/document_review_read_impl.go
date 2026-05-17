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

const reviewSelectColumns = `review_id, user_id, document_type, user_document_id, vehicle_document_id,
	persona_inquiry_id, persona_template_id, persona_session_token, session_expires_at,
	webhook_event_type, webhook_received_at, persona_raw_payload,
	attempt_number, previous_review_id,
	status, decision, reason_rejection, rejection_details,
	reviewed_by, review_type, reviewed_at,
	notes, extracted_data,
	submitted_at, updated_at`

func (r *documentReviewReadImpl) GetByID(ctx context.Context, reviewID string) (*domain.DocumentReview, error) {
	r.logger.Debug("récupération de la revue par ID", zap.String("reviewID", reviewID))

	query := `SELECT ` + reviewSelectColumns + `
	          FROM document_reviews WHERE review_id = $1`

	review := &domain.DocumentReview{}
	err := r.pool.QueryRow(ctx, query, reviewID).Scan(
		&review.ReviewID, &review.UserDocumentID, &review.VehicleDocumentID,
		&review.PersonaInquiryID, &review.PersonaTemplateID, &review.PersonaSessionToken, &review.SessionExpiresAt,
		&review.WebhookEventType, &review.WebhookReceivedAt, &review.PersonaRawPayload,
		&review.AttemptNumber, &review.PreviousReviewID,
		&review.Status, &review.Decision, &review.ReasonRejection, &review.RejectionDetails,
		&review.ReviewedBy, &review.ReviewType, &review.ReviewedAt,
		&review.Notes, &review.ExtractedData,
		&review.SubmittedAt, &review.UpdatedAt,
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

	query := `SELECT ` + reviewSelectColumns + `
	          FROM document_reviews WHERE user_document_id = $1
	          ORDER BY reviewed_at DESC`

	return r.queryReviews(ctx, query, userDocumentID)
}

func (r *documentReviewReadImpl) GetByVehicleDocumentID(ctx context.Context, vehicleDocumentID string) ([]*domain.DocumentReview, error) {
	r.logger.Debug("récupération des revues par vehicleDocumentID", zap.String("vehicleDocumentID", vehicleDocumentID))

	query := `SELECT ` + reviewSelectColumns + `
	          FROM document_reviews WHERE vehicle_document_id = $1
	          ORDER BY reviewed_at DESC`

	return r.queryReviews(ctx, query, vehicleDocumentID)
}

func (r *documentReviewReadImpl) GetByPersonaInquiryID(ctx context.Context, personaInquiryID string) (*domain.DocumentReview, error) {
	r.logger.Debug("récupération de la revue par persona_inquiry_id", zap.String("personaInquiryID", personaInquiryID))

	query := `SELECT ` + reviewSelectColumns + `
	          FROM document_reviews WHERE persona_inquiry_id = $1`

	review := &domain.DocumentReview{}
	err := r.pool.QueryRow(ctx, query, personaInquiryID).Scan(
		&review.ReviewID, &review.UserDocumentID, &review.VehicleDocumentID,
		&review.PersonaInquiryID, &review.PersonaTemplateID, &review.PersonaSessionToken, &review.SessionExpiresAt,
		&review.WebhookEventType, &review.WebhookReceivedAt, &review.PersonaRawPayload,
		&review.AttemptNumber, &review.PreviousReviewID,
		&review.Status, &review.Decision, &review.ReasonRejection, &review.RejectionDetails,
		&review.ReviewedBy, &review.ReviewType, &review.ReviewedAt,
		&review.Notes, &review.ExtractedData,
		&review.SubmittedAt, &review.UpdatedAt,
	)
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

	// user_id est dénormalisé directement sur document_reviews depuis la
	// migration 000008 : plus besoin de joindre via les FK doc (qui sont NULL
	// pour les reviews Persona 100%).
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

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		r.logger.Error("erreur requête List", zap.Error(err))
		return nil, fileErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	var reviews []*domain.DocumentReview
	for rows.Next() {
		review := &domain.DocumentReview{}
		err := rows.Scan(
			&review.ReviewID, &review.UserID, &review.DocumentType, &review.UserDocumentID, &review.VehicleDocumentID,
			&review.PersonaInquiryID, &review.PersonaTemplateID, &review.PersonaSessionToken, &review.SessionExpiresAt,
			&review.WebhookEventType, &review.WebhookReceivedAt, &review.PersonaRawPayload,
			&review.AttemptNumber, &review.PreviousReviewID,
			&review.Status, &review.Decision, &review.ReasonRejection, &review.RejectionDetails,
			&review.ReviewedBy, &review.ReviewType, &review.ReviewedAt,
			&review.Notes, &review.ExtractedData,
			&review.SubmittedAt, &review.UpdatedAt,
		)
		if err != nil {
			r.logger.Error("erreur scan revue dans List", zap.Error(err))
			return nil, fileErrors.ErrorDataRetrievalFailed
		}
		reviews = append(reviews, review)
	}
	return reviews, nil
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
			&review.ReviewID, &review.UserID, &review.DocumentType, &review.UserDocumentID, &review.VehicleDocumentID,
			&review.PersonaInquiryID, &review.PersonaTemplateID, &review.PersonaSessionToken, &review.SessionExpiresAt,
			&review.WebhookEventType, &review.WebhookReceivedAt, &review.PersonaRawPayload,
			&review.AttemptNumber, &review.PreviousReviewID,
			&review.Status, &review.Decision, &review.ReasonRejection, &review.RejectionDetails,
			&review.ReviewedBy, &review.ReviewType, &review.ReviewedAt,
			&review.Notes, &review.ExtractedData,
			&review.SubmittedAt, &review.UpdatedAt,
		)
		if err != nil {
			r.logger.Error("erreur scan revue", zap.String("id", id), zap.Error(err))
			return nil, fileErrors.ErrorDataRetrievalFailed
		}
		reviews = append(reviews, review)
	}
	return reviews, nil
}
