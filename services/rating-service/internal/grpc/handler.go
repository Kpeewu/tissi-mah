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
type RatingHandler struct {
	ratingpb.UnimplementedRatingServiceServer
	service serviceInterfaces.RatingService
	logger  *zap.Logger
}

func NewRatingHandler(service serviceInterfaces.RatingService, logger *zap.Logger) *RatingHandler {
	return &RatingHandler{service: service, logger: logger}
}

// RateUser crée une nouvelle note pour un utilisateur.
func (h *RatingHandler) RateUser(ctx context.Context, req *ratingpb.RateUserRequest) (*ratingpb.RateUserResponse, error) {
	h.logger.Debug("handler: RateUser called",
		zap.String("raterID", req.RaterId),
		zap.String("userRatedID", req.UserRatedId),
		zap.Int32("stars", req.NumberOfStars),
	)

	rating, err := h.service.RateUser(ctx, req.RaterId, req.UserRatedId, int16(req.NumberOfStars), req.Comment)
	if err != nil {
		h.logger.Error("handler: RateUser failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	h.logger.Info("handler: RateUser success", zap.String("ratingID", rating.RatingID))
	return &ratingpb.RateUserResponse{Rating: toProtoRatingDetail(rating)}, nil
}

// GetUserRatings récupère toutes les notes reçues par un utilisateur.
func (h *RatingHandler) GetUserRatings(ctx context.Context, req *ratingpb.GetUserRatingsRequest) (*ratingpb.GetUserRatingsResponse, error) {
	h.logger.Debug("handler: GetUserRatings called", zap.String("userRatedID", req.UserRatedId))

	ratings, err := h.service.GetUserRatings(ctx, req.UserRatedId)
	if err != nil {
		h.logger.Error("handler: GetUserRatings failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	protoRatings := make([]*ratingpb.RatingDetail, 0, len(ratings))
	for _, r := range ratings {
		protoRatings = append(protoRatings, toProtoRatingDetail(r))
	}

	return &ratingpb.GetUserRatingsResponse{Ratings: protoRatings}, nil
}

// GetUserRatingsAverage récupère la moyenne des notes d'un utilisateur.
func (h *RatingHandler) GetUserRatingsAverage(ctx context.Context, req *ratingpb.GetUserRatingsAverageRequest) (*ratingpb.GetUserRatingsAverageResponse, error) {
	h.logger.Debug("handler: GetUserRatingsAverage called", zap.String("userRatedID", req.UserRatedId))

	average, total, err := h.service.GetUserRatingsAverage(ctx, req.UserRatedId)
	if err != nil {
		h.logger.Error("handler: GetUserRatingsAverage failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	return &ratingpb.GetUserRatingsAverageResponse{
		Average:      average,
		TotalRatings: total,
	}, nil
}

// UpdateRating modifie une note existante (seul le rater peut modifier).
func (h *RatingHandler) UpdateRating(ctx context.Context, req *ratingpb.UpdateRatingRequest) (*ratingpb.UpdateRatingResponse, error) {
	h.logger.Debug("handler: UpdateRating called", zap.String("ratingID", req.RatingId))

	rating, err := h.service.UpdateRating(ctx, req.RaterId, req.RatingId, req.UserRatedId, int16(req.NumberOfStars), req.Comment)
	if err != nil {
		h.logger.Error("handler: UpdateRating failed", zap.Error(err))
		return nil, toGRPCError(err)
	}

	h.logger.Info("handler: UpdateRating success", zap.String("ratingID", rating.RatingID))
	return &ratingpb.UpdateRatingResponse{Rating: toProtoRatingDetail(rating)}, nil
}

// Health retourne l'état de santé du service (route publique, sans auth).
func (h *RatingHandler) Health(_ context.Context, _ *ratingpb.HealthRequest) (*ratingpb.HealthResponse, error) {
	return &ratingpb.HealthResponse{
		Status:    "SERVING",
		Version:   serviceVersion,
		Timestamp: time.Now().Unix(),
	}, nil
}

// toGRPCError traduit les erreurs domaine en codes de statut gRPC.
func toGRPCError(err error) error {
	switch {
	case errors.Is(err, ratingErrors.ErrorRatingNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, ratingErrors.ErrorUserNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, ratingErrors.ErrorRatingAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, ratingErrors.ErrorInvalidStars):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, ratingErrors.ErrorSelfRating):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, ratingErrors.ErrorMissingRaterID):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, ratingErrors.ErrorMissingUserRatedID):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, ratingErrors.ErrorUnauthorizedAction):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, ratingErrors.ErrorDataRetrievalFailed),
		errors.Is(err, ratingErrors.ErrorInternalServer):
		return status.Error(codes.Internal, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}

// toProtoRatingDetail convertit domain.Rating en message proto RatingDetail.
// Les timestamps sont formatés en ISO 8601 (TIMESTAMPTZ).
func toProtoRatingDetail(r *domain.Rating) *ratingpb.RatingDetail {
	detail := &ratingpb.RatingDetail{
		RatingId:      r.RatingID,
		RaterId:       r.RaterID,
		UserRatedId:   r.UserRatedID,
		NumberOfStars: int32(r.NumberOfStars),
		CreatedAt:     r.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     r.UpdatedAt.Format(time.RFC3339),
	}
	if r.Comment != nil {
		detail.Comment = *r.Comment
	}
	return detail
}
