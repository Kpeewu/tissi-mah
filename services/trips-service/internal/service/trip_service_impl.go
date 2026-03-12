package service

import (
	"context"
	"time"

	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/domain"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/trips-service/internal/repository/interfaces"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/trips-service/internal/service/interfaces"
	tripErrors "github.com/Kpeewu/tissi-mah/services/trips-service/pkg/errors"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const minWaypoints = 2

type tripServiceImpl struct {
	readRepo   repoInterfaces.TripRepositoryRead
	writeRepo  repoInterfaces.TripRepositoryWrite
	userClient client.UserClient
	logger     *zap.Logger
}

func NewTripService(
	readRepo repoInterfaces.TripRepositoryRead,
	writeRepo repoInterfaces.TripRepositoryWrite,
	userClient client.UserClient,
	logger *zap.Logger,
) serviceInterfaces.TripService {
	return &tripServiceImpl{
		readRepo:   readRepo,
		writeRepo:  writeRepo,
		userClient: userClient,
		logger:     logger,
	}
}

// CreateTrip crée un nouveau trajet avec ses waypoints.
func (s *tripServiceImpl) CreateTrip(ctx context.Context, input *serviceInterfaces.CreateTripInput) (*domain.Trip, error) {
	s.logger.Debug("service: CreateTrip called",
		zap.String("driverID", input.DriverID),
		zap.String("vehicleID", input.VehicleID),
	)

	// Validation des champs obligatoires
	if err := s.validateInput(input); err != nil {
		return nil, err
	}

	// Parse des datetimes
	departure, estimatedArrival, err := s.parseDatetimes(input.DepartureDatetime, input.EstimatedArrivalDatetime)
	if err != nil {
		return nil, err
	}

	// Vérification : conducteur certifié
	if err := s.validateVerifiedDriver(ctx, input.DriverID); err != nil {
		return nil, err
	}

	// Vérification : pas de chevauchement (intervalle minimum 2h)
	overlaps, err := s.readRepo.HasOverlappingTrip(ctx, input.DriverID, departure, estimatedArrival)
	if err != nil {
		s.logger.Error("overlap check failed", zap.Error(err), zap.String("driverID", input.DriverID))
		return nil, tripErrors.ErrorInternalServer
	}
	if overlaps {
		s.logger.Warn("trip overlap detected",
			zap.String("driverID", input.DriverID),
			zap.Time("departure", departure),
		)
		return nil, tripErrors.ErrorTripOverlap
	}

	// Construction du domaine
	trip, waypoints := s.buildTripAndWaypoints(input, departure, estimatedArrival)

	// Persistance atomique (trip + waypoints)
	tripID, err := s.writeRepo.Create(ctx, trip, waypoints)
	if err != nil {
		s.logger.Error("create trip failed", zap.Error(err), zap.String("tripID", trip.TripID))
		return nil, err
	}

	s.logger.Info("trip created", zap.String("tripID", tripID))
	trip.TripID = tripID
	return trip, nil
}

// validateInput vérifie les champs obligatoires et la cohérence des waypoints.
func (s *tripServiceImpl) validateInput(input *serviceInterfaces.CreateTripInput) error {
	if input.DriverID == "" || input.VehicleID == "" {
		return tripErrors.ErrorInvalidInput
	}
	if input.DepartureDatetime == "" || input.EstimatedArrivalDatetime == "" {
		return tripErrors.ErrorInvalidInput
	}
	if input.TotalSeats <= 0 {
		return tripErrors.ErrorInvalidInput
	}
	if input.EstimatedDurationMinutes <= 0 || input.EstimatedDistanceMeters <= 0 {
		return tripErrors.ErrorInvalidInput
	}
	if len(input.Waypoints) < minWaypoints {
		s.logger.Warn("not enough waypoints", zap.Int("count", len(input.Waypoints)))
		return tripErrors.ErrorInvalidWaypoints
	}

	// Vérification : exactement 1 departure et 1 arrival
	var departures, arrivals int
	for _, wp := range input.Waypoints {
		switch domain.WaypointType(wp.WaypointType) {
		case domain.WaypointTypeDeparture:
			departures++
		case domain.WaypointTypeArrival:
			arrivals++
		case domain.WaypointTypeStop:
			// ok
		default:
			return tripErrors.ErrorInvalidWaypoints
		}
	}
	if departures != 1 || arrivals != 1 {
		s.logger.Warn("invalid waypoint types",
			zap.Int("departures", departures),
			zap.Int("arrivals", arrivals),
		)
		return tripErrors.ErrorInvalidWaypoints
	}

	return nil
}

// parseDatetimes parse les datetimes RFC3339 et vérifie leur cohérence.
func (s *tripServiceImpl) parseDatetimes(departureStr, arrivalStr string) (time.Time, time.Time, error) {
	departure, err := time.Parse(time.RFC3339, departureStr)
	if err != nil {
		s.logger.Warn("invalid departure datetime", zap.String("value", departureStr))
		return time.Time{}, time.Time{}, tripErrors.ErrorInvalidDatetime
	}

	estimatedArrival, err := time.Parse(time.RFC3339, arrivalStr)
	if err != nil {
		s.logger.Warn("invalid estimated arrival datetime", zap.String("value", arrivalStr))
		return time.Time{}, time.Time{}, tripErrors.ErrorInvalidDatetime
	}

	if !estimatedArrival.After(departure) {
		s.logger.Warn("arrival must be after departure",
			zap.Time("departure", departure),
			zap.Time("arrival", estimatedArrival),
		)
		return time.Time{}, time.Time{}, tripErrors.ErrorInvalidDatetime
	}

	return departure, estimatedArrival, nil
}

// validateVerifiedDriver vérifie que le conducteur a un profil conducteur certifié.
func (s *tripServiceImpl) validateVerifiedDriver(ctx context.Context, driverID string) error {
	verified, err := s.userClient.IsVerifiedDriver(ctx, driverID)
	if err != nil {
		s.logger.Error("user-service check failed",
			zap.Error(err),
			zap.String("driverID", driverID),
		)
		return tripErrors.ErrorInternalServer
	}
	if !verified {
		s.logger.Warn("driver not verified or not found", zap.String("driverID", driverID))
		// On distingue "non trouvé" de "non vérifié" côté client via IsVerifiedDriver
		// Pour simplifier, on retourne ErrorDriverNotVerified dans les deux cas
		return tripErrors.ErrorDriverNotVerified
	}
	return nil
}

// buildTripAndWaypoints construit les structs domaine à partir de l'input.
func (s *tripServiceImpl) buildTripAndWaypoints(
	input *serviceInterfaces.CreateTripInput,
	departure time.Time,
	estimatedArrival time.Time,
) (*domain.Trip, []*domain.Waypoint) {
	trip := &domain.Trip{
		TripID:                   uuid.New().String(),
		DriverID:                 input.DriverID,
		VehicleID:                input.VehicleID,
		DepartureDatetime:        departure,
		EstimatedArrivalDatetime: estimatedArrival,
		EstimatedDurationMinutes: input.EstimatedDurationMinutes,
		EstimatedDistanceMeters:  input.EstimatedDistanceMeters,
		TotalSeats:               int16(input.TotalSeats),
		AvailableSeats:           int16(input.TotalSeats),
		PricePerSeat:             input.PricePerSeat,
		PaymentMethodsAccepted:   input.PaymentMethodsAccepted,
		AllowLuggages:            input.AllowLuggages,
		AllowPets:                input.AllowPets,
		AllowFood:                input.AllowFood,
		AllowSmoking:             input.AllowSmoking,
		Status:                   domain.TripStatusScheduled,
		AutoApproveEnabled:       input.AutoApprove,
		Description:              input.Description,
	}

	waypoints := make([]*domain.Waypoint, 0, len(input.Waypoints))
	for _, wpInput := range input.Waypoints {
		wp := &domain.Waypoint{
			WaypointID:           uuid.New().String(),
			TripID:               trip.TripID,
			SequencerOrder:       wpInput.SequencerOrder,
			WaypointType:         domain.WaypointType(wpInput.WaypointType),
			LocationName:         wpInput.LocationName,
			LocationLng:          wpInput.LocationLng,
			LocationLat:          wpInput.LocationLat,
			City:                 wpInput.City,
			Country:              wpInput.Country,
			PriceFromPrevious:    wpInput.PriceFromPrevious,
			MinutesFromDeparture: s.computeMinutesFromDeparture(wpInput.ScheduledDatetime, departure),
		}

		// Parse du scheduled_datetime optionnel
		if wpInput.ScheduledDatetime != "" {
			if t, err := time.Parse(time.RFC3339, wpInput.ScheduledDatetime); err == nil {
				wp.ScheduledPickupDatetime = &t
			}
		}

		waypoints = append(waypoints, wp)
	}

	return trip, waypoints
}

// computeMinutesFromDeparture calcule le décalage en minutes depuis le départ.
func (s *tripServiceImpl) computeMinutesFromDeparture(scheduledStr string, departure time.Time) int {
	if scheduledStr == "" {
		return 0
	}
	t, err := time.Parse(time.RFC3339, scheduledStr)
	if err != nil {
		return 0
	}
	diff := t.Sub(departure)
	if diff < 0 {
		return 0
	}
	return int(diff.Minutes())
}
