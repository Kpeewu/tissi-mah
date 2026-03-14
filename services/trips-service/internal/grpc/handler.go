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

// CreateRecurringTrip programme un trajet récurrent.
func (h *TripHandler) CreateRecurringTrip(ctx context.Context, req *trippb.CreateRecurringTripRequest) (*trippb.CreateRecurringTripResponse, error) {
	h.logger.Debug("handler: CreateRecurringTrip called",
		zap.String("driverID", req.DriverId),
		zap.String("vehicleID", req.VehicleId),
		zap.String("recurrenceType", req.RecurrenceType),
	)

	var daysOfWeek []int32
	if req.DaysOfWeek != nil {
		daysOfWeek = req.DaysOfWeek.Days
	}

	input := &serviceInterfaces.CreateRecurringTripInput{
		DriverID:              req.DriverId,
		VehicleID:             req.VehicleId,
		DepartureTime:         req.DepartureTime,
		RecurrenceType:        req.RecurrenceType,
		DaysOfWeek:            daysOfWeek,
		StartDate:             req.StartDate,
		EndDate:               req.EndDate,
		TotalSeats:            int(req.TotalSeats),
		PricePerSeat:          int(req.PricePerSeat),
		AllowLuggages:         req.AllowLuggages,
		AllowPets:             req.AllowPets,
		AllowFood:             req.AllowFood,
		AllowSmoking:          req.AllowSmoking,
		AutoApprove:           req.AutoApprove,
		Description:           req.Description,
		GenerationHorizonDays: int(req.GenerationHorizonDays),
		Waypoints:             toServiceWaypoints(req.TripWaypoints),
	}

	_, err := h.service.CreateRecurringTrip(ctx, input)
	if err != nil {
		h.logger.Error("handler: CreateRecurringTrip failed", zap.Error(err))
		return &trippb.CreateRecurringTripResponse{Success: false, ErrorMessage: err.Error()}, toGRPCError(err)
	}

	h.logger.Info("handler: CreateRecurringTrip success")
	return &trippb.CreateRecurringTripResponse{Success: true}, nil
}

// GetTripsPreviews retourne la liste paginée des trajets du conducteur.
func (h *TripHandler) GetTripsPreviews(ctx context.Context, req *trippb.GetTripsPreviewsRequest) (*trippb.GetTripsPreviewsResponse, error) {
	h.logger.Debug("handler: GetTripsPreviews called",
		zap.String("driverID", req.DriverId),
		zap.Int32("index", req.Index),
	)

	results, err := h.service.GetTripsPreviews(ctx, &serviceInterfaces.GetTripsPreviewsInput{
		DriverID:  req.DriverId,
		PageIndex: int(req.Index),
	})
	if err != nil {
		h.logger.Error("handler: GetTripsPreviews failed", zap.Error(err))
		return &trippb.GetTripsPreviewsResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}

	pbPreviews := make([]*trippb.TripPreview, 0, len(results))
	for _, r := range results {
		pbPreviews = append(pbPreviews, &trippb.TripPreview{
			TripId:                r.TripID,
			DriverId:              r.DriverID,
			DriverName:            r.DriverName,
			VehicleId:             r.VehicleID,
			VehicleBrand:          r.VehicleBrand,
			VehiclePlate:          r.VehiclePlate,
			DepartureDate:         r.DepartureDatetime.Format("2006-01-02"),
			DepartureTime:         r.DepartureDatetime.Format("15:04"),
			TotalSeats:            int32(r.TotalSeats),
			AvailableSeats:        int32(r.AvailableSeats),
			DepartureLocationName: r.DepartureLocationName,
			ArrivalLocationName:   r.ArrivalLocationName,
		})
	}

	h.logger.Info("handler: GetTripsPreviews success",
		zap.String("driverID", req.DriverId),
		zap.Int("count", len(pbPreviews)),
	)
	return &trippb.GetTripsPreviewsResponse{TripsPreviews: pbPreviews}, nil
}

