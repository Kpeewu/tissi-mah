package implementations

import (
	"context"
	"fmt"
	"time"

	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/domain"
	tripErrors "github.com/Kpeewu/tissi-mah/services/trips-service/pkg/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// CreateRecurringPattern insère un pattern récurrent, ses waypoints de pattern,
// et génère les instances de trajet dans l'horizon de génération.
// Tout est atomique (transaction unique).
func (r *tripWriteRepositoryImpl) CreateRecurringPattern(
	ctx context.Context,
	pattern *domain.RecurringPattern,
	patternWaypoints []*domain.PatternWaypoint,
) (string, error) {
	r.logger.Debug("creating recurring pattern",
		zap.String("patternID", pattern.TripPatternID),
		zap.String("driverID", pattern.DriverID),
		zap.String("recurrenceType", string(pattern.RecurrenceType)),
	)

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		r.logger.Error("begin transaction failed", zap.Error(err))
		return "", tripErrors.ErrorInternalServer
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// 1. Insérer le pattern
	patternID, err := r.insertRecurringPattern(ctx, tx, pattern)
	if err != nil {
		return "", err
	}

	// 2. Insérer les waypoints du pattern
	if err := r.insertPatternWaypoints(ctx, tx, patternID, patternWaypoints); err != nil {
		return "", err
	}

	// 3. Générer les instances de trajet dans l'horizon
	lastGenDate, err := r.generateTripInstances(ctx, tx, patternID, pattern, patternWaypoints)
	if err != nil {
		return "", err
	}

	// 4. Mettre à jour last_generated_date si des trajets ont été créés
	if lastGenDate != nil {
		if err := r.updateLastGeneratedDate(ctx, tx, patternID, *lastGenDate); err != nil {
			return "", err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		r.logger.Error("commit transaction failed", zap.Error(err), zap.String("patternID", patternID))
		return "", tripErrors.ErrorInternalServer
	}

	r.logger.Info("recurring pattern created", zap.String("patternID", patternID))
	return patternID, nil
}

func (r *tripWriteRepositoryImpl) insertRecurringPattern(ctx context.Context, tx pgx.Tx, pattern *domain.RecurringPattern) (string, error) {
	query := `
		INSERT INTO recurring_patterns (
			trip_pattern_id, driver_id, vehicle_id,
			departure_time, recurrence_type,
			days_of_week, start_date, end_date,
			total_seats, price_per_seat,
			allow_luggages, allow_pets, allow_food, allow_smoking,
			auto_approve_enabled, description,
			generation_horizon_days, is_active
		) VALUES (
			$1, $2, $3,
			$4, $5::recurrence_type,
			$6, $7, $8,
			$9, $10,
			$11, $12, $13, $14,
			$15, $16,
			$17, TRUE
		)
		RETURNING trip_pattern_id`

	var patternID string
	err := tx.QueryRow(ctx, query,
		pattern.TripPatternID, pattern.DriverID, pattern.VehicleID,
		pattern.DepartureTime, string(pattern.RecurrenceType),
		pattern.DaysOfWeek, pattern.StartDate, pattern.EndDate,
		pattern.TotalSeats, pattern.PricePerSeat,
		pattern.AllowLuggages, pattern.AllowPets, pattern.AllowFood, pattern.AllowSmoking,
		pattern.AutoApproveEnabled, pattern.Description,
		pattern.GenerationHorizonDays,
	).Scan(&patternID)
	if err != nil {
		r.logger.Error("insert recurring_pattern failed", zap.Error(err))
		return "", fmt.Errorf("%w: %s", tripErrors.ErrorInternalServer, err.Error())
	}
	return patternID, nil
}

func (r *tripWriteRepositoryImpl) insertPatternWaypoints(ctx context.Context, tx pgx.Tx, patternID string, pws []*domain.PatternWaypoint) error {
	query := `
		INSERT INTO waypoint_recurring_patterns (
			pattern_waypoint_id, trip_pattern_id, sequencer_order,
			waypoint_type, location_name,
			location_lng, location_lat,
			city, country,
			price_from_previous, minutes_from_departure
		) VALUES (
			$1, $2, $3,
			$4::waypoint_type, $5,
			$6, $7,
			$8, $9,
			$10, $11
		)`

	for _, pw := range pws {
		_, err := tx.Exec(ctx, query,
			pw.PatternWaypointID, patternID, pw.SequencerOrder,
			string(pw.WaypointType), pw.LocationName,
			pw.LocationLng, pw.LocationLat,
			pw.City, pw.Country,
			pw.PriceFromPrevious, pw.MinutesFromDeparture,
		)
		if err != nil {
			r.logger.Error("insert pattern waypoint failed",
				zap.Error(err),
				zap.String("patternWaypointID", pw.PatternWaypointID),
			)
			return fmt.Errorf("%w: %s", tripErrors.ErrorInternalServer, err.Error())
		}
	}
	return nil
}

// generateTripInstances crée les trajets instanciés dans l'horizon de génération.
// Retourne la dernière date générée (nil si aucune instance créée).
func (r *tripWriteRepositoryImpl) generateTripInstances(
	ctx context.Context,
	tx pgx.Tx,
	patternID string,
	pattern *domain.RecurringPattern,
	pws []*domain.PatternWaypoint,
) (*time.Time, error) {
	// Trouver le waypoint d'arrivée pour l'heure estimée d'arrivée
	arrivalMinutes := r.findArrivalMinutes(pws)

	// Calculer l'horizon de génération
	today := time.Now().UTC().Truncate(24 * time.Hour)
	horizonEnd := today.AddDate(0, 0, pattern.GenerationHorizonDays)
	if pattern.EndDate.Before(horizonEnd) {
		horizonEnd = pattern.EndDate
	}

	// Générer les dates éligibles
	dates := generateEligibleDates(pattern.StartDate, horizonEnd, pattern.RecurrenceType, pattern.DaysOfWeek)
	if len(dates) == 0 {
		return nil, nil
	}

	for _, date := range dates {
		// Combiner la date avec l'heure de départ du pattern
		departure := time.Date(
			date.Year(), date.Month(), date.Day(),
			pattern.DepartureTime.Hour(), pattern.DepartureTime.Minute(), 0, 0,
			time.UTC,
		)
		estimatedArrival := departure.Add(time.Duration(arrivalMinutes) * time.Minute)

		trip := buildTripFromPattern(patternID, pattern, departure, estimatedArrival, arrivalMinutes)
		tripID, err := r.insertTrip(ctx, tx, trip)
		if err != nil {
			r.logger.Error("generate trip instance failed",
				zap.Error(err),
				zap.Time("departure", departure),
			)
			return nil, err
		}

		waypoints := buildWaypointsFromPattern(tripID, pws, departure)
		if err := r.insertWaypoints(ctx, tx, tripID, waypoints); err != nil {
			return nil, err
		}
	}

	lastDate := dates[len(dates)-1]
	return &lastDate, nil
}

func (r *tripWriteRepositoryImpl) updateLastGeneratedDate(ctx context.Context, tx pgx.Tx, patternID string, date time.Time) error {
	_, err := tx.Exec(ctx,
		`UPDATE recurring_patterns SET last_generated_date = $1 WHERE trip_pattern_id = $2`,
		date, patternID,
	)
	if err != nil {
		r.logger.Error("update last_generated_date failed", zap.Error(err))
		return tripErrors.ErrorInternalServer
	}
	return nil
}

// findArrivalMinutes retourne le minutes_from_departure du waypoint d'arrivée.
func (r *tripWriteRepositoryImpl) findArrivalMinutes(pws []*domain.PatternWaypoint) int {
	for _, pw := range pws {
		if pw.WaypointType == domain.WaypointTypeArrival {
			return pw.MinutesFromDeparture
		}
	}
	return 0
}

// generateEligibleDates retourne toutes les dates éligibles entre start et end (inclus)
// selon le type de récurrence et les jours de la semaine.
// Convention : 1=Lundi … 7=Dimanche. Go : Sunday=0, Monday=1, …, Saturday=6.
func generateEligibleDates(start, end time.Time, recurrenceType domain.RecurrenceType, daysOfWeek []int16) []time.Time {
	// Construire un set des jours Go à partir de notre convention
	daySet := make(map[time.Weekday]bool)
	for _, d := range daysOfWeek {
		// 1=Lundi…6=Samedi → time.Monday(1)…time.Saturday(6)
		// 7=Dimanche → time.Sunday(0)
		goDay := time.Weekday(d % 7)
		daySet[goDay] = true
	}

	var dates []time.Time
	current := start.Truncate(24 * time.Hour)
	end = end.Truncate(24 * time.Hour)

	for !current.After(end) {
		eligible := false
		switch recurrenceType {
		case domain.RecurrenceTypeDaily:
			eligible = true
		case domain.RecurrenceTypeWeekly, domain.RecurrenceTypeCustom:
			eligible = daySet[current.Weekday()]
		}
		if eligible {
			dates = append(dates, current)
		}
		current = current.AddDate(0, 0, 1)
	}
	return dates
}

// buildTripFromPattern construit un domain.Trip à partir d'un pattern pour une date donnée.
func buildTripFromPattern(
	patternID string,
	pattern *domain.RecurringPattern,
	departure, estimatedArrival time.Time,
	durationMinutes int,
) *domain.Trip {
	return &domain.Trip{
		TripID:                   uuid.New().String(),
		DriverID:                 pattern.DriverID,
		VehicleID:                pattern.VehicleID,
		RecurringPatternID:       &patternID,
		DepartureDatetime:        departure,
		EstimatedArrivalDatetime: estimatedArrival,
		EstimatedDurationMinutes: durationMinutes,
		EstimatedDistanceMeters:  1, // non connu pour les trajets récurrents
		TotalSeats:               int16(pattern.TotalSeats),
		AvailableSeats:           int16(pattern.TotalSeats),
		PricePerSeat:             pattern.PricePerSeat,
		PaymentMethodsAccepted:   []string{},
		AllowLuggages:            pattern.AllowLuggages,
		AllowPets:                pattern.AllowPets,
		AllowFood:                pattern.AllowFood,
		AllowSmoking:             pattern.AllowSmoking,
		Status:                   domain.TripStatusScheduled,
		AutoApproveEnabled:       pattern.AutoApproveEnabled,
		Description:              pattern.Description,
	}
}

// buildWaypointsFromPattern construit les waypoints d'un trajet instancié
// à partir des waypoints du pattern et de l'heure de départ.
func buildWaypointsFromPattern(tripID string, pws []*domain.PatternWaypoint, departure time.Time) []*domain.Waypoint {
	waypoints := make([]*domain.Waypoint, 0, len(pws))
	for _, pw := range pws {
		scheduled := departure.Add(time.Duration(pw.MinutesFromDeparture) * time.Minute)
		waypoints = append(waypoints, &domain.Waypoint{
			WaypointID:              uuid.New().String(),
			TripID:                  tripID,
			SequencerOrder:          pw.SequencerOrder,
			WaypointType:            pw.WaypointType,
			LocationName:            pw.LocationName,
			LocationLng:             pw.LocationLng,
			LocationLat:             pw.LocationLat,
			City:                    pw.City,
			Country:                 pw.Country,
			ScheduledPickupDatetime: &scheduled,
			MinutesFromDeparture:    pw.MinutesFromDeparture,
			PriceFromPrevious:       pw.PriceFromPrevious,
		})
	}
	return waypoints
}
