package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	filepb "github.com/Kpeewu/tissi-mah/services/file-service/proto/gen"
	"github.com/Kpeewu/tissi-mah/services/kyc-service/internal/domain"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// fileServiceClientImpl est le client gRPC vers file-service.
// Utilise TLS avec skip-verify pour la communication intra-cluster.
type fileServiceClientImpl struct {
	conn       *grpc.ClientConn
	grpcClient filepb.FileServiceClient
	logger     *zap.Logger
}

// NewFileServiceClient établit la connexion gRPC vers file-service.
// address doit être au format "host:port" (ex: "file-service:50053").
func NewFileServiceClient(address string, logger *zap.Logger) (FileServiceClient, error) {
	logger.Debug("connecting to file-service", zap.String("address", address))

	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(grpcutil.ClientTransportCredentials()),
	)
	if err != nil {
		logger.Error("failed to connect to file-service", zap.Error(err), zap.String("address", address))
		return nil, fmt.Errorf("file-service: failed to connect to %s: %w", address, err)
	}

	return &fileServiceClientImpl{
		conn:       conn,
		grpcClient: filepb.NewFileServiceClient(conn),
		logger:     logger,
	}, nil
}

func (c *fileServiceClientImpl) Close() error {
	return c.conn.Close()
}

func (c *fileServiceClientImpl) GetCurrentUserDocument(ctx context.Context, userID string, documentType string) (*domain.DocumentRef, error) {
	c.logger.Debug("client: GetCurrentUserDocument",
		zap.String("userID", userID),
		zap.String("documentType", documentType),
	)

	resp, err := c.grpcClient.GetCurrentUserDocument(ctx, &filepb.GetCurrentUserDocumentRequest{
		UserId:       userID,
		DocumentType: documentType,
	})
	if err != nil {
		c.logger.Error("client: GetCurrentUserDocument failed", zap.Error(err))
		return nil, fmt.Errorf("file-service: GetCurrentUserDocument: %w", err)
	}

	return &domain.DocumentRef{
		DocumentID:   resp.DocumentId,
		DocumentType: resp.DocumentType,
	}, nil
}

func (c *fileServiceClientImpl) GetUserDocument(ctx context.Context, documentID string) (*domain.DocumentRef, error) {
	c.logger.Debug("client: GetUserDocument", zap.String("documentID", documentID))

	resp, err := c.grpcClient.GetUserDocument(ctx, &filepb.GetDocumentByIDRequest{
		DocumentId:     documentID,
		PresignTTLSecs: 86400, // 24h — Persona doit pouvoir fetcher le document
	})
	if err != nil {
		c.logger.Error("client: GetUserDocument failed", zap.Error(err))
		return nil, fmt.Errorf("file-service: GetUserDocument: %w", err)
	}

	return &domain.DocumentRef{
		DocumentID:   resp.DocumentId,
		DocumentType: resp.DocumentType,
		OwnerID:      resp.UserId,
		DocumentURL:  resp.DocumentUrl,
	}, nil
}

func (c *fileServiceClientImpl) GetVehicleDocument(ctx context.Context, documentID string) (*domain.DocumentRef, error) {
	c.logger.Debug("client: GetVehicleDocument", zap.String("documentID", documentID))

	resp, err := c.grpcClient.GetVehicleDocument(ctx, &filepb.GetDocumentByIDRequest{
		DocumentId:     documentID,
		PresignTTLSecs: 86400, // 24h — Persona doit pouvoir fetcher le document
	})
	if err != nil {
		c.logger.Error("client: GetVehicleDocument failed", zap.Error(err))
		return nil, fmt.Errorf("file-service: GetVehicleDocument: %w", err)
	}

	return &domain.DocumentRef{
		DocumentID:   resp.DocumentId,
		DocumentType: resp.DocumentType,
		OwnerID:      resp.VehicleId,
		DocumentURL:  resp.DocumentUrl,
		UserID:       resp.UserId,
	}, nil
}

