package grpc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"time"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/file-service/internal/domain"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/file-service/internal/service/interfaces"
	"github.com/Kpeewu/tissi-mah/services/file-service/internal/storage"
	fileErrors "github.com/Kpeewu/tissi-mah/services/file-service/pkg/errors"
	filepb "github.com/Kpeewu/tissi-mah/services/file-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const serviceVersion = "1.0.0"

const defaultPresignTTL = 1 * time.Hour
const maxPresignTTL = 24 * time.Hour

// FileHandler implémente filepb.FileServiceServer.
type FileHandler struct {
	filepb.UnimplementedFileServiceServer
	service       serviceInterfaces.FileService
	userClient    client.UserClient
	vehicleClient client.VehicleClient
	storage       storage.StorageClient
	logger        *zap.Logger
}

func NewFileHandler(service serviceInterfaces.FileService, userClient client.UserClient, vehicleClient client.VehicleClient, storageClient storage.StorageClient, logger *zap.Logger) *FileHandler {
	return &FileHandler{service: service, userClient: userClient, vehicleClient: vehicleClient, storage: storageClient, logger: logger}
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

	presignedURL, _ := h.storage.GeneratePresignedURL(stream.Context(), doc.DocumentKey, defaultPresignTTL)
	h.logger.Info("handler: UploadUserDocument success", zap.String("documentID", doc.DocumentID))
	return stream.SendAndClose(toProtoUserDocument(doc, presignedURL))
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
// Si UserID fourni : vérifie la propriété. Si x-support-uid présent en metadata : accès direct.
// Le SupportID vient exclusivement de la metadata gRPC injectée par JWTSupport — req.SupportID est ignoré.
func (h *FileHandler) GetDocument(ctx context.Context, req *filepb.GetDocumentRequest) (*filepb.GetDocumentResponse, error) {
	var supportID string
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("x-support-uid"); len(vals) > 0 {
			supportID = vals[0]
		}
	}

	h.logger.Debug("handler: GetDocument called",
		zap.String("fileID", req.FileID),
		zap.String("userID", req.UserID),
		zap.Bool("isSupportAccess", supportID != ""),
	)

	result, err := h.service.GetDocument(ctx, serviceInterfaces.GetDocumentInput{
		FileID:    req.FileID,
		UserID:    req.UserID,
		SupportID: supportID,
	})
	if err != nil {
		h.logger.Error("handler: GetDocument failed", zap.String("fileID", req.FileID), zap.Error(err))
		return &filepb.GetDocumentResponse{ErrorMessage: err.Error()}, nil
	}

	h.logger.Info("handler: GetDocument success", zap.String("fileID", req.FileID))
	return &filepb.GetDocumentResponse{
		File: &filepb.DocumentFile{
			FileID:                result.FileID,
			FileURL:               result.FileURL,
			FileType:              result.FileType,
			PresignedUrlExpiresAt: result.PresignedURLExpiresAt,
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

	doc, err := h.service.ChangeDocument(ctx, serviceInterfaces.ChangeDocumentInput{
		UserID:         req.UserID,
		FileID:         req.FileID,
		NewDocument:    req.NewDocument,
		DocumentNumber: req.DocumentNumber,
		IssuedAt:       req.IssuedAt,
		ExpireAt:       req.ExpireAt,
		IssuingPlace:   req.IssuingPlace,
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

	h.logger.Info("handler: ChangeDocument success",
		zap.String("oldFileID", req.FileID),
		zap.String("newFileID", doc.DocumentID),
	)
	return &filepb.ChangeDocumentResponse{
		Success:  true,
		Document: toProtoUploadedDocument(doc),
	}, nil
}

// --- Upload identité (HTTP via api-gateway) ---

// UploadIdDocument reçoit les documents d'identité en base64 JSON, les upload vers S3/MinIO
// et sauvegarde les URLs en base. Retourne toujours HTTP 200 avec ErrorMessage si erreur.
//
// Sécurité : le Firebase UID est lu depuis la metadata gRPC x-firebase-uid (injectée
// par l'api-gateway après validation JWT), puis résolu en profil (UUID + prenom + nom)
// via user-service. Le champ req.UserID du body est ignoré car client-supplied et non sûr.
func (h *FileHandler) UploadIdDocument(ctx context.Context, req *filepb.UploadIdDocumentRequest) (*filepb.UploadIdDocumentResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		h.logger.Error("handler: UploadIdDocument - missing metadata")
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}
	uids := md.Get("x-firebase-uid")
	if len(uids) == 0 || uids[0] == "" {
		h.logger.Error("handler: UploadIdDocument - missing x-firebase-uid")
		return nil, status.Error(codes.Unauthenticated, "missing firebase uid")
	}
	firebaseUID := uids[0]

	profile, err := h.userClient.GetUserProfileByFirebaseID(ctx, firebaseUID)
	if err != nil {
		h.logger.Error("handler: UploadIdDocument - failed to resolve firebaseUID",
			zap.String("firebaseUID", firebaseUID),
			zap.Error(err),
		)
		return &filepb.UploadIdDocumentResponse{
			Success:      false,
			ErrorMessage: fileErrors.ErrorUserServiceUnavailable.Error(),
		}, nil
	}

	h.logger.Debug("handler: UploadIdDocument called",
		zap.String("firebaseUID", firebaseUID),
		zap.String("internalUserID", profile.UserID),
		zap.String("documentType", req.DocumentType),
	)

	docs, err := h.service.UploadIdDocument(ctx, serviceInterfaces.UploadIdDocumentInput{
		UserID:             profile.UserID,
		FirstName:          profile.FirstName,
		LastName:           profile.LastName,
		DocumentType:       req.DocumentType,
		IDCardRecto:        req.IDCardRecto,
		IDCardVerso:        req.IDCardVerso,
		DriverLicenceRecto: req.DriverLicenceRecto,
		DriverLicenceVerso: req.DriverLicenceVerso,
		Passport:           req.Passport,
		DocumentNumber:     req.DocumentNumber,
		IssuedAt:           req.IssuedAt,
		ExpireAt:           req.ExpireAt,
		IssuingCountry:     req.IssuingCountry,
	})
	if err != nil {
		h.logger.Error("handler: UploadIdDocument failed",
			zap.String("firebaseUID", firebaseUID),
			zap.String("internalUserID", profile.UserID),
			zap.Error(err),
		)
		return &filepb.UploadIdDocumentResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	h.logger.Info("handler: UploadIdDocument success",
		zap.String("firebaseUID", firebaseUID),
		zap.String("internalUserID", profile.UserID),
		zap.Int("count", len(docs)),
	)
	return &filepb.UploadIdDocumentResponse{
		Success:   true,
		Documents: toProtoUploadedDocuments(docs),
	}, nil
}

// UploadVehicleDocuments reçoit les documents du véhicule en base64 JSON,
// les upload vers S3/MinIO et sauvegarde les URLs en base.
//
// Sécurité : le Firebase UID est lu depuis la metadata gRPC x-firebase-uid (injectée
// par l'api-gateway après validation JWT), puis résolu en profil (UUID + prenom + nom)
// via user-service. Le champ req.UserID du body est ignoré car client-supplied et non sûr.
func (h *FileHandler) UploadVehicleDocuments(ctx context.Context, req *filepb.UploadVehicleDocumentsRequest) (*filepb.UploadVehicleDocumentsResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		h.logger.Error("handler: UploadVehicleDocuments - missing metadata")
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}
	uids := md.Get("x-firebase-uid")
	if len(uids) == 0 || uids[0] == "" {
		h.logger.Error("handler: UploadVehicleDocuments - missing x-firebase-uid")
		return nil, status.Error(codes.Unauthenticated, "missing firebase uid")
	}
	firebaseUID := uids[0]

	profile, err := h.userClient.GetUserProfileByFirebaseID(ctx, firebaseUID)
	if err != nil {
		h.logger.Error("handler: UploadVehicleDocuments - failed to resolve firebaseUID",
			zap.String("firebaseUID", firebaseUID),
			zap.Error(err),
		)
		return &filepb.UploadVehicleDocumentsResponse{
			Success:      false,
			ErrorMessage: fileErrors.ErrorUserServiceUnavailable.Error(),
		}, nil
	}

	h.logger.Debug("handler: UploadVehicleDocuments called",
		zap.String("firebaseUID", firebaseUID),
		zap.String("internalUserID", profile.UserID),
		zap.String("vehicleID", req.VehicleID),
	)

	docs, err := h.service.UploadVehicleDocuments(ctx, serviceInterfaces.UploadVehicleDocumentsInput{
		UserID:    profile.UserID,
		VehicleID: req.VehicleID,
		FirstName: profile.FirstName,
		LastName:  profile.LastName,
		DriverLicence: serviceInterfaces.VehicleDocFileInput{
			Data:             req.DriverLicenceImage,
			DocumentNumber:   req.GetDriverLicenceMetadata().GetDocumentNumber(),
			IssuedAt:         req.GetDriverLicenceMetadata().GetIssuedAt(),
			ExpireAt:         req.GetDriverLicenceMetadata().GetExpireAt(),
			IssuingAuthority: req.GetDriverLicenceMetadata().GetIssuingAuthority(),
		},
		Assurance: serviceInterfaces.VehicleDocFileInput{
			Data:             req.Assurance,
			DocumentNumber:   req.GetAssuranceMetadata().GetDocumentNumber(),
			IssuedAt:         req.GetAssuranceMetadata().GetIssuedAt(),
			ExpireAt:         req.GetAssuranceMetadata().GetExpireAt(),
			IssuingAuthority: req.GetAssuranceMetadata().GetIssuingAuthority(),
		},
		RegistrationCard: serviceInterfaces.VehicleDocFileInput{
			Data:             req.VehicleRegistration,
			DocumentNumber:   req.GetRegistrationCardMetadata().GetDocumentNumber(),
			IssuedAt:         req.GetRegistrationCardMetadata().GetIssuedAt(),
			ExpireAt:         req.GetRegistrationCardMetadata().GetExpireAt(),
			IssuingAuthority: req.GetRegistrationCardMetadata().GetIssuingAuthority(),
		},
	})
	if err != nil {
		h.logger.Error("handler: UploadVehicleDocuments failed",
			zap.String("firebaseUID", firebaseUID),
			zap.String("internalUserID", profile.UserID),
			zap.String("vehicleID", req.VehicleID),
			zap.Error(err),
		)
		return &filepb.UploadVehicleDocumentsResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	h.logger.Info("handler: UploadVehicleDocuments success",
		zap.String("firebaseUID", firebaseUID),
		zap.String("internalUserID", profile.UserID),
		zap.String("vehicleID", req.VehicleID),
		zap.Int("count", len(docs)),
	)
	return &filepb.UploadVehicleDocumentsResponse{
		Success:   true,
		Documents: toProtoUploadedDocuments(docs),
	}, nil
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

	presignedURL, _ := h.storage.GeneratePresignedURL(stream.Context(), doc.DocumentKey, defaultPresignTTL)
	h.logger.Info("handler: UploadVehicleDocument success", zap.String("documentID", doc.DocumentID))
	return stream.SendAndClose(toProtoVehicleDocument(doc, presignedURL, nil))
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
		presignedURL, _ := h.storage.GeneratePresignedURL(ctx, doc.DocumentKey, defaultPresignTTL)
		protoDocs = append(protoDocs, toProtoUserDocument(doc, presignedURL))
	}
	return &filepb.GetUserDocumentsResponse{Documents: protoDocs}, nil
}

func (h *FileHandler) GetUserDocument(ctx context.Context, req *filepb.GetDocumentByIDRequest) (*filepb.UserDocumentResponse, error) {
	h.logger.Debug("handler: GetUserDocument", zap.String("documentID", req.DocumentId))
	doc, err := h.service.GetUserDocument(ctx, req.DocumentId)
	if err != nil {
		h.logHandlerError("handler: GetUserDocument", err)
		return nil, toGRPCError(err)
	}
	ttl := presignTTLFromRequest(req.PresignTTLSecs)
	presignedURL, _ := h.storage.GeneratePresignedURL(ctx, doc.DocumentKey, ttl)
	return toProtoUserDocument(doc, presignedURL), nil
}

func (h *FileHandler) GetCurrentUserDocument(ctx context.Context, req *filepb.GetCurrentUserDocumentRequest) (*filepb.UserDocumentResponse, error) {
	h.logger.Debug("handler: GetCurrentUserDocument", zap.String("userID", req.UserId), zap.String("type", req.DocumentType))
	doc, err := h.service.GetCurrentUserDocument(ctx, req.UserId, req.DocumentType)
	if err != nil {
		h.logHandlerError("handler: GetCurrentUserDocument", err)
		return nil, toGRPCError(err)
	}
	presignedURL, _ := h.storage.GeneratePresignedURL(ctx, doc.DocumentKey, defaultPresignTTL)
	return toProtoUserDocument(doc, presignedURL), nil
}

func (h *FileHandler) GetVehicleDocuments(ctx context.Context, req *filepb.GetVehicleDocumentsRequest) (*filepb.GetVehicleDocumentsResponse, error) {
	h.logger.Debug("handler: GetVehicleDocuments", zap.String("vehicleID", req.VehicleId))
	docs, err := h.service.GetVehicleDocuments(ctx, req.VehicleId)
	if err != nil {
		h.logger.Error("handler: GetVehicleDocuments failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	vehicleInfo := h.fetchVehicleInfo(ctx, req.VehicleId)
	var protoDocs []*filepb.VehicleDocumentResponse
	for _, doc := range docs {
		presignedURL, _ := h.storage.GeneratePresignedURL(ctx, doc.DocumentKey, defaultPresignTTL)
		protoDocs = append(protoDocs, toProtoVehicleDocument(doc, presignedURL, vehicleInfo))
	}
	return &filepb.GetVehicleDocumentsResponse{Documents: protoDocs}, nil
}

func (h *FileHandler) GetVehicleDocument(ctx context.Context, req *filepb.GetDocumentByIDRequest) (*filepb.VehicleDocumentResponse, error) {
	h.logger.Debug("handler: GetVehicleDocument", zap.String("documentID", req.DocumentId))
	doc, err := h.service.GetVehicleDocument(ctx, req.DocumentId)
	if err != nil {
		h.logHandlerError("handler: GetVehicleDocument", err)
		return nil, toGRPCError(err)
	}
	ttl := presignTTLFromRequest(req.PresignTTLSecs)
	presignedURL, _ := h.storage.GeneratePresignedURL(ctx, doc.DocumentKey, ttl)
	vehicleInfo := h.fetchVehicleInfo(ctx, doc.VehicleID)
	return toProtoVehicleDocument(doc, presignedURL, vehicleInfo), nil
}

func (h *FileHandler) GetVehicleDocumentsByUserID(ctx context.Context, req *filepb.GetVehicleDocumentsByUserIDRequest) (*filepb.GetVehicleDocumentsResponse, error) {
	h.logger.Debug("handler: GetVehicleDocumentsByUserID", zap.String("userID", req.UserId))
	docs, err := h.service.GetVehicleDocumentsByUserID(ctx, req.UserId)
	if err != nil {
		h.logger.Error("handler: GetVehicleDocumentsByUserID failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	// Dédupliquer les vehicleIDs pour n'appeler vehicle-service qu'une fois par véhicule.
	vehicleCache := make(map[string]*filepb.VehicleInfo)
	var protoDocs []*filepb.VehicleDocumentResponse
	for _, doc := range docs {
		if _, cached := vehicleCache[doc.VehicleID]; !cached {
			vehicleCache[doc.VehicleID] = h.fetchVehicleInfo(ctx, doc.VehicleID)
		}
		presignedURL, _ := h.storage.GeneratePresignedURL(ctx, doc.DocumentKey, defaultPresignTTL)
		protoDocs = append(protoDocs, toProtoVehicleDocument(doc, presignedURL, vehicleCache[doc.VehicleID]))
	}
	return &filepb.GetVehicleDocumentsResponse{Documents: protoDocs}, nil
}

func (h *FileHandler) ListKycDocuments(ctx context.Context, req *filepb.ListKycDocumentsRequest) (*filepb.ListKycDocumentsResponse, error) {
	h.logger.Debug("handler: ListKycDocuments", zap.Strings("statuses", req.Statuses))
	docs, err := h.service.ListKycDocuments(ctx, req.Statuses)
	if err != nil {
		h.logger.Error("handler: ListKycDocuments failed", zap.Error(err))
		return nil, toGRPCError(err)
	}
	out := make([]*filepb.KycDocument, 0, len(docs))
	for _, d := range docs {
		out = append(out, &filepb.KycDocument{
			DocumentId:   d.DocumentID,
			UserId:       d.UserID,
			VehicleId:    d.VehicleID,
			DocumentType: d.DocumentType,
			Status:       d.Status,
			OwnerKind:    d.OwnerKind,
			UpdatedAt:    d.UpdatedAt,
		})
	}
	return &filepb.ListKycDocumentsResponse{Documents: out}, nil
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
		UserID:               req.UserId,
		DocumentType:         req.DocumentType,
		UserDocumentID:       req.UserDocumentId,
		SecondUserDocumentID: req.SecondUserDocumentId,
		VehicleDocumentID:    req.VehicleDocumentId,

		PersonaInquiryID:    req.PersonaInquiryId,
		PersonaTemplateID:   req.PersonaTemplateId,
		PersonaSessionToken: req.PersonaSessionToken,
		SessionExpiresAt:    req.SessionExpiresAt,

		WebhookEventType:  req.WebhookEventType,
		WebhookReceivedAt: req.WebhookReceivedAt,
		PersonaRawPayload: req.PersonaRawPayload,

		AttemptNumber:    req.AttemptNumber,
		PreviousReviewID: req.PreviousReviewId,

		Status:           req.Status,
		Decision:         req.Decision,
		ReasonRejection:  req.ReasonRejection,
		RejectionDetails: req.RejectionDetails,

		ReviewedBy: req.ReviewedBy,
		ReviewType: req.ReviewType,

		Notes:         req.Notes,
		ExtractedData: req.ExtractedData,

		SubmittedAt: req.SubmittedAt,
	}

	review, err := h.service.CreateDocumentReview(ctx, input)
	if err != nil {
		h.logger.Error("handler: CreateDocumentReview failed", zap.Error(err))
		return nil, toGRPCError(err)
	}
	h.logger.Info("handler: CreateDocumentReview success", zap.String("reviewID", review.ReviewID))
	return toProtoDocumentReview(review), nil
}

func (h *FileHandler) GetDocumentReview(ctx context.Context, req *filepb.GetDocumentReviewByIDRequest) (*filepb.DocumentReviewResponse, error) {
	h.logger.Debug("handler: GetDocumentReview", zap.String("reviewID", req.ReviewId))

	review, err := h.service.GetDocumentReview(ctx, req.ReviewId)
	if err != nil {
		h.logHandlerError("handler: GetDocumentReview", err)
		return nil, toGRPCError(err)
	}
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

func (h *FileHandler) GetDocumentReviewByPersonaInquiryID(ctx context.Context, req *filepb.GetDocumentReviewByPersonaInquiryIDRequest) (*filepb.DocumentReviewResponse, error) {
	h.logger.Debug("handler: GetDocumentReviewByPersonaInquiryID", zap.String("personaInquiryID", req.PersonaInquiryId))

	review, err := h.service.GetDocumentReviewByPersonaInquiryID(ctx, req.PersonaInquiryId)
	if err != nil {
		h.logHandlerError("handler: GetDocumentReviewByPersonaInquiryID", err)
		return nil, toGRPCError(err)
	}
	return toProtoDocumentReview(review), nil
}

func (h *FileHandler) GetDocumentReviewsByUserID(ctx context.Context, req *filepb.GetDocumentReviewsByUserIDRequest) (*filepb.GetDocumentReviewsResponse, error) {
	h.logger.Debug("handler: GetDocumentReviewsByUserID", zap.String("userID", req.UserId))

	reviews, err := h.service.GetDocumentReviewsByUserID(ctx, req.UserId)
	if err != nil {
		h.logger.Error("handler: GetDocumentReviewsByUserID failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	var protoReviews []*filepb.DocumentReviewResponse
	for _, review := range reviews {
		protoReviews = append(protoReviews, toProtoDocumentReview(review))
	}
	return &filepb.GetDocumentReviewsResponse{Reviews: protoReviews}, nil
}

func (h *FileHandler) UpdateDocumentReview(ctx context.Context, req *filepb.UpdateDocumentReviewRequest) (*filepb.DocumentReviewResponse, error) {
	h.logger.Debug("handler: UpdateDocumentReview", zap.String("reviewID", req.ReviewId))

	// Récupérer la revue existante
	existing, err := h.service.GetDocumentReview(ctx, req.ReviewId)
	if err != nil {
		h.logger.Error("handler: UpdateDocumentReview — review not found", zap.Error(err))
		return nil, toGRPCError(err)
	}

	// Appliquer les champs non vides du request sur la revue existante
	if req.PersonaSessionToken != "" {
		existing.PersonaSessionToken = req.PersonaSessionToken
	}
	if req.SessionExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, req.SessionExpiresAt)
		if err == nil {
			existing.SessionExpiresAt = &t
		}
	}
	if req.WebhookEventType != "" {
		existing.WebhookEventType = req.WebhookEventType
	}
	if req.WebhookReceivedAt != "" {
		t, err := time.Parse(time.RFC3339, req.WebhookReceivedAt)
		if err == nil {
			existing.WebhookReceivedAt = &t
		}
	}
	if len(req.PersonaRawPayload) > 0 {
		existing.PersonaRawPayload = req.PersonaRawPayload
	}
	if req.Status != "" {
		existing.Status = req.Status
	}
	if req.Decision != "" {
		existing.Decision = req.Decision
	}
	if req.ReasonRejection != "" {
		existing.ReasonRejection = req.ReasonRejection
	}
	if req.RejectionDetails != "" {
		existing.RejectionDetails = req.RejectionDetails
	}
	if req.ReviewedBy != "" {
		existing.ReviewedBy = req.ReviewedBy
	}
	if req.ReviewType != "" {
		existing.ReviewType = req.ReviewType
	}
	if req.Notes != "" {
		existing.Notes = req.Notes
	}
	if req.SubmittedAt != "" {
		t, err := time.Parse(time.RFC3339, req.SubmittedAt)
		if err == nil {
			existing.SubmittedAt = &t
		}
	}
	existing.UpdatedAt = time.Now().UTC()

	updated, err := h.service.UpdateDocumentReview(ctx, existing)
	if err != nil {
		h.logger.Error("handler: UpdateDocumentReview failed", zap.Error(err))
		return nil, toGRPCError(err)
	}
	h.logger.Info("handler: UpdateDocumentReview success", zap.String("reviewID", updated.ReviewID))
	return toProtoDocumentReview(updated), nil
}

func (h *FileHandler) ListDocumentReviews(ctx context.Context, req *filepb.ListDocumentReviewsRequest) (*filepb.GetDocumentReviewsResponse, error) {
	h.logger.Debug("handler: ListDocumentReviews",
		zap.String("userID", req.UserId),
		zap.String("status", req.Status),
		zap.String("decision", req.Decision),
		zap.Int32("page", req.Page),
		zap.Int32("pageSize", req.PageSize),
	)

	reviews, err := h.service.ListDocumentReviews(ctx, req.UserId, req.Status, req.Decision, req.Page, req.PageSize)
	if err != nil {
		h.logger.Error("handler: ListDocumentReviews failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	var protoReviews []*filepb.DocumentReviewResponse
	for _, review := range reviews {
		protoReviews = append(protoReviews, toProtoDocumentReview(review))
	}
	return &filepb.GetDocumentReviewsResponse{Reviews: protoReviews}, nil
}

func (h *FileHandler) GetDocumentReviewHistory(ctx context.Context, req *filepb.GetDocumentReviewHistoryRequest) (*filepb.GetDocumentReviewHistoryResponse, error) {
	h.logger.Debug("handler: GetDocumentReviewHistory",
		zap.String("userID", req.UserId),
		zap.String("logicalDocumentType", req.LogicalDocumentType),
	)

	reviews, err := h.service.GetDocumentReviewHistory(ctx, req.UserId, req.LogicalDocumentType)
	if err != nil {
		h.logger.Error("handler: GetDocumentReviewHistory failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	protoReviews := make([]*filepb.DocumentReviewResponse, 0, len(reviews))
	for _, review := range reviews {
		protoReviews = append(protoReviews, toProtoDocumentReview(review))
	}
	return &filepb.GetDocumentReviewHistoryResponse{Reviews: protoReviews}, nil
}

// --- Suppression de compte ---

func (h *FileHandler) DeleteAllUserFiles(ctx context.Context, req *filepb.DeleteAllUserFilesRequest) (*filepb.DeleteAllUserFilesResponse, error) {
	if err := h.service.DeleteAllUserFiles(ctx, req.UserID); err != nil {
		h.logger.Error("handler: DeleteAllUserFiles failed", zap.Error(err))
		return &filepb.DeleteAllUserFilesResponse{Success: false, ErrorMessage: err.Error()}, toGRPCError(err)
	}
	return &filepb.DeleteAllUserFilesResponse{Success: true}, nil
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

func formatTimeOrEmpty(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func toProtoUploadedDocument(doc *serviceInterfaces.UploadedDocument) *filepb.UploadedDocument {
	if doc == nil {
		return nil
	}
	return &filepb.UploadedDocument{
		DocumentID:          doc.DocumentID,
		DocumentURL:         doc.DocumentURL,
		DocumentType:        doc.DocumentType,
		DocumentName:        doc.DocumentName,
		LogicalDocumentType: domain.ToLogicalDocumentType(doc.DocumentType),
	}
}

func toProtoUploadedDocuments(docs []*serviceInterfaces.UploadedDocument) []*filepb.UploadedDocument {
	out := make([]*filepb.UploadedDocument, 0, len(docs))
	for _, d := range docs {
		out = append(out, toProtoUploadedDocument(d))
	}
	return out
}

func toProtoUserDocument(doc *domain.UserDocument, presignedURL string) *filepb.UserDocumentResponse {
	return &filepb.UserDocumentResponse{
		DocumentId:          doc.DocumentID,
		UserId:              doc.UserID,
		DocumentName:        doc.DocumentName,
		DocumentType:        doc.DocumentType,
		LogicalDocumentType: domain.ToLogicalDocumentType(doc.DocumentType),
		DocumentUrl:         presignedURL,
		FileSizeBytes:       doc.FileSizeBytes,
		MimeType:            doc.MimeType,
		DocumentNumber:      doc.DocumentNumber,
		IssuingCountry:      doc.IssuingCountry,
		Status:              doc.Status,
		IsCurrent:           doc.IsCurrent,
		UploadedAt:          doc.UploadedAt.Format(time.RFC3339),
		UpdatedAt:           doc.UpdatedAt.Format(time.RFC3339),
		IssuedAt:            formatTimeOrEmpty(doc.IssuedAt),
		ExpiredAt:           formatTimeOrEmpty(doc.ExpireAt),
	}
}

func toProtoVehicleDocument(doc *domain.VehicleDocument, presignedURL string, vehicle *filepb.VehicleInfo) *filepb.VehicleDocumentResponse {
	return &filepb.VehicleDocumentResponse{
		DocumentId:       doc.DocumentID,
		VehicleId:        doc.VehicleID,
		UserId:           doc.UserID,
		DocumentName:     doc.DocumentName,
		DocumentType:     doc.DocumentType,
		DocumentUrl:      presignedURL,
		FileSizeBytes:    doc.FileSizeBytes,
		MimeType:         doc.MimeType,
		DocumentNumber:   doc.DocumentNumber,
		IssuingAuthority: doc.IssuingAuthority,
		Status:           doc.Status,
		IsCurrent:        doc.IsCurrent,
		UploadedAt:       doc.UploadedAt.Format(time.RFC3339),
		UpdatedAt:        doc.UpdatedAt.Format(time.RFC3339),
		IssuedAt:         formatTimeOrEmpty(doc.IssuedAt),
		ExpireAt:         formatTimeOrEmpty(doc.ExpireAt),
		Vehicle:          vehicle,
	}
}

// fetchVehicleInfo appelle vehicle-service et convertit le résultat en proto VehicleInfo.
// En cas d'erreur ou véhicule inconnu, retourne nil (dégradation gracieuse).
func (h *FileHandler) fetchVehicleInfo(ctx context.Context, vehicleID string) *filepb.VehicleInfo {
	if h.vehicleClient == nil || vehicleID == "" {
		return nil
	}
	info, err := h.vehicleClient.GetVehicleInfo(ctx, vehicleID)
	if err != nil || info == nil {
		return nil
	}
	return &filepb.VehicleInfo{
		VehicleId:     info.VehicleID,
		Brand:         info.Brand,
		BrandModel:    info.BrandModel,
		Color:         info.Color,
		LicencePlate:  info.LicencePlate,
		NumberOfSeats: info.NumberOfSeats,
		IsVerified:    info.IsVerified,
	}
}

// presignTTLFromRequest retourne le TTL à utiliser pour la présignature, plafonné à 24h.
func presignTTLFromRequest(secs int64) time.Duration {
	if secs <= 0 {
		return defaultPresignTTL
	}
	ttl := time.Duration(secs) * time.Second
	if ttl > maxPresignTTL {
		return maxPresignTTL
	}
	return ttl
}

func toProtoDocumentReview(review *domain.DocumentReview) *filepb.DocumentReviewResponse {
	resp := &filepb.DocumentReviewResponse{
		ReviewId:     review.ReviewID,
		UserId:       review.UserID,
		DocumentType: review.DocumentType,

		PersonaInquiryId:    review.PersonaInquiryID,
		PersonaTemplateId:   review.PersonaTemplateID,
		PersonaSessionToken: review.PersonaSessionToken,

		WebhookEventType:  review.WebhookEventType,
		PersonaRawPayload: review.PersonaRawPayload,

		AttemptNumber: int32(review.AttemptNumber),

		Status:           review.Status,
		Decision:         review.Decision,
		ReasonRejection:  review.ReasonRejection,
		RejectionDetails: review.RejectionDetails,

		ReviewedBy: review.ReviewedBy,
		ReviewType: review.ReviewType,
		ReviewedAt: review.ReviewedAt.Format(time.RFC3339),

		Notes:         review.Notes,
		ExtractedData: review.ExtractedData,

		UpdatedAt: review.UpdatedAt.Format(time.RFC3339),
	}
	if review.UserDocumentID != nil {
		resp.UserDocumentId = *review.UserDocumentID
	}
	if review.VehicleDocumentID != nil {
		resp.VehicleDocumentId = *review.VehicleDocumentID
	}
	if review.SessionExpiresAt != nil {
		resp.SessionExpiresAt = review.SessionExpiresAt.Format(time.RFC3339)
	}
	if review.WebhookReceivedAt != nil {
		resp.WebhookReceivedAt = review.WebhookReceivedAt.Format(time.RFC3339)
	}
	if review.PreviousReviewID != nil {
		resp.PreviousReviewId = *review.PreviousReviewID
	}
	if review.SubmittedAt != nil {
		resp.SubmittedAt = review.SubmittedAt.Format(time.RFC3339)
	}
	if review.SecondUserDocumentID != nil {
		resp.SecondUserDocumentId = *review.SecondUserDocumentID
	}
	resp.LogicalDocumentType = review.LogicalDocumentType
	return resp
}

// logHandlerError logs not-found errors at DEBUG (expected condition) and all
// other errors at ERROR (unexpected failures).
func (h *FileHandler) logHandlerError(handlerName string, err error) {
	if errors.Is(err, fileErrors.ErrorDocumentNotFound) || errors.Is(err, fileErrors.ErrorReviewNotFound) {
		h.logger.Debug(handlerName+" failed", zap.Error(err))
	} else {
		h.logger.Error(handlerName+" failed", zap.Error(err))
	}
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
		errors.Is(err, fileErrors.ErrorMissingUserID),
		errors.Is(err, fileErrors.ErrorMissingDocumentReference),
		errors.Is(err, fileErrors.ErrorMultipleDocumentReference),
		errors.Is(err, fileErrors.ErrorInvalidReviewStatus),
		errors.Is(err, fileErrors.ErrorInvalidReviewType),
		errors.Is(err, fileErrors.ErrorInvalidReasonRejection):
		return status.Error(codes.InvalidArgument, err.Error())

	case errors.Is(err, fileErrors.ErrorUnauthorized):
		return status.Error(codes.PermissionDenied, err.Error())

	case errors.Is(err, fileErrors.ErrorContentBlocked):
		return status.Error(codes.InvalidArgument, err.Error())

	case errors.Is(err, fileErrors.ErrorFileTooLarge):
		return status.Error(codes.ResourceExhausted, err.Error())

	case errors.Is(err, fileErrors.ErrorUploadFailed):
		return status.Error(codes.Unavailable, err.Error())

	case errors.Is(err, fileErrors.ErrorDocumentAlreadySubmitted):
		return status.Error(codes.AlreadyExists, err.Error())

	case errors.Is(err, fileErrors.ErrorDocumentNotReplaceable):
		return status.Error(codes.FailedPrecondition, err.Error())

	case errors.Is(err, fileErrors.ErrorMissingDocumentMetadata):
		return status.Error(codes.InvalidArgument, err.Error())

	default:
		return status.Error(codes.Internal, err.Error())
	}
}
