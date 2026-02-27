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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const serviceVersion = "1.0.0"

// FileHandler implémente filepb.FileServiceServer.
type FileHandler struct {
	filepb.UnimplementedFileServiceServer
	service serviceInterfaces.FileService
}

func NewFileHandler(service serviceInterfaces.FileService) *FileHandler {
	return &FileHandler{service: service}
}

// --- Upload streaming : documents utilisateur ---

func (h *FileHandler) UploadUserDocument(stream filepb.FileService_UploadUserDocumentServer) error {
	// 1er message : métadonnées
	firstMsg, err := stream.Recv()
	if err != nil {
		return status.Error(codes.InvalidArgument, "failed to receive metadata")
	}

	metadata := firstMsg.GetMetadata()
	if metadata == nil {
		return status.Error(codes.InvalidArgument, "first message must contain metadata")
	}

	// Lire les chunks suivants
	var buf bytes.Buffer
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
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
		return toGRPCError(err)
	}

	return stream.SendAndClose(toProtoUserDocument(doc))
}

// --- Upload streaming : documents véhicule ---

func (h *FileHandler) UploadVehicleDocument(stream filepb.FileService_UploadVehicleDocumentServer) error {
	firstMsg, err := stream.Recv()
	if err != nil {
		return status.Error(codes.InvalidArgument, "failed to receive metadata")
	}

	metadata := firstMsg.GetMetadata()
	if metadata == nil {
		return status.Error(codes.InvalidArgument, "first message must contain metadata")
	}

	var buf bytes.Buffer
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
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
		return toGRPCError(err)
	}

	return stream.SendAndClose(toProtoVehicleDocument(doc))
}

// --- Lecture ---

func (h *FileHandler) GetUserDocuments(ctx context.Context, req *filepb.GetUserDocumentsRequest) (*filepb.GetUserDocumentsResponse, error) {
	docs, err := h.service.GetUserDocuments(ctx, req.UserId)
	if err != nil {
		return nil, toGRPCError(err)
	}

	var protoDocs []*filepb.UserDocumentResponse
	for _, doc := range docs {
		protoDocs = append(protoDocs, toProtoUserDocument(doc))
	}
	return &filepb.GetUserDocumentsResponse{Documents: protoDocs}, nil
}

func (h *FileHandler) GetUserDocument(ctx context.Context, req *filepb.GetDocumentByIDRequest) (*filepb.UserDocumentResponse, error) {
	doc, err := h.service.GetUserDocument(ctx, req.DocumentId)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return toProtoUserDocument(doc), nil
}

func (h *FileHandler) GetCurrentUserDocument(ctx context.Context, req *filepb.GetCurrentUserDocumentRequest) (*filepb.UserDocumentResponse, error) {
	doc, err := h.service.GetCurrentUserDocument(ctx, req.UserId, req.DocumentType)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return toProtoUserDocument(doc), nil
}

func (h *FileHandler) GetVehicleDocuments(ctx context.Context, req *filepb.GetVehicleDocumentsRequest) (*filepb.GetVehicleDocumentsResponse, error) {
	docs, err := h.service.GetVehicleDocuments(ctx, req.VehicleId)
	if err != nil {
		return nil, toGRPCError(err)
	}

	var protoDocs []*filepb.VehicleDocumentResponse
	for _, doc := range docs {
		protoDocs = append(protoDocs, toProtoVehicleDocument(doc))
	}
	return &filepb.GetVehicleDocumentsResponse{Documents: protoDocs}, nil
}

func (h *FileHandler) GetVehicleDocument(ctx context.Context, req *filepb.GetDocumentByIDRequest) (*filepb.VehicleDocumentResponse, error) {
	doc, err := h.service.GetVehicleDocument(ctx, req.DocumentId)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return toProtoVehicleDocument(doc), nil
}

// --- Suppression ---

func (h *FileHandler) DeleteUserDocument(ctx context.Context, req *filepb.DeleteDocumentRequest) (*filepb.OperationResponse, error) {
	if err := h.service.DeleteUserDocument(ctx, req.DocumentId); err != nil {
		return nil, toGRPCError(err)
	}
	return &filepb.OperationResponse{Success: true}, nil
}

func (h *FileHandler) DeleteVehicleDocument(ctx context.Context, req *filepb.DeleteDocumentRequest) (*filepb.OperationResponse, error) {
	if err := h.service.DeleteVehicleDocument(ctx, req.DocumentId); err != nil {
		return nil, toGRPCError(err)
	}
	return &filepb.OperationResponse{Success: true}, nil
}

// --- Revues ---

func (h *FileHandler) CreateDocumentReview(ctx context.Context, req *filepb.CreateDocumentReviewRequest) (*filepb.DocumentReviewResponse, error) {
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
		return nil, toGRPCError(err)
	}
	return toProtoDocumentReview(review), nil
}

func (h *FileHandler) GetDocumentReviews(ctx context.Context, req *filepb.GetDocumentReviewsRequest) (*filepb.GetDocumentReviewsResponse, error) {
	reviews, err := h.service.GetDocumentReviews(ctx, req.UserDocumentId, req.VehicleDocumentId)
	if err != nil {
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
		DocumentId:    doc.DocumentID,
		UserId:        doc.UserID,
		DocumentName:  doc.DocumentName,
		DocumentType:  doc.DocumentType,
		DocumentUrl:   doc.DocumentURL,
		FileSizeBytes: doc.FileSizeBytes,
		MimeType:      doc.MimeType,
		DocumentNumber: doc.DocumentNumber,
		IssuingCountry: doc.IssuingCountry,
		Status:        doc.Status,
		IsCurrent:     doc.IsCurrent,
		UploadedAt:    doc.UploadedAt.Format(time.RFC3339),
		UpdatedAt:     doc.UpdatedAt.Format(time.RFC3339),
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
		ReviewId:       review.ReviewID,
		Decision:       review.Decision,
		ReasonRejection:  review.ReasonRejection,
		RejectionDetails: review.RejectionDetails,
		ReviewedBy:      review.ReviewedBy,
		ReviewedByType:  review.ReviewedByType,
		ReviewedAt:      review.ReviewedAt.Format(time.RFC3339),
		Notes:           review.Notes,
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
		errors.Is(err, fileErrors.ErrorInvalidReviewDecision):
		return status.Error(codes.InvalidArgument, err.Error())

	case errors.Is(err, fileErrors.ErrorFileTooLarge):
		return status.Error(codes.ResourceExhausted, err.Error())

	case errors.Is(err, fileErrors.ErrorUploadFailed):
		return status.Error(codes.Unavailable, err.Error())

	default:
		return status.Error(codes.Internal, err.Error())
	}
}