func (c *fileServiceClientImpl) GetUserDocuments(ctx context.Context, userID string) ([]*domain.DocumentRef, error) {
	c.logger.Debug("client: GetUserDocuments", zap.String("userID", userID))

	resp, err := c.grpcClient.GetUserDocuments(ctx, &filepb.GetUserDocumentsRequest{
		UserId: userID,
	})
	if err != nil {
		c.logger.Error("client: GetUserDocuments failed", zap.Error(err))
		return nil, fmt.Errorf("file-service: GetUserDocuments: %w", err)
	}

	refs := make([]*domain.DocumentRef, 0, len(resp.Documents))
	for _, d := range resp.Documents {
		refs = append(refs, &domain.DocumentRef{
			DocumentID:   d.DocumentId,
			DocumentType: d.DocumentType,
		})
	}
	return refs, nil
}

func (c *fileServiceClientImpl) GetVehicleDocuments(ctx context.Context, vehicleID string) ([]*domain.DocumentRef, error) {
	c.logger.Debug("client: GetVehicleDocuments", zap.String("vehicleID", vehicleID))

	resp, err := c.grpcClient.GetVehicleDocuments(ctx, &filepb.GetVehicleDocumentsRequest{
		VehicleId: vehicleID,
	})
	if err != nil {
		c.logger.Error("client: GetVehicleDocuments failed", zap.Error(err))
		return nil, fmt.Errorf("file-service: GetVehicleDocuments: %w", err)
	}

	refs := make([]*domain.DocumentRef, 0, len(resp.Documents))
	for _, d := range resp.Documents {
		refs = append(refs, &domain.DocumentRef{
			DocumentID:   d.DocumentId,
			DocumentType: d.DocumentType,
		})
	}
	return refs, nil
}

func (c *fileServiceClientImpl) ListKycDocuments(ctx context.Context, statuses []string) ([]*domain.KycDocument, error) {
	c.logger.Debug("client: ListKycDocuments", zap.Strings("statuses", statuses))

	resp, err := c.grpcClient.ListKycDocuments(ctx, &filepb.ListKycDocumentsRequest{Statuses: statuses})
	if err != nil {
		c.logger.Error("client: ListKycDocuments failed", zap.Error(err))
		return nil, fmt.Errorf("file-service: ListKycDocuments: %w", err)
	}

	docs := make([]*domain.KycDocument, 0, len(resp.Documents))
	for _, d := range resp.Documents {
		docs = append(docs, &domain.KycDocument{
			DocumentID:   d.DocumentId,
			UserID:       d.UserId,
			VehicleID:    d.VehicleId,
			DocumentType: d.DocumentType,
			Status:       d.Status,
			OwnerKind:    d.OwnerKind,
			UpdatedAt:    d.UpdatedAt,
		})
	}
	return docs, nil
}

func (c *fileServiceClientImpl) GetUserDocumentSummaries(ctx context.Context, userID string) ([]*domain.DocumentSummary, error) {
	c.logger.Debug("client: GetUserDocumentSummaries", zap.String("userID", userID))

	resp, err := c.grpcClient.GetUserDocuments(ctx, &filepb.GetUserDocumentsRequest{UserId: userID})
	if err != nil {
		c.logger.Error("client: GetUserDocumentSummaries failed", zap.Error(err))
		return nil, fmt.Errorf("file-service: GetUserDocuments: %w", err)
	}

	out := make([]*domain.DocumentSummary, 0, len(resp.Documents))
	for _, d := range resp.Documents {
		if !d.IsCurrent {
			continue
		}
		out = append(out, &domain.DocumentSummary{
			DocumentID:          d.DocumentId,
			DocumentType:        d.DocumentType,
			LogicalDocumentType: domain.ToLogicalDocumentType(d.DocumentType),
			Status:              d.Status,
			OwnerKind:           "user",
			OwnerID:             d.UserId,
			Category:            domain.DocumentCategory(d.DocumentType, "user"),
			DocumentURL:         d.DocumentUrl,
			FileSizeBytes:       d.FileSizeBytes,
			MimeType:            d.MimeType,
			DocumentNumber:      d.DocumentNumber,
			IsCurrent:           d.IsCurrent,
			UploadedAt:          d.UploadedAt,
			UpdatedAt:           d.UpdatedAt,
			IssuedAt:            d.IssuedAt,
			ExpiredAt:           d.ExpiredAt,
			IssuingCountry:      d.IssuingCountry,
		})
	}
	return out, nil
}

