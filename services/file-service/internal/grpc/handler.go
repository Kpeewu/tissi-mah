package grpc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"time"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/domain"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/file-service/internal/service/interfaces"
	fileErrors "github.com/Kpeewu/tissi-mah/services/file-service/pkg/errors"
	filepb "github.com/Kpeewu/tissi-mah/services/file-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const serviceVersion = "1.0.0"

// FileHandler implémente filepb.FileServiceServer.
type FileHandler struct {
	filepb.UnimplementedFileServiceServer
	service serviceInterfaces.FileService
	logger  *zap.Logger
}

func NewFileHandler(service serviceInterfaces.FileService, logger *zap.Logger) *FileHandler {
	return &FileHandler{service: service, logger: logger}
}

// --- Upload streaming : documents utilisateur ---

func (h *FileHandler) UploadUserDocument(stream filepb.FileService_UploadUserDocumentServer) error {
	h.logger.Debug("handler: UploadUserDocument called")

	firstMsg, err := stream.Recv()
	if err != nil {
		h.logger.Error("handler: UploadUserDocument - failed to receive metadata", zap.Error(err))
		return status.Error(codes.InvalidArgument, "failed to receive metadata")
	}

	metadata := firstMsg.GetMetadata()
	if metadata == nil {
		h.logger.Error("handler: UploadUserDocument - first message has no metadata")
		return status.Error(codes.InvalidArgument, "first message must contain metadata")
	}

	h.logger.Debug("handler: UploadUserDocument metadata",
		zap.String("userID", metadata.UserId),
		zap.String("type", metadata.DocumentType),
		zap.String("mime", metadata.MimeType),
	)

	var buf bytes.Buffer
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			h.logger.Error("handler: UploadUserDocument - failed to receive chunk", zap.Error(err))
			return status.Error(codes.Internal, "failed to receive chunk")
		}
		chunk := msg.GetChunk()
		if chunk != nil {
			buf.Write(chunk)
		}
	}

	input := serviceInterfaces.UploadUserDocumentInput{
		UserID:         metadata.UserId,
		DocumentName:   metadata.DocumentName,
		DocumentType:   metadata.DocumentType,
		MimeType:       metadata.MimeType,
		FileSizeBytes:  metadata.FileSizeBytes,
		Data:           &buf,
		DocumentNumber: metadata.DocumentNumber,
		IssuingCountry: metadata.IssuingCountry,
	}

	doc, err := h.service.UploadUserDocument(stream.Context(), input)
	if err != nil {
		h.logger.Error("handler: UploadUserDocument failed", zap.Error(err))
		return toGRPCError(err)
	}

	h.logger.Info("handler: UploadUserDocument success", zap.String("documentID", doc.DocumentID))
	return stream.SendAndClose(toProtoUserDocument(doc))
}

// --- Suppression document (HTTP via api-gateway) ---

