package grpc

import (
	"context"
	"errors"
	"time"

	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/trips-service/internal/service/interfaces"
	tripErrors "github.com/Kpeewu/tissi-mah/services/trips-service/pkg/errors"
	trippb "github.com/Kpeewu/tissi-mah/services/trips-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const serviceVersion = "1.0.0"

// TripHandler implémente trippb.TripServiceServer.
// Il traduit les requêtes proto en appels de service et mappe les erreurs domaine
// vers les codes gRPC appropriés.
type TripHandler struct {
	trippb.UnimplementedTripServiceServer
	service serviceInterfaces.TripService
	logger  *zap.Logger
}

func NewTripHandler(service serviceInterfaces.TripService, logger *zap.Logger) *TripHandler {
	return &TripHandler{service: service, logger: logger}
}

// CreateTrip crée un nouveau trajet avec ses waypoints.
func (h *TripHandler) CreateTrip(ctx context.Context, req *trippb.CreateTripRequest) (*trippb.CreateTripResponse, error) {
	h.logger.Debug("handler: CreateTrip called",
		zap.String("driverID", req.DriverId),
		zap.String("vehicleID", req.VehicleId),
		zap.Int("waypointCount", len(req.TripWaypoints)),
	)

	input := &serviceInterfaces.CreateTripInput{
		DriverID:                 req.DriverId,
		VehicleID:                req.VehicleId,
		DepartureDatetime:        req.DepartureDatetime,
		EstimatedArrivalDatetime: req.EstimatedArrivalDatetime,
		EstimatedDurationMinutes: int(req.EstimatedDurationMinutes),
		EstimatedDistanceMeters:  int(req.EstimatedDistanceMeters),
		TotalSeats:               int(req.TotalSeats),
		PricePerSeat:             int(req.PricePerSeat),
		PaymentMethodsAccepted:   req.PaymentMethodsAccepted,
		AllowLuggages:            req.AllowLuggages,
		AllowPets:                req.AllowPets,
		AllowSmoking:             req.AllowSmoking,
		AllowFood:                req.AllowFood,
		AutoApprove:              req.AutoApprove,
		Description:              req.Description,
		Waypoints:                toServiceWaypoints(req.TripWaypoints),
	}

	trip, err := h.service.CreateTrip(ctx, input)
	if err != nil {
		h.logger.Error("handler: CreateTrip failed", zap.Error(err))
		return &trippb.CreateTripResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}

	h.logger.Info("handler: CreateTrip success", zap.String("tripID", trip.TripID))
	return &trippb.CreateTripResponse{TripId: trip.TripID}, nil
}

// Health retourne l'état de santé du service.
func (h *TripHandler) Health(_ context.Context, _ *trippb.HealthRequest) (*trippb.HealthResponse, error) {
	return &trippb.HealthResponse{
		Status:    "SERVING",
		Version:   serviceVersion,
		Timestamp: time.Now().Unix(),
	}, nil
}

// toServiceWaypoints convertit les waypoints proto en input de service.
func toServiceWaypoints(pbWaypoints []*trippb.WaypointInput) []serviceInterfaces.WaypointInput {
	result := make([]serviceInterfaces.WaypointInput, 0, len(pbWaypoints))
	for _, wp := range pbWaypoints {
		result = append(result, serviceInterfaces.WaypointInput{
			SequencerOrder:    int16(wp.SequencerOrder),
			WaypointType:      wp.WaypointType,
			LocationName:      wp.LocationName,
			LocationLng:       wp.LocationLng,
			LocationLat:       wp.LocationLat,
			City:              wp.City,
			Country:           wp.Country,
			ScheduledDatetime: wp.ScheduledDatetime,
			PriceFromPrevious: int(wp.PriceFromPrevious),
		})
	}
	return result
}

// toGRPCError traduit les erreurs domaine en codes de statut gRPC.
func toGRPCError(err error) error {
	switch {
	case errors.Is(err, tripErrors.ErrorInvalidInput),
		errors.Is(err, tripErrors.ErrorInvalidDatetime),
		errors.Is(err, tripErrors.ErrorInvalidWaypoints):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, tripErrors.ErrorDriverNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, tripErrors.ErrorDriverNotVerified):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, tripErrors.ErrorTripOverlap):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, tripErrors.ErrorUnauthorized):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, tripErrors.ErrorTripNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, tripErrors.ErrorDataRetrievalFailed),
		errors.Is(err, tripErrors.ErrorInternalServer):
		return status.Error(codes.Internal, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
