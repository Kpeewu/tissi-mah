package service

import (
	"context"
	"time"

	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/domain"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/trips-service/internal/service/interfaces"
	tripErrors "github.com/Kpeewu/tissi-mah/services/trips-service/pkg/errors"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// CreateRecurringTrip crée un pattern récurrent et génère les instances de trajet.
func (s *tripServiceImpl) CreateRecurringTrip(ctx context.Context, input *serviceInterfaces.CreateRecurringTripInput) (string, error) {
	s.logger.Debug("service: CreateRecurringTrip called",
		zap.String("driverID", input.DriverID),
		zap.String("vehicleID", input.VehicleID),
		zap.String("recurrenceType", input.RecurrenceType),
	)

	// Validation des champs obligatoires
	if err := s.validateRecurringInput(input); err != nil {
		return "", err
	}

	// Parse de l'heure de départ (seule la partie heure est utilisée)
	departureTime, err := time.Parse(time.RFC3339, input.DepartureTime)
	if err != nil {
		s.logger.Warn("invalid departure time", zap.String("value", input.DepartureTime))
		return "", tripErrors.ErrorInvalidDatetime
	}

	// Parse des dates de début et fin
	startDate, err := time.Parse("2006-01-02", input.StartDate)
	if err != nil {
		s.logger.Warn("invalid start_date", zap.String("value", input.StartDate))
		return "", tripErrors.ErrorInvalidDatetime
	}
	endDate, err := time.Parse("2006-01-02", input.EndDate)
	if err != nil {
		s.logger.Warn("invalid end_date", zap.String("value", input.EndDate))
		return "", tripErrors.ErrorInvalidDatetime
	}
	if !endDate.After(startDate) && !endDate.Equal(startDate) {
		s.logger.Warn("end_date must be >= start_date")
		return "", tripErrors.ErrorInvalidDatetime
	}

	// Vérification : conducteur certifié
	if err := s.validateVerifiedDriver(ctx, input.DriverID); err != nil {
		return "", err
	}

	// Construction du domaine
	pattern, patternWaypoints := s.buildRecurringPattern(input, departureTime, startDate, endDate)

	// Persistance atomique
	patternID, err := s.writeRepo.CreateRecurringPattern(ctx, pattern, patternWaypoints)
	if err != nil {
		s.logger.Error("create recurring pattern failed", zap.Error(err))
		return "", err
	}

	// Invalidation du cache des previews pour ce conducteur
	if s.cache != nil {
		s.cache.InvalidateDriverPreviews(ctx, input.DriverID)
	}

	s.logger.Info("recurring pattern created",
		zap.String("patternID", patternID),
		zap.String("driverID", input.DriverID),
	)
	return patternID, nil
}

// validateRecurringInput vérifie les champs obligatoires du pattern récurrent.
func (s *tripServiceImpl) validateRecurringInput(input *serviceInterfaces.CreateRecurringTripInput) error {
	if input.DriverID == "" || input.VehicleID == "" {
		return tripErrors.ErrorInvalidInput
	}
	if input.DepartureTime == "" || input.StartDate == "" || input.EndDate == "" {
		return tripErrors.ErrorInvalidInput
	}
	if input.TotalSeats <= 0 {
		return tripErrors.ErrorInvalidInput
	}
	if input.GenerationHorizonDays <= 0 {
		return tripErrors.ErrorInvalidInput
	}

	// Valider le type de récurrence
	switch domain.RecurrenceType(input.RecurrenceType) {
	case domain.RecurrenceTypeDaily:
		// OK — days_of_week ignoré pour daily
	case domain.RecurrenceTypeWeekly, domain.RecurrenceTypeCustom:
		if len(input.DaysOfWeek) == 0 {
			s.logger.Warn("days_of_week required for weekly/custom recurrence")
			return tripErrors.ErrorInvalidInput
		}
	default:
		s.logger.Warn("invalid recurrence_type", zap.String("value", input.RecurrenceType))
		return tripErrors.ErrorInvalidInput
	}

	// Valider les waypoints (mêmes règles que CreateTrip)
	if err := s.validateRecurringWaypoints(input.Waypoints); err != nil {
		return err
	}

	return nil
}

// validateRecurringWaypoints vérifie les waypoints du pattern (min 2, 1 départ, 1 arrivée).
func (s *tripServiceImpl) validateRecurringWaypoints(waypoints []serviceInterfaces.WaypointInput) error {
	if len(waypoints) < minWaypoints {
		s.logger.Warn("not enough waypoints", zap.Int("count", len(waypoints)))
		return tripErrors.ErrorInvalidWaypoints
	}

	var departures, arrivals int
	for _, wp := range waypoints {
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

// buildRecurringPattern construit les structs domaine à partir de l'input.
func (s *tripServiceImpl) buildRecurringPattern(
	input *serviceInterfaces.CreateRecurringTripInput,
	departureTime time.Time,
	startDate, endDate time.Time,
) (*domain.RecurringPattern, []*domain.PatternWaypoint) {
	daysOfWeek := make([]int16, 0, len(input.DaysOfWeek))
	for _, d := range input.DaysOfWeek {
		daysOfWeek = append(daysOfWeek, int16(d))
	}

	patternID := uuid.New().String()
	pattern := &domain.RecurringPattern{
		TripPatternID:         patternID,
		DriverID:              input.DriverID,
		VehicleID:             input.VehicleID,
		DepartureTime:         departureTime,
		RecurrenceType:        domain.RecurrenceType(input.RecurrenceType),
		DaysOfWeek:            daysOfWeek,
		StartDate:             startDate,
		EndDate:               endDate,
		TotalSeats:            input.TotalSeats,
		PricePerSeat:          0, // calculé après la boucle waypoints
		AllowLuggages:         input.AllowLuggages,
		AllowPets:             input.AllowPets,
		AllowFood:             input.AllowFood,
		AllowSmoking:          input.AllowSmoking,
		AutoApproveEnabled:    input.AutoApprove,
		Description:           input.Description,
		GenerationHorizonDays: input.GenerationHorizonDays,
		IsActive:              true,
	}

	patternWaypoints := make([]*domain.PatternWaypoint, 0, len(input.Waypoints))
	for _, wpInput := range input.Waypoints {
		minutesFromDeparture := s.computeMinutesFromDepartureTime(wpInput.ScheduledDatetime, departureTime)
		patternWaypoints = append(patternWaypoints, &domain.PatternWaypoint{
			PatternWaypointID:    uuid.New().String(),
			TripPatternID:        patternID,
			SequencerOrder:       wpInput.SequencerOrder,
			WaypointType:         domain.WaypointType(wpInput.WaypointType),
			LocationName:         wpInput.LocationName,
			LocationLng:          wpInput.LocationLng,
			LocationLat:          wpInput.LocationLat,
			City:                 wpInput.City,
			Country:              wpInput.Country,
			PriceFromPrevious:    wpInput.PriceFromPrevious,
			MinutesFromDeparture: minutesFromDeparture,
		})
	}

	// Calculer price_per_seat comme la somme des price_from_previous
	pricePerSeat := 0
	for _, wp := range patternWaypoints {
		pricePerSeat += wp.PriceFromPrevious
	}
	pattern.PricePerSeat = pricePerSeat

	return pattern, patternWaypoints
}

// computeMinutesFromDepartureTime calcule l'offset en minutes entre un waypoint schedulé
// et l'heure de départ du pattern (seule la partie heure est utilisée).
func (s *tripServiceImpl) computeMinutesFromDepartureTime(scheduledStr string, departureTime time.Time) int {
	if scheduledStr == "" {
		return 0
	}
	t, err := time.Parse(time.RFC3339, scheduledStr)
	if err != nil {
		return 0
	}
	// Comparer uniquement les heures/minutes en normalisant sur la même date
	ref := time.Date(0, 1, 1, departureTime.Hour(), departureTime.Minute(), 0, 0, time.UTC)
	wp := time.Date(0, 1, 1, t.Hour(), t.Minute(), 0, 0, time.UTC)
	diff := wp.Sub(ref)
	if diff < 0 {
		return 0
	}
	return int(diff.Minutes())
}
