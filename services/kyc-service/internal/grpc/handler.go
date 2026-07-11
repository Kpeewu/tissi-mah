package grpc

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Kpeewu/tissi-mah/services/kyc-service/internal/middleware"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/kyc-service/internal/service/interfaces"
	kycErrors "github.com/Kpeewu/tissi-mah/services/kyc-service/pkg/errors"
	kycpb "github.com/Kpeewu/tissi-mah/services/kyc-service/proto/gen"
)

const serviceVersion = "1.0.0"

// KYCHandler implémente kycpb.KYCServiceServer.
type KYCHandler struct {
	kycpb.UnimplementedKYCServiceServer
	service serviceInterfaces.KYCService
	logger  *zap.Logger
}

func NewKYCHandler(service serviceInterfaces.KYCService, logger *zap.Logger) *KYCHandler {
	return &KYCHandler{service: service, logger: logger}
}

// =============================================================================
// Helpers
// =============================================================================

// getUserID extrait le Firebase UID depuis le contexte gRPC.
func getUserID(ctx context.Context) (string, error) {
	uid, ok := ctx.Value(middleware.FirebaseIDKey).(string)
	if !ok || uid == "" {
		return "", status.Error(codes.Unauthenticated, "missing firebase ID in context")
	}
	return uid, nil
}

// getSupportID extrait l'UID de l'agent support depuis le contexte gRPC.
func getSupportID(ctx context.Context) (string, error) {
	uid, ok := ctx.Value(middleware.SupportIDKey).(string)
	if !ok || uid == "" {
		return "", status.Error(codes.Unauthenticated, "missing support ID in context")
	}
	return uid, nil
}

// =============================================================================
// CreateInquiry
// =============================================================================