func (c *fileServiceClientImpl) GetVehicleDocumentSummariesByUserID(ctx context.Context, userID string) ([]*domain.DocumentSummary, error) {
	c.logger.Debug("client: GetVehicleDocumentSummariesByUserID", zap.String("userID", userID))

	resp, err := c.grpcClient.GetVehicleDocumentsByUserID(ctx, &filepb.GetVehicleDocumentsByUserIDRequest{UserId: userID})
	if err != nil {
		c.logger.Error("client: GetVehicleDocumentSummariesByUserID failed", zap.Error(err))
		return nil, fmt.Errorf("file-service: GetVehicleDocumentsByUserID: %w", err)
	}

	out := make([]*domain.DocumentSummary, 0, len(resp.Documents))
	for _, d := range resp.Documents {
		if !d.IsCurrent {
			continue
		}
		var vd *domain.VehicleDetails
		if d.Vehicle != nil {
			vd = &domain.VehicleDetails{
				VehicleID:     d.Vehicle.VehicleId,
				Brand:         d.Vehicle.Brand,
				BrandModel:    d.Vehicle.BrandModel,
				Color:         d.Vehicle.Color,
				LicencePlate:  d.Vehicle.LicencePlate,
				NumberOfSeats: d.Vehicle.NumberOfSeats,
				IsVerified:    d.Vehicle.IsVerified,
			}
		}
		out = append(out, &domain.DocumentSummary{
			DocumentID:          d.DocumentId,
			DocumentType:        d.DocumentType,
			LogicalDocumentType: domain.ToLogicalDocumentType(d.DocumentType),
			Status:              d.Status,
			OwnerKind:           "vehicle",
			OwnerID:             d.VehicleId,
			Category:            domain.DocumentCategory(d.DocumentType, "vehicle"),
			DocumentURL:         d.DocumentUrl,
			FileSizeBytes:       d.FileSizeBytes,
			MimeType:            d.MimeType,
			DocumentNumber:      d.DocumentNumber,
			IsCurrent:           d.IsCurrent,
			UploadedAt:          d.UploadedAt,
			UpdatedAt:           d.UpdatedAt,
			IssuedAt:            d.IssuedAt,
			ExpiredAt:           d.ExpireAt,
			IssuingAuthority:    d.IssuingAuthority,
			Vehicle:             vd,
		})
	}
	return out, nil
}

func (c *fileServiceClientImpl) CreateDocumentReview(ctx context.Context, review *domain.Review) (*domain.Review, error) {
	c.logger.Debug("client: CreateDocumentReview")

	req := &filepb.CreateDocumentReviewRequest{
		UserId:               review.UserID,
		DocumentType:         review.DocumentType,
		UserDocumentId:       review.UserDocumentID,
		SecondUserDocumentId: review.SecondUserDocumentID,
		VehicleDocumentId:    review.VehicleDocumentID,
		PersonaInquiryId:    review.PersonaInquiryID,
		PersonaTemplateId:   review.PersonaTemplateID,
		PersonaSessionToken: review.PersonaSessionToken,
		AttemptNumber:       review.AttemptNumber,
		PreviousReviewId:    review.PreviousReviewID,
		Status:              review.Status,
		Decision:            review.Decision,
		ReasonRejection:     review.ReasonRejection,
		RejectionDetails:    review.RejectionDetails,
		ReviewedBy:          review.ReviewedBy,
		ReviewType:          review.ReviewType,
		Notes:               review.Notes,
		ExtractedData:       review.ExtractedData,
	}
	if review.SessionExpiresAt != nil {
		req.SessionExpiresAt = review.SessionExpiresAt.Format(time.RFC3339)
	}
	if review.WebhookReceivedAt != nil {
		req.WebhookReceivedAt = review.WebhookReceivedAt.Format(time.RFC3339)
	}
	if review.SubmittedAt != nil {
		req.SubmittedAt = review.SubmittedAt.Format(time.RFC3339)
	}
	if len(review.PersonaRawPayload) > 0 {
		req.PersonaRawPayload = review.PersonaRawPayload
	}
	req.WebhookEventType = review.WebhookEventType

	resp, err := c.grpcClient.CreateDocumentReview(ctx, req)
	if err != nil {
		c.logger.Error("client: CreateDocumentReview failed", zap.Error(err))
		return nil, fmt.Errorf("file-service: CreateDocumentReview: %w", err)
	}

	return protoToReview(resp), nil
}