// DeleteFile supprime un document après vérification que UserID est bien propriétaire.
func (h *FileHandler) DeleteFile(ctx context.Context, req *filepb.DeleteFileRequest) (*filepb.DeleteFileResponse, error) {
	h.logger.Debug("handler: DeleteFile called",
		zap.String("userID", req.UserID),
		zap.String("fileID", req.FileID),
	)

	err := h.service.DeleteFile(ctx, serviceInterfaces.DeleteFileInput{
		UserID: req.UserID,
		FileID: req.FileID,
	})
	if err != nil {
		h.logger.Error("handler: DeleteFile failed", zap.String("fileID", req.FileID), zap.Error(err))
		return &filepb.DeleteFileResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	h.logger.Info("handler: DeleteFile success", zap.String("fileID", req.FileID))
	return &filepb.DeleteFileResponse{Success: true}, nil
}

// --- Lecture document (HTTP via api-gateway) ---

// GetDocument récupère un document par son ID avec contrôle d'accès.
// Si UserID fourni : vérifie la propriété. Si SupportID fourni : accès direct.
func (h *FileHandler) GetDocument(ctx context.Context, req *filepb.GetDocumentRequest) (*filepb.GetDocumentResponse, error) {
	h.logger.Debug("handler: GetDocument called",
		zap.String("fileID", req.FileID),
		zap.String("userID", req.UserID),
		zap.String("supportID", req.SupportID),
	)

	result, err := h.service.GetDocument(ctx, serviceInterfaces.GetDocumentInput{
		FileID:    req.FileID,
		UserID:    req.UserID,
		SupportID: req.SupportID,
	})
	if err != nil {
		h.logger.Error("handler: GetDocument failed", zap.String("fileID", req.FileID), zap.Error(err))
		return &filepb.GetDocumentResponse{ErrorMessage: err.Error()}, nil
	}

	h.logger.Info("handler: GetDocument success", zap.String("fileID", req.FileID))
	return &filepb.GetDocumentResponse{
		File: &filepb.DocumentFile{
			FileID:   result.FileID,
			FileURL:  result.FileURL,
			FileType: result.FileType,
		},
	}, nil
}

// --- Remplacement de document (HTTP via api-gateway) ---

// ChangeDocument remplace le fichier d'un document utilisateur existant.
func (h *FileHandler) ChangeDocument(ctx context.Context, req *filepb.ChangeDocumentRequest) (*filepb.ChangeDocumentResponse, error) {
	h.logger.Debug("handler: ChangeDocument called",
		zap.String("userID", req.UserID),
		zap.String("fileID", req.FileID),
	)

	err := h.service.ChangeDocument(ctx, serviceInterfaces.ChangeDocumentInput{
		UserID:      req.UserID,
		FileID:      req.FileID,
		NewDocument: req.NewDocument,
	})
	if err != nil {
		h.logger.Error("handler: ChangeDocument failed",
			zap.String("fileID", req.FileID),
			zap.Error(err),
		)
		return &filepb.ChangeDocumentResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	h.logger.Info("handler: ChangeDocument success", zap.String("fileID", req.FileID))
	return &filepb.ChangeDocumentResponse{Success: true}, nil
}

// --- Upload identité (HTTP via api-gateway) ---

// UploadIdDocument reçoit les documents d'identité en base64 JSON, les upload vers S3/MinIO
// et sauvegarde les URLs en base. Retourne toujours HTTP 200 avec ErrorMessage si erreur.
func (h *FileHandler) UploadIdDocument(ctx context.Context, req *filepb.UploadIdDocumentRequest) (*filepb.UploadIdDocumentResponse, error) {
	h.logger.Debug("handler: UploadIdDocument called",
		zap.String("profileID", req.UserID),
		zap.String("documentType", req.DocumentType),
	)

	err := h.service.UploadIdDocument(ctx, serviceInterfaces.UploadIdDocumentInput{
		UserID:          req.UserID,
		DocumentType:       req.DocumentType,
		IDCardRecto:        req.IDCardRecto,
		IDCardVerso:        req.IDCardVerso,
		DriverLicenceRecto: req.DriverLicenceRecto,
		DriverLicenceVerso: req.DriverLicenceVerso,
		Passport:           req.Passport,
	})
	if err != nil {
		h.logger.Error("handler: UploadIdDocument failed",
			zap.String("profileID", req.UserID),
			zap.Error(err),
		)
		return &filepb.UploadIdDocumentResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	h.logger.Info("handler: UploadIdDocument success", zap.String("profileID", req.UserID))
	return &filepb.UploadIdDocumentResponse{Success: true}, nil
}

// UploadVehicleDocuments reçoit les documents du véhicule en base64 JSON,
// les upload vers S3/MinIO et sauvegarde les URLs en base.
func (h *FileHandler) UploadVehicleDocuments(ctx context.Context, req *filepb.UploadVehicleDocumentsRequest) (*filepb.UploadVehicleDocumentsResponse, error) {
	h.logger.Debug("handler: UploadVehicleDocuments called",
		zap.String("profileID", req.UserID),
		zap.String("vehicleID", req.VehicleID),
	)

	err := h.service.UploadVehicleDocuments(ctx, serviceInterfaces.UploadVehicleDocumentsInput{
		UserID:           req.UserID,
		VehicleID:           req.VehicleID,
		DriverLicenceImage:  req.DriverLicenceImage,
		Assurance:           req.Assurance,
		VehicleRegistration: req.VehicleRegistration,
	})
	if err != nil {
		h.logger.Error("handler: UploadVehicleDocuments failed",
			zap.String("vehicleID", req.VehicleID),
			zap.Error(err),
		)
		return &filepb.UploadVehicleDocumentsResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	h.logger.Info("handler: UploadVehicleDocuments success", zap.String("vehicleID", req.VehicleID))
	return &filepb.UploadVehicleDocumentsResponse{Success: true}, nil
}

// --- Upload streaming : documents véhicule ---

func (h *FileHandler) UploadVehicleDocument(stream filepb.FileService_UploadVehicleDocumentServer) error {
	h.logger.Debug("handler: UploadVehicleDocument called")

	firstMsg, err := stream.Recv()
	if err != nil {
		h.logger.Error("handler: UploadVehicleDocument - failed to receive metadata", zap.Error(err))
		return status.Error(codes.InvalidArgument, "failed to receive metadata")
	}

	metadata := firstMsg.GetMetadata()
	if metadata == nil {
		h.logger.Error("handler: UploadVehicleDocument - first message has no metadata")
		return status.Error(codes.InvalidArgument, "first message must contain metadata")
	}

	h.logger.Debug("handler: UploadVehicleDocument metadata",
		zap.String("vehicleID", metadata.VehicleId),
		zap.String("type", metadata.DocumentType),
	)

	var buf bytes.Buffer
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			h.logger.Error("handler: UploadVehicleDocument - failed to receive chunk", zap.Error(err))
			return status.Error(codes.Internal, "failed to receive chunk")
		}
		chunk := msg.GetChunk()
		if chunk != nil {
			buf.Write(chunk)
		}
	}

	input := serviceInterfaces.UploadVehicleDocumentInput{
		VehicleID:        metadata.VehicleId,
		DocumentName:     metadata.DocumentName,
		DocumentType:     metadata.DocumentType,
		MimeType:         metadata.MimeType,
		FileSizeBytes:    metadata.FileSizeBytes,
		Data:             &buf,
		DocumentNumber:   metadata.DocumentNumber,
		IssuingAuthority: metadata.IssuingAuthority,
	}

	doc, err := h.service.UploadVehicleDocument(stream.Context(), input)
	if err != nil {
		h.logger.Error("handler: UploadVehicleDocument failed", zap.Error(err))
		return toGRPCError(err)
	}

	h.logger.Info("handler: UploadVehicleDocument success", zap.String("documentID", doc.DocumentID))
	return stream.SendAndClose(toProtoVehicleDocument(doc))
}

// --- Lecture ---

func (h *FileHandler) GetUserDocuments(ctx context.Context, req *filepb.GetUserDocumentsRequest) (*filepb.GetUserDocumentsResponse, error) {
	h.logger.Debug("handler: GetUserDocuments", zap.String("userID", req.UserId))
	docs, err := h.service.GetUserDocuments(ctx, req.UserId)
	if err != nil {
		h.logger.Error("handler: GetUserDocuments failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	var protoDocs []*filepb.UserDocumentResponse
	for _, doc := range docs {
		protoDocs = append(protoDocs, toProtoUserDocument(doc))
	}
	return &filepb.GetUserDocumentsResponse{Documents: protoDocs}, nil
}

func (h *FileHandler) GetUserDocument(ctx context.Context, req *filepb.GetDocumentByIDRequest) (*filepb.UserDocumentResponse, error) {
	h.logger.Debug("handler: GetUserDocument", zap.String("documentID", req.DocumentId))
	doc, err := h.service.GetUserDocument(ctx, req.DocumentId)
	if err != nil {
		h.logger.Error("handler: GetUserDocument failed", zap.Error(err))
		return nil, toGRPCError(err)
	}
	return toProtoUserDocument(doc), nil
}

func (h *FileHandler) GetCurrentUserDocument(ctx context.Context, req *filepb.GetCurrentUserDocumentRequest) (*filepb.UserDocumentResponse, error) {
	h.logger.Debug("handler: GetCurrentUserDocument", zap.String("userID", req.UserId), zap.String("type", req.DocumentType))
	doc, err := h.service.GetCurrentUserDocument(ctx, req.UserId, req.DocumentType)
	if err != nil {
		h.logger.Error("handler: GetCurrentUserDocument failed", zap.Error(err))
		return nil, toGRPCError(err)
	}
	return toProtoUserDocument(doc), nil
}

func (h *FileHandler) GetVehicleDocuments(ctx context.Context, req *filepb.GetVehicleDocumentsRequest) (*filepb.GetVehicleDocumentsResponse, error) {
	h.logger.Debug("handler: GetVehicleDocuments", zap.String("vehicleID", req.VehicleId))
	docs, err := h.service.GetVehicleDocuments(ctx, req.VehicleId)
	if err != nil {
		h.logger.Error("handler: GetVehicleDocuments failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	var protoDocs []*filepb.VehicleDocumentResponse
	for _, doc := range docs {
		protoDocs = append(protoDocs, toProtoVehicleDocument(doc))
	}
	return &filepb.GetVehicleDocumentsResponse{Documents: protoDocs}, nil
}

func (h *FileHandler) GetVehicleDocument(ctx context.Context, req *filepb.GetDocumentByIDRequest) (*filepb.VehicleDocumentResponse, error) {
	h.logger.Debug("handler: GetVehicleDocument", zap.String("documentID", req.DocumentId))
	doc, err := h.service.GetVehicleDocument(ctx, req.DocumentId)
	if err != nil {
		h.logger.Error("handler: GetVehicleDocument failed", zap.Error(err))
		return nil, toGRPCError(err)
	}
	return toProtoVehicleDocument(doc), nil
}

// --- Suppression ---

func (h *FileHandler) DeleteUserDocument(ctx context.Context, req *filepb.DeleteDocumentRequest) (*filepb.OperationResponse, error) {
	h.logger.Debug("handler: DeleteUserDocument", zap.String("documentID", req.DocumentId))
	if err := h.service.DeleteUserDocument(ctx, req.DocumentId); err != nil {
		h.logger.Error("handler: DeleteUserDocument failed", zap.Error(err))
		return nil, toGRPCError(err)
	}
	h.logger.Info("handler: DeleteUserDocument success", zap.String("documentID", req.DocumentId))
	return &filepb.OperationResponse{Success: true}, nil
}

func (h *FileHandler) DeleteVehicleDocument(ctx context.Context, req *filepb.DeleteDocumentRequest) (*filepb.OperationResponse, error) {
	h.logger.Debug("handler: DeleteVehicleDocument", zap.String("documentID", req.DocumentId))
	if err := h.service.DeleteVehicleDocument(ctx, req.DocumentId); err != nil {
		h.logger.Error("handler: DeleteVehicleDocument failed", zap.Error(err))
		return nil, toGRPCError(err)
	}
	h.logger.Info("handler: DeleteVehicleDocument success", zap.String("documentID", req.DocumentId))
	return &filepb.OperationResponse{Success: true}, nil
}

// --- Revues ---

func (h *FileHandler) CreateDocumentReview(ctx context.Context, req *filepb.CreateDocumentReviewRequest) (*filepb.DocumentReviewResponse, error) {
	h.logger.Debug("handler: CreateDocumentReview",
		zap.String("userDocID", req.UserDocumentId),
		zap.String("vehicleDocID", req.VehicleDocumentId),
		zap.String("decision", req.Decision),
	)

	input := serviceInterfaces.CreateReviewInput{
		UserDocumentID:    req.UserDocumentId,
		VehicleDocumentID: req.VehicleDocumentId,
		Decision:          req.Decision,
		ReasonRejection:   req.ReasonRejection,
		RejectionDetails:  req.RejectionDetails,
		ReviewedBy:        req.ReviewedBy,
		ReviewedByType:    req.ReviewedByType,
		Notes:             req.Notes,
		ExtractedData:     req.ExtractedData,
	}

	review, err := h.service.CreateDocumentReview(ctx, input)
	if err != nil {
		h.logger.Error("handler: CreateDocumentReview failed", zap.Error(err))
		return nil, toGRPCError(err)
	}
	h.logger.Info("handler: CreateDocumentReview success", zap.String("reviewID", review.ReviewID))
	return toProtoDocumentReview(review), nil
}

func (h *FileHandler) GetDocumentReviews(ctx context.Context, req *filepb.GetDocumentReviewsRequest) (*filepb.GetDocumentReviewsResponse, error) {
	h.logger.Debug("handler: GetDocumentReviews",
		zap.String("userDocID", req.UserDocumentId),
		zap.String("vehicleDocID", req.VehicleDocumentId),
	)
	reviews, err := h.service.GetDocumentReviews(ctx, req.UserDocumentId, req.VehicleDocumentId)
	if err != nil {
		h.logger.Error("handler: GetDocumentReviews failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	var protoReviews []*filepb.DocumentReviewResponse
	for _, review := range reviews {
		protoReviews = append(protoReviews, toProtoDocumentReview(review))
	}
	return &filepb.GetDocumentReviewsResponse{Reviews: protoReviews}, nil
}

// --- Health ---

func (h *FileHandler) Health(_ context.Context, _ *filepb.HealthRequest) (*filepb.HealthResponse, error) {
	return &filepb.HealthResponse{
		Status:    "healthy",
		Version:   serviceVersion,
		Timestamp: time.Now().Unix(),
	}, nil
}

// =============================================================================
// Mappers domain → proto
// =============================================================================

func toProtoUserDocument(doc *domain.UserDocument) *filepb.UserDocumentResponse {
	return &filepb.UserDocumentResponse{
		DocumentId:     doc.DocumentID,
		UserId:         doc.UserID,
		DocumentName:   doc.DocumentName,
		DocumentType:   doc.DocumentType,
		DocumentUrl:    doc.DocumentURL,
		FileSizeBytes:  doc.FileSizeBytes,
		MimeType:       doc.MimeType,
		DocumentNumber: doc.DocumentNumber,
		IssuingCountry: doc.IssuingCountry,
		Status:         doc.Status,
		IsCurrent:      doc.IsCurrent,
		UploadedAt:     doc.UploadedAt.Format(time.RFC3339),
		UpdatedAt:      doc.UpdatedAt.Format(time.RFC3339),
	}
}

func toProtoVehicleDocument(doc *domain.VehicleDocument) *filepb.VehicleDocumentResponse {
	return &filepb.VehicleDocumentResponse{
		DocumentId:       doc.DocumentID,
		VehicleId:        doc.VehicleID,
		DocumentName:     doc.DocumentName,
		DocumentType:     doc.DocumentType,
		DocumentUrl:      doc.DocumentURL,
		FileSizeBytes:    doc.FileSizeBytes,
		MimeType:         doc.MimeType,
		DocumentNumber:   doc.DocumentNumber,
		IssuingAuthority: doc.IssuingAuthority,
		Status:           doc.Status,
		IsCurrent:        doc.IsCurrent,
		UploadedAt:       doc.UploadedAt.Format(time.RFC3339),
		UpdatedAt:        doc.UpdatedAt.Format(time.RFC3339),
	}
}

func toProtoDocumentReview(review *domain.DocumentReview) *filepb.DocumentReviewResponse {
	resp := &filepb.DocumentReviewResponse{
		ReviewId:         review.ReviewID,
		Decision:         review.Decision,
		ReasonRejection:  review.ReasonRejection,
		RejectionDetails: review.RejectionDetails,
		ReviewedBy:       review.ReviewedBy,
		ReviewedByType:   review.ReviewedByType,
		ReviewedAt:       review.ReviewedAt.Format(time.RFC3339),
		Notes:            review.Notes,
	}
	if review.UserDocumentID != nil {
		resp.UserDocumentId = *review.UserDocumentID
	}
	if review.VehicleDocumentID != nil {
		resp.VehicleDocumentId = *review.VehicleDocumentID
	}
	return resp
}

// =============================================================================
// Error mapping domain → gRPC
// =============================================================================

func toGRPCError(err error) error {
	switch {
	case errors.Is(err, fileErrors.ErrorDocumentNotFound),
		errors.Is(err, fileErrors.ErrorReviewNotFound):
		return status.Error(codes.NotFound, err.Error())

	case errors.Is(err, fileErrors.ErrorInvalidDocumentType),
		errors.Is(err, fileErrors.ErrorInvalidMimeType),
		errors.Is(err, fileErrors.ErrorInvalidReviewDecision),
		errors.Is(err, fileErrors.ErrorMissingDocumentReference),
		errors.Is(err, fileErrors.ErrorMultipleDocumentReference):
		return status.Error(codes.InvalidArgument, err.Error())

	case errors.Is(err, fileErrors.ErrorUnauthorized):
		return status.Error(codes.PermissionDenied, err.Error())

	case errors.Is(err, fileErrors.ErrorFileTooLarge):
		return status.Error(codes.ResourceExhausted, err.Error())

	case errors.Is(err, fileErrors.ErrorUploadFailed):
		return status.Error(codes.Unavailable, err.Error())

	default:
		return status.Error(codes.Internal, err.Error())
	}
}