func (h *KYCHandler) CreateInquiry(ctx context.Context, req *kycpb.CreateInquiryRequest) (*kycpb.CreateInquiryResponse, error) {
	userID, err := getUserID(ctx)
	if err != nil {
		return nil, err
	}

	if req.VehicleId != "" {
		return nil, status.Error(codes.InvalidArgument, "vehicle documents must be validated manually via ValidateDocument")
	}

	h.logger.Debug("handler: CreateInquiry called",
		zap.String("userID", userID),
		zap.String("documentType", req.DocumentType),
		zap.String("documentID", req.DocumentId),
	)

	result, err := h.service.CreateInquiry(ctx, serviceInterfaces.CreateInquiryInput{
		UserID:       userID,
		DocumentID:   req.DocumentId,
		DocumentType: req.DocumentType,
		VehicleID:    req.VehicleId,
	})
	if err != nil {
		h.logger.Error("handler: CreateInquiry failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	h.logger.Info("handler: CreateInquiry success", zap.String("reviewID", result.ReviewID))
	return &kycpb.CreateInquiryResponse{
		ReviewId:          result.ReviewID,
		PersonaInquiryId:  result.PersonaInquiryID,
		PersonaTemplateId: result.PersonaTemplateID,
		SessionToken:      result.SessionToken,
		SessionExpiresAt:  result.SessionExpiresAt,
		Status:            result.Status,
		AttemptNumber:     result.AttemptNumber,
		CreatedAt:         result.CreatedAt,
	}, nil
}

// =============================================================================
// GetInquiry
// =============================================================================

func (h *KYCHandler) GetInquiry(ctx context.Context, req *kycpb.GetInquiryRequest) (*kycpb.GetInquiryResponse, error) {
	userID, err := getUserID(ctx)
	if err != nil {
		return nil, err
	}

	h.logger.Debug("handler: GetInquiry called",
		zap.String("userID", userID),
		zap.String("personaInquiryID", req.PersonaInquiryId),
	)

	detail, err := h.service.GetInquiry(ctx, userID, req.PersonaInquiryId)
	if err != nil {
		h.logger.Error("handler: GetInquiry failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	return &kycpb.GetInquiryResponse{
		Inquiry: toProtoInquiryDetail(detail),
	}, nil
}

// =============================================================================
// GetKYCStatus
// =============================================================================

func (h *KYCHandler) GetKYCStatus(ctx context.Context, _ *kycpb.GetKYCStatusRequest) (*kycpb.GetKYCStatusResponse, error) {
	userID, err := getUserID(ctx)
	if err != nil {
		return nil, err
	}

	h.logger.Debug("handler: GetKYCStatus called", zap.String("userID", userID))

	kycStatus, err := h.service.GetKYCStatus(ctx, userID)
	if err != nil {
		h.logger.Error("handler: GetKYCStatus failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	resp := &kycpb.GetKYCStatusResponse{
		IdentityVerified: kycStatus.IdentityVerified,
		DriverVerified:   kycStatus.DriverVerified,
	}

	// Pending reviews
	for _, pr := range kycStatus.PendingReviews {
		item := &kycpb.PendingReviewItem{
			ReviewId:         pr.ReviewID,
			PersonaInquiryId: pr.PersonaInquiryID,
			Status:           pr.Status,
			AttemptNumber:    pr.AttemptNumber,
			DocumentType:     pr.DocumentType,
		}
		if pr.SessionExpiresAt != nil {
			item.SessionExpiresAt = pr.SessionExpiresAt.Format(time.RFC3339)
		}
		resp.PendingReviews = append(resp.PendingReviews, item)
	}

	// Latest rejection
	if kycStatus.LatestRejection != nil {
		lr := kycStatus.LatestRejection
		resp.LatestRejection = &kycpb.LatestRejectionItem{
			ReviewId:         lr.ReviewID,
			ReasonRejection:  lr.ReasonRejection,
			RejectionDetails: lr.RejectionDetails,
			ReviewType:       lr.ReviewType,
		}
		if lr.ReviewedAt != nil {
			resp.LatestRejection.ReviewedAt = lr.ReviewedAt.Format(time.RFC3339)
		}
	}

	return resp, nil
}

// =============================================================================
// ResumeInquiry
// =============================================================================

func (h *KYCHandler) ResumeInquiry(ctx context.Context, req *kycpb.ResumeInquiryRequest) (*kycpb.ResumeInquiryResponse, error) {
	userID, err := getUserID(ctx)
	if err != nil {
		return nil, err
	}

	h.logger.Debug("handler: ResumeInquiry called",
		zap.String("userID", userID),
		zap.String("personaInquiryID", req.PersonaInquiryId),
	)

	result, err := h.service.ResumeInquiry(ctx, userID, req.PersonaInquiryId)
	if err != nil {
		h.logger.Error("handler: ResumeInquiry failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	return &kycpb.ResumeInquiryResponse{
		ReviewId:         result.ReviewID,
		PersonaInquiryId: result.PersonaInquiryID,
		SessionToken:     result.SessionToken,
		SessionExpiresAt: result.SessionExpiresAt,
		Status:           result.Status,
		AttemptNumber:    result.AttemptNumber,
	}, nil
}

// =============================================================================
// ProcessWebhook
// =============================================================================

func (h *KYCHandler) ProcessWebhook(ctx context.Context, req *kycpb.ProcessWebhookRequest) (*kycpb.ProcessWebhookResponse, error) {
	h.logger.Debug("handler: ProcessWebhook called",
		zap.String("eventType", req.WebhookEventType),
		zap.String("personaInquiryID", req.PersonaInquiryId),
	)

	err := h.service.ProcessWebhook(ctx, serviceInterfaces.WebhookInput{
		Signature:         req.Signature,
		PersonaInquiryID:  req.PersonaInquiryId,
		WebhookEventType:  req.WebhookEventType,
		OccurredAt:        req.OccurredAt,
		PersonaRawPayload: req.PersonaRawPayload,
	})
	if err != nil {
		h.logger.Error("handler: ProcessWebhook failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	return &kycpb.ProcessWebhookResponse{Success: true}, nil
}

// =============================================================================
// GetAdminReviews
// =============================================================================

func (h *KYCHandler) GetAdminReviews(ctx context.Context, req *kycpb.GetAdminReviewsRequest) (*kycpb.GetAdminReviewsResponse, error) {
	_, err := getSupportID(ctx)
	if err != nil {
		return nil, err
	}

	h.logger.Debug("handler: GetAdminReviews called",
		zap.String("filterUserID", req.UserId),
		zap.String("status", req.Status),
		zap.String("decision", req.Decision),
		zap.Int32("index", req.Index),
	)

	items, err := h.service.GetAdminReviews(ctx, serviceInterfaces.GetAdminReviewsInput{
		UserID:   req.UserId,
		Status:   req.Status,
		Decision: req.Decision,
		Index:    req.Index,
	})
	if err != nil {
		h.logger.Error("handler: GetAdminReviews failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	protoItems := make([]*kycpb.AdminReviewItem, 0, len(items))
	for _, item := range items {
		protoItems = append(protoItems, toProtoAdminReviewItem(item))
	}

	return &kycpb.GetAdminReviewsResponse{Reviews: protoItems}, nil
}

// =============================================================================
// GetAdminReview
// =============================================================================

func (h *KYCHandler) GetAdminReview(ctx context.Context, req *kycpb.GetAdminReviewRequest) (*kycpb.GetAdminReviewResponse, error) {
	supportID, err := getSupportID(ctx)
	if err != nil {
		return nil, err
	}

	h.logger.Debug("handler: GetAdminReview called",
		zap.String("reviewID", req.ReviewId),
	)

	detail, err := h.service.GetAdminReview(ctx, supportID, req.ReviewId)
	if err != nil {
		h.logger.Error("handler: GetAdminReview failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	return &kycpb.GetAdminReviewResponse{
		Review: toProtoAdminReviewDetail(detail),
	}, nil
}

// =============================================================================
// OverrideReview
// =============================================================================

func (h *KYCHandler) OverrideReview(ctx context.Context, req *kycpb.OverrideReviewRequest) (*kycpb.OverrideReviewResponse, error) {
	supportID, err := getSupportID(ctx)
	if err != nil {
		return nil, err
	}

	h.logger.Debug("handler: OverrideReview called",
		zap.String("reviewID", req.ReviewId),
		zap.String("decision", req.Decision),
	)

	result, err := h.service.OverrideReview(ctx, serviceInterfaces.OverrideReviewInput{
		UserID:           supportID,
		ReviewID:         req.ReviewId,
		Decision:         req.Decision,
		ReasonRejection:  req.ReasonRejection,
		RejectionDetails: req.RejectionDetails,
		Notes:            req.Notes,
	})
	if err != nil {
		h.logger.Error("handler: OverrideReview failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	return &kycpb.OverrideReviewResponse{
		ReviewId:         result.ReviewID,
		PersonaInquiryId: result.PersonaInquiryID,
		Decision:         result.Decision,
		ReasonRejection:  result.ReasonRejection,
		RejectionDetails: result.RejectionDetails,
		ReviewedBy:       result.ReviewedBy,
		ReviewType:       result.ReviewType,
		ReviewedAt:       result.ReviewedAt,
		Notes:            result.Notes,
		UpdatedAt:        result.UpdatedAt,
	}, nil
}

// =============================================================================
// ValidateDocument
// =============================================================================

func (h *KYCHandler) ValidateDocument(ctx context.Context, req *kycpb.ValidateDocumentRequest) (*kycpb.ValidateDocumentResponse, error) {
	supportID, err := getSupportID(ctx)
	if err != nil {
		return nil, err
	}

	h.logger.Debug("handler: ValidateDocument called",
		zap.String("supportID", supportID),
		zap.String("documentID", req.DocumentId),
		zap.String("vehicleID", req.VehicleId),
		zap.String("decision", req.Decision),
	)

	result, err := h.service.ValidateDocument(ctx, serviceInterfaces.ValidateDocumentInput{
		SupportAgentID:   supportID,
		DocumentID:       req.DocumentId,
		VehicleID:        req.VehicleId,
		Decision:         req.Decision,
		ReasonRejection:  req.ReasonRejection,
		RejectionDetails: req.RejectionDetails,
		Notes:            req.Notes,
	})
	if err != nil {
		h.logger.Error("handler: ValidateDocument failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	h.logger.Info("handler: ValidateDocument success",
		zap.String("reviewID", result.ReviewID),
		zap.String("decision", result.Decision),
	)
	return &kycpb.ValidateDocumentResponse{
		ReviewId:   result.ReviewID,
		Decision:   result.Decision,
		ReviewedBy: result.ReviewedBy,
		ReviewType: result.ReviewType,
		ReviewedAt: result.ReviewedAt,
		Notes:      result.Notes,
	}, nil
}

// =============================================================================
// GetManualReviewRequests / GetManualReviewRequestDetail (support)
// =============================================================================

func (h *KYCHandler) GetManualReviewRequests(ctx context.Context, req *kycpb.GetManualReviewRequestsRequest) (*kycpb.GetManualReviewRequestsResponse, error) {
	if _, err := getSupportID(ctx); err != nil {
		return nil, err
	}
	h.logger.Debug("handler: GetManualReviewRequests called",
		zap.String("status", req.Status), zap.Int32("page", req.Page), zap.Int32("pageSize", req.PageSize))

	result, err := h.service.GetManualReviewRequests(ctx, serviceInterfaces.GetManualReviewRequestsInput{
		Status:      req.Status,
		Page:        req.Page,
		PageSize:    req.PageSize,
		Name:        req.Name,
		FirstName:   req.FirstName,
		DepositFrom: req.DepositFrom,
		DepositTo:   req.DepositTo,
	})
	if err != nil {
		h.logger.Error("handler: GetManualReviewRequests failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	items := make([]*kycpb.ManualReviewRequestItem, 0, len(result.Requests))
	for _, r := range result.Requests {
		items = append(items, &kycpb.ManualReviewRequestItem{
			UserId:          r.User.UserID,
			Name:            r.User.Name,
			FirstName:       r.User.FirstName,
			Email:           r.User.Email,
			PhoneNumber:     r.User.PhoneNumber,
			ProfileImageURL: r.User.ProfileImageURL,
			PassengerStatus: r.PassengerStatus,
			DriverStatus:    r.DriverStatus,
			TotalDocuments:  r.TotalDocuments,
			LastDepositAt:   r.LastDepositAt,
		})
	}
	return &kycpb.GetManualReviewRequestsResponse{Requests: items, Total: result.Total}, nil
}

func (h *KYCHandler) GetManualReviewRequestDetail(ctx context.Context, req *kycpb.GetManualReviewRequestDetailRequest) (*kycpb.GetManualReviewRequestDetailResponse, error) {
	if _, err := getSupportID(ctx); err != nil {
		return nil, err
	}
	h.logger.Debug("handler: GetManualReviewRequestDetail called", zap.String("userID", req.UserId))

	detail, err := h.service.GetManualReviewRequestDetail(ctx, req.UserId)
	if err != nil {
		h.logger.Error("handler: GetManualReviewRequestDetail failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	docs := make([]*kycpb.ManualReviewDocument, 0, len(detail.Documents))
	for _, d := range detail.Documents {
		pd := &kycpb.ManualReviewDocument{
			DocumentId:          d.DocumentID,
			DocumentType:        d.DocumentType,
			LogicalDocumentType: d.LogicalDocumentType,
			Status:              d.Status,
			OwnerKind:           d.OwnerKind,
			OwnerId:             d.OwnerID,
			Category:            d.Category,
			// Métadonnées document (recto / document principal)
			DocumentUrl:      d.DocumentURL,
			FileSizeBytes:    d.FileSizeBytes,
			MimeType:         d.MimeType,
			DocumentNumber:   d.DocumentNumber,
			IsCurrent:        d.IsCurrent,
			UploadedAt:       d.UploadedAt,
			UpdatedAt:        d.UpdatedAt,
			IssuedAt:         d.IssuedAt,
			ExpiredAt:        d.ExpiredAt,
			IssuingCountry:   d.IssuingCountry,
			IssuingAuthority: d.IssuingAuthority,
			// Verso recto-verso
			SecondDocumentId:    d.SecondDocumentID,
			SecondDocumentUrl:   d.SecondDocumentURL,
			SecondFileSizeBytes: d.SecondFileSizeBytes,
			SecondMimeType:      d.SecondMimeType,
			SecondUploadedAt:    d.SecondUploadedAt,
			SecondUpdatedAt:     d.SecondUpdatedAt,
		}
		if d.Vehicle != nil {
			pd.Vehicle = &kycpb.ManualReviewVehicleInfo{
				VehicleId:     d.Vehicle.VehicleID,
				Brand:         d.Vehicle.Brand,
				BrandModel:    d.Vehicle.BrandModel,
				Color:         d.Vehicle.Color,
				LicencePlate:  d.Vehicle.LicencePlate,
				NumberOfSeats: d.Vehicle.NumberOfSeats,
				IsVerified:    d.Vehicle.IsVerified,
			}
		}
		if d.LatestReview != nil {
			pd.LatestReview = &kycpb.ManualReviewDocumentReview{
				ReviewId:                  d.LatestReview.ReviewID,
				Status:                    d.LatestReview.Status,
				Decision:                  d.LatestReview.Decision,
				ReasonRejection:           d.LatestReview.ReasonRejection,
				RejectionDetails:          d.LatestReview.RejectionDetails,
				ReviewType:                d.LatestReview.ReviewType,
				ReviewedBy:                d.LatestReview.ReviewedBy,
				ReviewedByFirstName:       d.LatestReview.ReviewedByFirstName,
				ReviewedByLastName:        d.LatestReview.ReviewedByLastName,
				ReviewedByProfileImageURL: d.LatestReview.ReviewedByProfileImageURL,
			}
			if d.LatestReview.ReviewedAt != nil {
				pd.LatestReview.ReviewedAt = d.LatestReview.ReviewedAt.Format(time.RFC3339)
			}
		}
		docs = append(docs, pd)
	}

	return &kycpb.GetManualReviewRequestDetailResponse{
		UserId:          detail.User.UserID,
		Name:            detail.User.Name,
		FirstName:       detail.User.FirstName,
		Email:           detail.User.Email,
		PhoneNumber:     detail.User.PhoneNumber,
		ProfileImageURL: detail.User.ProfileImageURL,
		Documents:       docs,
	}, nil
}

// =============================================================================
// GetDocumentHistory — historique complet d'un document logique (support)
// =============================================================================

func (h *KYCHandler) GetDocumentHistory(ctx context.Context, req *kycpb.GetDocumentHistoryRequest) (*kycpb.GetDocumentHistoryResponse, error) {
	if _, err := getSupportID(ctx); err != nil {
		return nil, err
	}
	h.logger.Debug("handler: GetDocumentHistory called",
		zap.String("userID", req.UserId),
		zap.String("logicalDocumentType", req.LogicalDocumentType),
	)

	entries, err := h.service.GetDocumentHistory(ctx, req.UserId, req.LogicalDocumentType)
	if err != nil {
		h.logger.Error("handler: GetDocumentHistory failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	pbEntries := make([]*kycpb.DocumentHistoryEntry, 0, len(entries))
	for _, e := range entries {
		pe := &kycpb.DocumentHistoryEntry{
			ReviewId:                  e.ReviewID,
			Status:                    e.Status,
			Decision:                  e.Decision,
			ReasonRejection:           e.ReasonRejection,
			RejectionDetails:          e.RejectionDetails,
			Notes:                     e.Notes,
			ReviewType:                e.ReviewType,
			ReviewedBy:                e.ReviewedBy,
			AttemptNumber:             e.AttemptNumber,
			DocumentId:                e.DocumentID,
			SecondDocumentId:          e.SecondDocumentID,
			LogicalDocumentType:       e.LogicalDocumentType,
			CreatedAt:                 e.CreatedAt.Format(time.RFC3339),
			UpdatedAt:                 e.UpdatedAt.Format(time.RFC3339),
			ReviewedByFirstName:       e.ReviewedByFirstName,
			ReviewedByLastName:        e.ReviewedByLastName,
			ReviewedByProfileImageURL: e.ReviewedByProfileImageURL,
		}
		if e.ReviewedAt != nil {
			pe.ReviewedAt = e.ReviewedAt.Format(time.RFC3339)
		}
		pbEntries = append(pbEntries, pe)
	}

	return &kycpb.GetDocumentHistoryResponse{Entries: pbEntries}, nil
}

// =============================================================================
// Health
// =============================================================================

func (h *KYCHandler) Health(_ context.Context, _ *kycpb.HealthRequest) (*kycpb.HealthResponse, error) {
	return &kycpb.HealthResponse{
		Status:    "SERVING",
		Version:   serviceVersion,
		Timestamp: time.Now().Unix(),
	}, nil
}

// =============================================================================
// Error mapping
// =============================================================================

// toGRPCError traduit les erreurs domaine en codes de statut gRPC.
func toGRPCError(err error) error {
	switch {
	// 3 - INVALID_ARGUMENT
	case errors.Is(err, kycErrors.ErrorMissingUserID),
		errors.Is(err, kycErrors.ErrorMissingDocumentType),
		errors.Is(err, kycErrors.ErrorMissingDocumentID),
		errors.Is(err, kycErrors.ErrorDocumentMismatch),
		errors.Is(err, kycErrors.ErrorMissingInquiryID),
		errors.Is(err, kycErrors.ErrorMissingReviewID),
		errors.Is(err, kycErrors.ErrorInvalidDecision),
		errors.Is(err, kycErrors.ErrorInvalidDateRange),
		errors.Is(err, kycErrors.ErrorVehicleDocumentNotAllowed):
		return status.Error(codes.InvalidArgument, err.Error())

	// 5 - NOT_FOUND
	case errors.Is(err, kycErrors.ErrorInquiryNotFound),
		errors.Is(err, kycErrors.ErrorReviewNotFound),
		errors.Is(err, kycErrors.ErrorUserNotFound):
		return status.Error(codes.NotFound, err.Error())

	// 6 - ALREADY_EXISTS (409)
	case errors.Is(err, kycErrors.ErrorInquiryAlreadyActive):
		return status.Error(codes.AlreadyExists, err.Error())

	// 7 - PERMISSION_DENIED (403)
	case errors.Is(err, kycErrors.ErrorUnauthorized):
		return status.Error(codes.PermissionDenied, err.Error())

	// 9 - FAILED_PRECONDITION (410 / not overridable)
	case errors.Is(err, kycErrors.ErrorInquiryNotResumable),
		errors.Is(err, kycErrors.ErrorReviewNotOverridable),
		errors.Is(err, kycErrors.ErrorOnlyRejectionOverridable),
		errors.Is(err, kycErrors.ErrorDocumentAlreadyReviewed),
		errors.Is(err, kycErrors.ErrorCompanionDocumentMissing):
		return status.Error(codes.FailedPrecondition, err.Error())

	// 16 - UNAUTHENTICATED (invalid webhook signature)
	case errors.Is(err, kycErrors.ErrorInvalidWebhookSignature):
		return status.Error(codes.Unauthenticated, err.Error())

	// 14 - UNAVAILABLE (503)
	case errors.Is(err, kycErrors.ErrorFileServiceUnavailable),
		errors.Is(err, kycErrors.ErrorPersonaUnavailable):
		return status.Error(codes.Unavailable, err.Error())

	// 13 - INTERNAL
	case errors.Is(err, kycErrors.ErrorInternalServer):
		return status.Error(codes.Internal, err.Error())

	default:
		return status.Error(codes.Internal, "internal server error")
	}
}

// =============================================================================
// Proto converters
// =============================================================================

func toProtoInquiryDetail(d *serviceInterfaces.InquiryDetail) *kycpb.InquiryDetail {
	return &kycpb.InquiryDetail{
		ReviewId:          d.ReviewID,
		PersonaInquiryId:  d.PersonaInquiryID,
		UserDocumentId:    d.UserDocumentID,
		PersonaTemplateId: d.PersonaTemplateID,
		VehicleDocumentId: d.VehicleDocumentID,
		Status:            d.Status,
		Decision:          d.Decision,
		ReasonRejection:   d.ReasonRejection,
		RejectionDetails:  d.RejectionDetails,
		AttemptNumber:     d.AttemptNumber,
		PreviousReviewId:  d.PreviousReviewID,
		ReviewType:        d.ReviewType,
		ReviewedAt:        d.ReviewedAt,
		CreatedAt:         d.CreatedAt,
		UpdatedAt:         d.UpdatedAt,
	}
}

func toProtoAdminReviewItem(item *serviceInterfaces.AdminReviewItem) *kycpb.AdminReviewItem {
	return &kycpb.AdminReviewItem{
		ReviewId:          item.ReviewID,
		PersonaInquiryId:  item.PersonaInquiryID,
		UserDocumentId:    item.UserDocumentID,
		VehicleDocumentId: item.VehicleDocumentID,
		Status:            item.Status,
		Decision:          item.Decision,
		ReasonRejection:   item.ReasonRejection,
		RejectionDetails:  item.RejectionDetails,
		ReviewType:        item.ReviewType,
		ReviewedAt:        item.ReviewedAt,
		AttemptNumber:     item.AttemptNumber,
		PreviousReviewId:  item.PreviousReviewID,
		WebhookEventType:  item.WebhookEventType,
		WebhookReceivedAt: item.WebhookReceivedAt,
		CreatedAt:         item.CreatedAt,
		UpdatedAt:         item.UpdatedAt,
	}
}

func toProtoAdminReviewDetail(d *serviceInterfaces.AdminReviewDetail) *kycpb.AdminReviewDetail {
	return &kycpb.AdminReviewDetail{
		ReviewId:          d.ReviewID,
		PersonaInquiryId:  d.PersonaInquiryID,
		PersonaTemplateId: d.PersonaTemplateID,
		UserDocumentId:    d.UserDocumentID,
		VehicleDocumentId: d.VehicleDocumentID,
		Status:            d.Status,
		Decision:          d.Decision,
		ReasonRejection:   d.ReasonRejection,
		RejectionDetails:  d.RejectionDetails,
		ReviewedBy:        d.ReviewedBy,
		ReviewType:        d.ReviewType,
		ReviewedAt:        d.ReviewedAt,
		Notes:             d.Notes,
		ExtractedData:     d.ExtractedData,
		WebhookEventType:  d.WebhookEventType,
		WebhookReceivedAt: d.WebhookReceivedAt,
		AttemptNumber:     d.AttemptNumber,
		PreviousReviewId:  d.PreviousReviewID,
		SessionExpiresAt:  d.SessionExpiresAt,
		CreatedAt:         d.CreatedAt,
		UpdatedAt:         d.UpdatedAt,
	}
}