func (c *fileServiceClientImpl) GetDocumentReview(ctx context.Context, reviewID string) (*domain.Review, error) {
	c.logger.Debug("client: GetDocumentReview", zap.String("reviewID", reviewID))

	resp, err := c.grpcClient.GetDocumentReview(ctx, &filepb.GetDocumentReviewByIDRequest{
		ReviewId: reviewID,
	})
	if err != nil {
		c.logger.Error("client: GetDocumentReview failed", zap.Error(err))
		return nil, fmt.Errorf("file-service: GetDocumentReview: %w", err)
	}

	return protoToReview(resp), nil
}

func (c *fileServiceClientImpl) GetDocumentReviewByPersonaInquiryID(ctx context.Context, personaInquiryID string) (*domain.Review, error) {
	c.logger.Debug("client: GetDocumentReviewByPersonaInquiryID", zap.String("personaInquiryID", personaInquiryID))

	resp, err := c.grpcClient.GetDocumentReviewByPersonaInquiryID(ctx, &filepb.GetDocumentReviewByPersonaInquiryIDRequest{
		PersonaInquiryId: personaInquiryID,
	})
	if err != nil {
		c.logger.Error("client: GetDocumentReviewByPersonaInquiryID failed", zap.Error(err))
		return nil, fmt.Errorf("file-service: GetDocumentReviewByPersonaInquiryID: %w", err)
	}

	return protoToReview(resp), nil
}

func (c *fileServiceClientImpl) GetDocumentReviewsByUserID(ctx context.Context, userID string) ([]*domain.Review, error) {
	c.logger.Debug("client: GetDocumentReviewsByUserID", zap.String("userID", userID))

	resp, err := c.grpcClient.GetDocumentReviewsByUserID(ctx, &filepb.GetDocumentReviewsByUserIDRequest{
		UserId: userID,
	})
	if err != nil {
		c.logger.Error("client: GetDocumentReviewsByUserID failed", zap.Error(err))
		return nil, fmt.Errorf("file-service: GetDocumentReviewsByUserID: %w", err)
	}

	reviews := make([]*domain.Review, 0, len(resp.Reviews))
	for _, r := range resp.Reviews {
		reviews = append(reviews, protoToReview(r))
	}
	return reviews, nil
}

func (c *fileServiceClientImpl) UpdateDocumentReview(ctx context.Context, review *domain.Review) (*domain.Review, error) {
	c.logger.Debug("client: UpdateDocumentReview", zap.String("reviewID", review.ReviewID))

	req := &filepb.UpdateDocumentReviewRequest{
		ReviewId:            review.ReviewID,
		PersonaSessionToken: review.PersonaSessionToken,
		WebhookEventType:    review.WebhookEventType,
		Status:              review.Status,
		Decision:            review.Decision,
		ReasonRejection:     review.ReasonRejection,
		RejectionDetails:    review.RejectionDetails,
		ReviewedBy:          review.ReviewedBy,
		ReviewType:          review.ReviewType,
		Notes:               review.Notes,
	}
	if review.SessionExpiresAt != nil {
		req.SessionExpiresAt = review.SessionExpiresAt.Format(time.RFC3339)
	}
	if review.WebhookReceivedAt != nil {
		req.WebhookReceivedAt = review.WebhookReceivedAt.Format(time.RFC3339)
	}
	if len(review.PersonaRawPayload) > 0 {
		req.PersonaRawPayload = review.PersonaRawPayload
	}
	if review.SubmittedAt != nil {
		req.SubmittedAt = review.SubmittedAt.Format(time.RFC3339)
	}

	resp, err := c.grpcClient.UpdateDocumentReview(ctx, req)
	if err != nil {
		c.logger.Error("client: UpdateDocumentReview failed", zap.Error(err))
		return nil, fmt.Errorf("file-service: UpdateDocumentReview: %w", err)
	}

	return protoToReview(resp), nil
}

