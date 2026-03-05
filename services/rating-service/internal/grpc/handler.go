package grpc

import (
	"context"
	"errors"
	"time"

	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/domain"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/rating-service/internal/service/interfaces"
	ratingErrors "github.com/Kpeewu/tissi-mah/services/rating-service/pkg/errors"
	ratingpb "github.com/Kpeewu/tissi-mah/services/rating-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const serviceVersion = "1.0.0"

// RatingHandler implémente ratingpb.RatingServiceServer.
// Il traduit les requêtes proto en appels de service et mappe les erreurs domaine
// vers les codes gRPC appropriés.
type RatingHandler struct {
	ratingpb.UnimplementedRatingServiceServer
	service serviceInterfaces.RatingService
	logger  *zap.Logger
}

func NewRatingHandler(service serviceInterfaces.RatingService, logger *zap.Logger) *RatingHandler {
	return &RatingHandler{service: service, logger: logger}
}

// CreateRating crée une nouvelle note pour un utilisateur.
func (h *RatingHandler) CreateRating(ctx context.Context, req *ratingpb.CreateRatingRequest) (*ratingpb.CreateRatingResponse, error) {
	h.logger.Debug("handler: CreateRating called",
		zap.String("userRatedID", req.UserRatedId),
		zap.Int32("stars", req.NumberOfStars),
	)

	rating, err := h.service.CreateRating(ctx, req.RaterId, req.UserRatedId, int16(req.NumberOfStars), req.Comment)
	if err != nil {
		h.logger.Error("handler: CreateRating failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	h.logger.Info("handler: CreateRating success", zap.String("ratingID", rating.RatingID))
	return &ratingpb.CreateRatingResponse{Rating: toProtoRatingDetail(rating)}, nil
}

// GetRating récupère une note par son ID.
func (h *RatingHandler) GetRating(ctx context.Context, req *ratingpb.GetRatingRequest) (*ratingpb.GetRatingResponse, error) {
	h.logger.Debug("handler: GetRating called", zap.String("ratingID", req.RatingId))

	rating, err := h.service.GetRating(ctx, req.RatingId)
	if err != nil {
		h.logger.Error("handler: GetRating failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	return &ratingpb.GetRatingResponse{Rating: toProtoRatingDetail(rating)}, nil
}

// GetRatingsForUser récupère toutes les notes reçues par un utilisateur.
func (h *RatingHandler) GetRatingsForUser(ctx context.Context, req *ratingpb.GetRatingsForUserRequest) (*ratingpb.GetRatingsForUserResponse, error) {
	h.logger.Debug("handler: GetRatingsForUser called", zap.String("userRatedID", req.UserRatedId))

	ratings, err := h.service.GetRatingsForUser(ctx, req.UserRatedId)
	if err != nil {
		h.logger.Error("handler: GetRatingsForUser failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	protoRatings := make([]*ratingpb.RatingDetail, 0, len(ratings))
	for _, r := range ratings {
		protoRatings = append(protoRatings, toProtoRatingDetail(r))
	}

	return &ratingpb.GetRatingsForUserResponse{Ratings: protoRatings}, nil
}

// GetAverageRating récupère la moyenne des notes d'un utilisateur.
func (h *RatingHandler) GetAverageRating(ctx context.Context, req *ratingpb.GetAverageRatingRequest) (*ratingpb.GetAverageRatingResponse, error) {
	h.logger.Debug("handler: GetAverageRating called", zap.String("userRatedID", req.UserRatedId))

	average, total, err := h.service.GetAverageRating(ctx, req.UserRatedId)
	if err != nil {
		h.logger.Error("handler: GetAverageRating failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	return &ratingpb.GetAverageRatingResponse{
		Average:      average,
		TotalRatings: total,
	}, nil
}

// UpdateRating modifie une note existante (seul le rater peut modifier).
func (h *RatingHandler) UpdateRating(ctx context.Context, req *ratingpb.UpdateRatingRequest) (*ratingpb.UpdateRatingResponse, error) {
	h.logger.Debug("handler: UpdateRating called", zap.String("ratingID", req.RatingId))

	rating, err := h.service.UpdateRating(ctx, req.RaterId, req.RatingId, int16(req.NumberOfStars), req.Comment)
	if err != nil {
		h.logger.Error("handler: UpdateRating failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	h.logger.Info("handler: UpdateRating success", zap.String("ratingID", rating.RatingID))
	return &ratingpb.UpdateRatingResponse{Rating: toProtoRatingDetail(rating)}, nil
}

// DeleteRating supprime une note (seul le rater peut supprimer).
func (h *RatingHandler) DeleteRating(ctx context.Context, req *ratingpb.DeleteRatingRequest) (*ratingpb.RatingServerResponse, error) {
	h.logger.Debug("handler: DeleteRating called", zap.String("ratingID", req.RatingId))

	if err := h.service.DeleteRating(ctx, req.RaterId, req.RatingId); err != nil {
		h.logger.Error("handler: DeleteRating failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	h.logger.Info("handler: DeleteRating success", zap.String("ratingID", req.RatingId))
	return &ratingpb.RatingServerResponse{Success: true}, nil
}

// Health retourne l'état de santé du service (route publique, sans auth).
func (h *RatingHandler) Health(_ context.Context, _ *ratingpb.HealthRequest) (*ratingpb.HealthResponse, error) {
	return &ratingpb.HealthResponse{
		Status:    "healthy",
		Version:   serviceVersion,
		Timestamp: time.Now().Unix(),
	}, nil
}

// toGRPCError traduit les erreurs domaine en codes de statut gRPC.
func toGRPCError(err error) error {
	switch {
	case errors.Is(err, ratingErrors.ErrorRatingNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, ratingErrors.ErrorRatingAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, ratingErrors.ErrorInvalidStars):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, ratingErrors.ErrorSelfRating):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, ratingErrors.ErrorMissingRaterID):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, ratingErrors.ErrorUserNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, ratingErrors.ErrorUnauthorizedAction):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, ratingErrors.ErrorCantDeleteRating):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, ratingErrors.ErrorDataRetrievalFailed),
		errors.Is(err, ratingErrors.ErrorInternalServer):
		return status.Error(codes.Internal, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}

// toProtoRatingDetail convertit domain.Rating en message proto RatingDetail.
func toProtoRatingDetail(r *domain.Rating) *ratingpb.RatingDetail {
	detail := &ratingpb.RatingDetail{
		RatingId:      r.RatingID,
		RaterId:       r.RaterID,
		UserRatedId:   r.UserRatedID,
		NumberOfStars: int32(r.NumberOfStars),
		CreatedAt:     r.CreatedAt.Unix(),
		UpdatedAt:     r.UpdatedAt.Unix(),
	}
	if r.Comment != nil {
		detail.Comment = *r.Comment
	}
	return detail
}
