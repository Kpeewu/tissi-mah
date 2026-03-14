package service

import (
	"context"
	"time"

	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/cache"
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
	readRepo      repoInterfaces.TripRepositoryRead
	writeRepo     repoInterfaces.TripRepositoryWrite
	userClient    client.UserClient
	vehicleClient client.VehicleClient
	cache         *cache.TripCache
	logger        *zap.Logger
}

func NewTripService(
	readRepo repoInterfaces.TripRepositoryRead,
	writeRepo repoInterfaces.TripRepositoryWrite,
	userClient client.UserClient,
	vehicleClient client.VehicleClient,
	tripCache *cache.TripCache,
	logger *zap.Logger,
) serviceInterfaces.TripService {
	return &tripServiceImpl{
		readRepo:      readRepo,
		writeRepo:     writeRepo,
		userClient:    userClient,
		vehicleClient: vehicleClient,
		cache:         tripCache,
		logger:        logger,
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

	// Construction du domaine
	trip, waypoints := s.buildTripAndWaypoints(input, departure, estimatedArrival)

	// Persistance atomique (trip + waypoints)
	tripID, err := s.writeRepo.Create(ctx, trip, waypoints)
	if err != nil {
		s.logger.Error("create trip failed", zap.Error(err), zap.String("tripID", trip.TripID))
		return nil, err
	}

	// Invalidation du cache des previews pour ce conducteur
	if s.cache != nil {
		s.cache.InvalidateDriverPreviews(ctx, input.DriverID)
	}

	s.logger.Info("trip created", zap.String("tripID", tripID))
	trip.TripID = tripID
	return trip, nil
}

// ChangeTripDateAndTime modifie la date/heure de départ d'un trajet planifié.
func (s *tripServiceImpl) ChangeTripDateAndTime(ctx context.Context, input *serviceInterfaces.ChangeTripDateAndTimeInput) error {
	s.logger.Debug("service: ChangeTripDateAndTime called",
		zap.String("driverID", input.DriverID),
		zap.String("tripID", input.TripID),
	)

	if input.DriverID == "" || input.TripID == "" || input.DepartureDatetime == "" {
		return tripErrors.ErrorInvalidInput
	}

	newDatetime, err := time.Parse(time.RFC3339, input.DepartureDatetime)
	if err != nil {
		s.logger.Warn("invalid departure datetime", zap.String("value", input.DepartureDatetime))
		return tripErrors.ErrorInvalidDatetime
	}

	if err := s.writeRepo.UpdateDepartureDatetime(ctx, input.TripID, input.DriverID, newDatetime); err != nil {
		return err
	}

	// Invalidation du cache des previews pour ce conducteur
	if s.cache != nil {
		s.cache.InvalidateDriverPreviews(ctx, input.DriverID)
	}

	s.logger.Info("trip departure datetime updated",
		zap.String("tripID", input.TripID),
		zap.String("driverID", input.DriverID),
	)
	return nil
}

// ChangeTripVehicle modifie le véhicule associé à un trajet planifié.
func (s *tripServiceImpl) ChangeTripVehicle(ctx context.Context, input *serviceInterfaces.ChangeTripVehicleInput) error {
	s.logger.Debug("service: ChangeTripVehicle called",
		zap.String("driverID", input.DriverID),
		zap.String("tripID", input.TripID),
		zap.String("vehicleID", input.VehicleID),
	)

	if input.DriverID == "" || input.TripID == "" || input.VehicleID == "" {
		return tripErrors.ErrorInvalidInput
	}

	// Vérification : le véhicule existe et appartient au conducteur
	brand, _, vehicleSeats, err := s.vehicleClient.GetVehicleInfo(ctx, input.DriverID, input.VehicleID)
	if err != nil {
		s.logger.Error("vehicle-service check failed",
			zap.Error(err),
			zap.String("vehicleID", input.VehicleID),
		)
		return tripErrors.ErrorInternalServer
	}
	if brand == "" {
		s.logger.Warn("vehicle not found or does not belong to driver",
			zap.String("driverID", input.DriverID),
			zap.String("vehicleID", input.VehicleID),
		)
		return tripErrors.ErrorVehicleNotFound
	}

	// Vérification : le véhicule doit avoir au moins autant de places que le trajet
	tripTotalSeats, err := s.readRepo.GetTripTotalSeats(ctx, input.TripID)
	if err != nil {
		return err
	}
	if vehicleSeats < int(tripTotalSeats) {
		s.logger.Warn("vehicle has fewer seats than trip requires",
			zap.Int("vehicleSeats", vehicleSeats),
			zap.Int16("tripTotalSeats", tripTotalSeats),
		)
		return tripErrors.ErrorVehicleInsufficientSeats
	}

	if err := s.writeRepo.UpdateVehicle(ctx, input.TripID, input.DriverID, input.VehicleID); err != nil {
		return err
	}

	// Invalidation du cache des previews pour ce conducteur
	if s.cache != nil {
		s.cache.InvalidateDriverPreviews(ctx, input.DriverID)
	}

	s.logger.Info("trip vehicle updated",
		zap.String("tripID", input.TripID),
		zap.String("vehicleID", input.VehicleID),
	)
	return nil
}

// ChangeTripAllowances modifie les autorisations d'un trajet planifié.
func (s *tripServiceImpl) ChangeTripAllowances(ctx context.Context, input *serviceInterfaces.ChangeTripAllowancesInput) error {
	s.logger.Debug("service: ChangeTripAllowances called",
		zap.String("driverID", input.DriverID),
		zap.String("tripID", input.TripID),
	)

	if input.DriverID == "" || input.TripID == "" {
		return tripErrors.ErrorInvalidInput
	}

	if err := s.writeRepo.UpdateAllowances(ctx, input.TripID, input.DriverID,
		input.AllowPets, input.AllowFood, input.AllowSmoking, input.AllowLuggages,
	); err != nil {
		return err
	}

	s.logger.Info("trip allowances updated",
		zap.String("tripID", input.TripID),
		zap.String("driverID", input.DriverID),
	)
	return nil
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