func (c *fileServiceClientImpl) ListDocumentReviews(ctx context.Context, userID string, status string, decision string, page int32, pageSize int32) ([]*domain.Review, error) {
	c.logger.Debug("client: ListDocumentReviews",
		zap.String("userID", userID),
		zap.String("status", status),
		zap.String("decision", decision),
		zap.Int32("page", page),
		zap.Int32("pageSize", pageSize),
	)

	resp, err := c.grpcClient.ListDocumentReviews(ctx, &filepb.ListDocumentReviewsRequest{
		UserId:   userID,
		Status:   status,
		Decision: decision,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		c.logger.Error("client: ListDocumentReviews failed", zap.Error(err))
		return nil, fmt.Errorf("file-service: ListDocumentReviews: %w", err)
	}

	reviews := make([]*domain.Review, 0, len(resp.Reviews))
	for _, r := range resp.Reviews {
		reviews = append(reviews, protoToReview(r))
	}
	return reviews, nil
}

func (c *fileServiceClientImpl) GetDocumentReviewHistory(ctx context.Context, userID string, logicalDocumentType string) ([]*domain.Review, error) {
	c.logger.Debug("client: GetDocumentReviewHistory",
		zap.String("userID", userID),
		zap.String("logicalDocumentType", logicalDocumentType),
	)

	resp, err := c.grpcClient.GetDocumentReviewHistory(ctx, &filepb.GetDocumentReviewHistoryRequest{
		UserId:              userID,
		LogicalDocumentType: logicalDocumentType,
	})
	if err != nil {
		c.logger.Error("client: GetDocumentReviewHistory failed", zap.Error(err))
		return nil, fmt.Errorf("file-service: GetDocumentReviewHistory: %w", err)
	}

	reviews := make([]*domain.Review, 0, len(resp.Reviews))
	for _, r := range resp.Reviews {
		reviews = append(reviews, protoToReview(r))
	}
	return reviews, nil
}

// protoToReview convertit un DocumentReviewResponse proto en domain.Review
func protoToReview(r *filepb.DocumentReviewResponse) *domain.Review {
	review := &domain.Review{
		ReviewID:             r.ReviewId,
		UserID:               r.UserId,
		DocumentType:         r.DocumentType,
		LogicalDocumentType:  r.LogicalDocumentType,
		UserDocumentID:       r.UserDocumentId,
		SecondUserDocumentID: r.SecondUserDocumentId,
		VehicleDocumentID:    r.VehicleDocumentId,

		PersonaInquiryID:    r.PersonaInquiryId,
		PersonaTemplateID:   r.PersonaTemplateId,
		PersonaSessionToken: r.PersonaSessionToken,

		WebhookEventType: r.WebhookEventType,

		AttemptNumber:    r.AttemptNumber,
		PreviousReviewID: r.PreviousReviewId,

		Status:           r.Status,
		Decision:         r.Decision,
		ReasonRejection:  r.ReasonRejection,
		RejectionDetails: r.RejectionDetails,

		ReviewedBy: r.ReviewedBy,
		ReviewType: r.ReviewType,

		Notes: r.Notes,
	}

	if len(r.ExtractedData) > 0 {
		review.ExtractedData = json.RawMessage(r.ExtractedData)
	}
	if len(r.PersonaRawPayload) > 0 {
		review.PersonaRawPayload = json.RawMessage(r.PersonaRawPayload)
	}

	if r.SessionExpiresAt != "" {
		if t, err := time.Parse(time.RFC3339, r.SessionExpiresAt); err == nil {
			review.SessionExpiresAt = &t
		}
	}
	if r.WebhookReceivedAt != "" {
		if t, err := time.Parse(time.RFC3339, r.WebhookReceivedAt); err == nil {
			review.WebhookReceivedAt = &t
		}
	}
	if r.ReviewedAt != "" {
		if t, err := time.Parse(time.RFC3339, r.ReviewedAt); err == nil {
			review.ReviewedAt = &t
		}
	}
	if r.SubmittedAt != "" {
		if t, err := time.Parse(time.RFC3339, r.SubmittedAt); err == nil {
			review.SubmittedAt = &t
		}
	}
	if r.UpdatedAt != "" {
		if t, err := time.Parse(time.RFC3339, r.UpdatedAt); err == nil {
			review.UpdatedAt = t
		}
	}

	return review
}