// GetCompletedTripsPreviews retourne la liste paginée des trajets complétés du conducteur.
func (h *TripHandler) GetCompletedTripsPreviews(ctx context.Context, req *trippb.GetCompletedTripsPreviewsRequest) (*trippb.GetCompletedTripsPreviewsResponse, error) {
	h.logger.Debug("handler: GetCompletedTripsPreviews called",
		zap.String("driverID", req.DriverId),
		zap.Int32("index", req.Index),
	)

	results, err := h.service.GetCompletedTripsPreviews(ctx, &serviceInterfaces.GetTripsPreviewsInput{
		DriverID:  req.DriverId,
		PageIndex: int(req.Index),
	})
	if err != nil {
		h.logger.Error("handler: GetCompletedTripsPreviews failed", zap.Error(err))
		return &trippb.GetCompletedTripsPreviewsResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}

	pbPreviews := make([]*trippb.CompletedTripPreview, 0, len(results))
	for _, r := range results {
		pbPreviews = append(pbPreviews, &trippb.CompletedTripPreview{
			TripId:                r.TripID,
			DriverId:              r.DriverID,
			DriverName:            r.DriverName,
			VehicleId:             r.VehicleID,
			VehicleBrand:          r.VehicleBrand,
			VehiclePlateNumber:    r.VehiclePlateNumber,
			DepartureDate:         r.DepartureDatetime.Format("2006-01-02"),
			DepartureTime:         r.DepartureDatetime.Format("15:04"),
			TotalSeats:            int32(r.TotalSeats),
			AvailableSeats:        int32(r.AvailableSeats),
			DepartureLocationName: r.DepartureLocationName,
			ArrivalLocationName:   r.ArrivalLocationName,
		})
	}

	h.logger.Info("handler: GetCompletedTripsPreviews success",
		zap.String("driverID", req.DriverId),
		zap.Int("count", len(pbPreviews)),
	)
	return &trippb.GetCompletedTripsPreviewsResponse{TripsPreviews: pbPreviews}, nil
}

// ChangeTripDateAndTime modifie la date/heure de départ d'un trajet planifié.
func (h *TripHandler) ChangeTripDateAndTime(ctx context.Context, req *trippb.ChangeTripDateAndTimeRequest) (*trippb.ChangeTripDateAndTimeResponse, error) {
	h.logger.Debug("handler: ChangeTripDateAndTime called",
		zap.String("driverID", req.DriverId),
		zap.String("tripID", req.TripId),
	)

	err := h.service.ChangeTripDateAndTime(ctx, &serviceInterfaces.ChangeTripDateAndTimeInput{
		DriverID:          req.DriverId,
		TripID:            req.TripId,
		DepartureDatetime: req.DepartureDatetime,
	})
	if err != nil {
		h.logger.Error("handler: ChangeTripDateAndTime failed", zap.Error(err))
		return &trippb.ChangeTripDateAndTimeResponse{Success: false, ErrorMessage: err.Error()}, toGRPCError(err)
	}

	h.logger.Info("handler: ChangeTripDateAndTime success", zap.String("tripID", req.TripId))
	return &trippb.ChangeTripDateAndTimeResponse{Success: true}, nil
}

// ChangeTripVehicle modifie le véhicule associé à un trajet planifié.
func (h *TripHandler) ChangeTripVehicle(ctx context.Context, req *trippb.ChangeTripVehicleRequest) (*trippb.ChangeTripVehicleResponse, error) {
	h.logger.Debug("handler: ChangeTripVehicle called",
		zap.String("driverID", req.DriverId),
		zap.String("tripID", req.TripId),
		zap.String("vehicleID", req.VehicleId),
	)

	err := h.service.ChangeTripVehicle(ctx, &serviceInterfaces.ChangeTripVehicleInput{
		DriverID:  req.DriverId,
		TripID:    req.TripId,
		VehicleID: req.VehicleId,
	})
	if err != nil {
		h.logger.Error("handler: ChangeTripVehicle failed", zap.Error(err))
		return &trippb.ChangeTripVehicleResponse{Success: false, ErrorMessage: err.Error()}, toGRPCError(err)
	}

	h.logger.Info("handler: ChangeTripVehicle success", zap.String("tripID", req.TripId))
	return &trippb.ChangeTripVehicleResponse{Success: true}, nil
}

