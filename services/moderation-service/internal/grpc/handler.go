package grpc

import (
	"context"
	"errors"
	"time"

	"github.com/Kpeewu/tissi-mah/services/moderation-service/internal/domain"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/moderation-service/internal/service/interfaces"
	modErrors "github.com/Kpeewu/tissi-mah/services/moderation-service/pkg/errors"
	moderationpb "github.com/Kpeewu/tissi-mah/services/moderation-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const serviceVersion = "1.0.0"

type ModerationHandler struct {
	moderationpb.UnimplementedModerationServiceServer
	service serviceInterfaces.ModerationService
	logger  *zap.Logger
}

func NewModerationHandler(svc serviceInterfaces.ModerationService, logger *zap.Logger) *ModerationHandler {
	return &ModerationHandler{service: svc, logger: logger}
}

func (h *ModerationHandler) ModerateText(ctx context.Context, req *moderationpb.ModerateTextRequest) (*moderationpb.ModerateTextResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if req.Text == "" {
		return nil, status.Error(codes.InvalidArgument, "text is required")
	}

	result, err := h.service.ModerateText(ctx, req.ContentId, req.ContentType, req.Text, req.AuthorId)
	if err != nil {
		h.logger.Error("ModerateText failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	return &moderationpb.ModerateTextResponse{
		Decision:     toProtoDec(result.Decision),
		Category:     toProtoCat(result.Category),
		Score:        result.Score,
		Reason:       result.Reason,
		UsedFallback: result.UsedFallback,
	}, nil
}

func (h *ModerationHandler) ModerateImage(ctx context.Context, req *moderationpb.ModerateImageRequest) (*moderationpb.ModerateImageResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if len(req.ImageBytes) == 0 {
		return nil, status.Error(codes.InvalidArgument, "image_bytes is required")
	}

	result, err := h.service.ModerateImage(ctx, req.ContentId, req.ContentType, req.ImageBytes, req.MimeType, req.AuthorId)
	if err != nil {
		h.logger.Error("ModerateImage failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	return &moderationpb.ModerateImageResponse{
		Decision:     toProtoDec(result.Decision),
		Category:     toProtoCat(result.Category),
		Score:        result.Score,
		Reason:       result.Reason,
		UsedFallback: result.UsedFallback,
	}, nil
}

func (h *ModerationHandler) Health(_ context.Context, _ *moderationpb.HealthRequest) (*moderationpb.HealthResponse, error) {
	return &moderationpb.HealthResponse{
		Status:    "healthy",
		Version:   serviceVersion,
		Timestamp: time.Now().Unix(),
	}, nil
}

func toGRPCError(err error) error {
	switch {
	case errors.Is(err, modErrors.ErrorContentBlocked), errors.Is(err, modErrors.ErrorInvalidInput),
		errors.Is(err, modErrors.ErrorUnsupportedMediaType), errors.Is(err, modErrors.ErrorInvalidImageFormat),
		errors.Is(err, modErrors.ErrorFileTooLarge):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, modErrors.ErrorAccountSuspended), errors.Is(err, modErrors.ErrorAccountBanned):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, modErrors.ErrorAuthServiceUnavailable),
		errors.Is(err, modErrors.ErrorUserServiceUnavailable),
		errors.Is(err, modErrors.ErrorModerationAPIFailed):
		return status.Error(codes.Unavailable, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}

func toProtoDec(d domain.Decision) moderationpb.ModerationDecision {
	switch d {
	case domain.DecisionFlagged:
		return moderationpb.ModerationDecision_FLAGGED
	case domain.DecisionBlocked:
		return moderationpb.ModerationDecision_BLOCKED
	default:
		return moderationpb.ModerationDecision_APPROVED
	}
}

func toProtoCat(c domain.Category) moderationpb.ContentCategory {
	switch c {
	case domain.CategoryHateSpeech:
		return moderationpb.ContentCategory_HATE_SPEECH
	case domain.CategoryObscene:
		return moderationpb.ContentCategory_OBSCENE
	case domain.CategoryInsults:
		return moderationpb.ContentCategory_INSULTS
	case domain.CategorySpam:
		return moderationpb.ContentCategory_SPAM
	case domain.CategoryNSFW:
		return moderationpb.ContentCategory_NSFW
	case domain.CategoryGore:
		return moderationpb.ContentCategory_GORE
	case domain.CategoryCSAM:
		return moderationpb.ContentCategory_CSAM
	default:
		return moderationpb.ContentCategory_NONE
	}
}