// ChangeTripAllowances modifie les autorisations d'un trajet planifié.
func (h *TripHandler) ChangeTripAllowances(ctx context.Context, req *trippb.ChangeTripAllowancesRequest) (*trippb.ChangeTripAllowancesResponse, error) {
	h.logger.Debug("handler: ChangeTripAllowances called",
		zap.String("driverID", req.DriverId),
		zap.String("tripID", req.TripId),
	)

	err := h.service.ChangeTripAllowances(ctx, &serviceInterfaces.ChangeTripAllowancesInput{
		DriverID:      req.DriverId,
		TripID:        req.TripId,
		AllowPets:     req.AllowPets,
		AllowFood:     req.AllowFood,
		AllowSmoking:  req.AllowSmoking,
		AllowLuggages: req.AllowLuggage,
	})
	if err != nil {
		h.logger.Error("handler: ChangeTripAllowances failed", zap.Error(err))
		return &trippb.ChangeTripAllowancesResponse{Success: false, ErrorMessage: err.Error()}, toGRPCError(err)
	}

	h.logger.Info("handler: ChangeTripAllowances success", zap.String("tripID", req.TripId))
	return &trippb.ChangeTripAllowancesResponse{Success: true}, nil
}

// ChangeAutoApprove active ou désactive l'approbation automatique d'un trajet.
func (h *TripHandler) ChangeAutoApprove(ctx context.Context, req *trippb.ChangeAutoApproveRequest) (*trippb.ChangeAutoApproveResponse, error) {
	h.logger.Debug("handler: ChangeAutoApprove called",
		zap.String("driverID", req.DriverId),
		zap.String("tripID", req.TripId),
		zap.Bool("autoApprove", req.AutoApprove),
	)

	err := h.service.ChangeAutoApprove(ctx, &serviceInterfaces.ChangeAutoApproveInput{
		DriverID:    req.DriverId,
		TripID:      req.TripId,
		AutoApprove: req.AutoApprove,
	})
	if err != nil {
		h.logger.Error("handler: ChangeAutoApprove failed", zap.Error(err))
		return &trippb.ChangeAutoApproveResponse{Success: false, ErrorMessage: err.Error()}, toGRPCError(err)
	}

	h.logger.Info("handler: ChangeAutoApprove success", zap.String("tripID", req.TripId))
	return &trippb.ChangeAutoApproveResponse{Success: true}, nil
}

// StartTrip démarre un trajet planifié.
func (h *TripHandler) StartTrip(ctx context.Context, req *trippb.StartTripRequest) (*trippb.StartTripResponse, error) {
	h.logger.Debug("handler: StartTrip called",
		zap.String("driverID", req.DriverId),
		zap.String("tripID", req.TripId),
	)

	err := h.service.StartTrip(ctx, &serviceInterfaces.StartTripInput{
		DriverID: req.DriverId,
		TripID:   req.TripId,
	})
	if err != nil {
		h.logger.Error("handler: StartTrip failed", zap.Error(err))
		return &trippb.StartTripResponse{Success: false, ErrorMessage: err.Error()}, toGRPCError(err)
	}

	h.logger.Info("handler: StartTrip success", zap.String("tripID", req.TripId))
	return &trippb.StartTripResponse{Success: true}, nil
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
	case errors.Is(err, tripErrors.ErrorUnauthorized):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, tripErrors.ErrorTripNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, tripErrors.ErrorTripNotScheduled):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, tripErrors.ErrorVehicleNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, tripErrors.ErrorVehicleInsufficientSeats):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, tripErrors.ErrorTripDepartureTooSoon):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, tripErrors.ErrorDriverAlreadyHasActiveTrip):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, tripErrors.ErrorDataRetrievalFailed),
		errors.Is(err, tripErrors.ErrorInternalServer):
		return status.Error(codes.Internal, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
