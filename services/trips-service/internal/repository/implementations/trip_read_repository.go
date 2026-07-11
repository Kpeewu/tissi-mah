package implementations

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/domain"
	i "github.com/Kpeewu/tissi-mah/services/trips-service/internal/repository/interfaces"
	tripErrors "github.com/Kpeewu/tissi-mah/services/trips-service/pkg/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// GetTripTotalSeats retourne le nombre total de places d'un trajet.
func (r *tripReadRepositoryImpl) GetTripTotalSeats(ctx context.Context, tripID string) (int16, error) {
	var totalSeats int16
	err := r.pool.QueryRow(ctx, `SELECT total_seats FROM trips WHERE trip_id = $1 AND deleted_at IS NULL`, tripID).Scan(&totalSeats)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, tripErrors.ErrorTripNotFound
		}
		r.logger.Error("GetTripTotalSeats failed", zap.Error(err), zap.String("tripID", tripID))
		return 0, tripErrors.ErrorInternalServer
	}
	return totalSeats, nil
}

// GetTripByID retourne les détails complets d'un trajet avec ses waypoints.
func (r *tripReadRepositoryImpl) GetTripByID(ctx context.Context, tripID string) (*domain.Trip, []*domain.Waypoint, error) {
	r.logger.Debug("GetTripByID", zap.String("tripID", tripID))

	// Récupérer le trajet
	tripQuery := `
		SELECT trip_id, driver_id, vehicle_id, recurring_pattern_id,
		       departure_datetime, actual_departure_datetime,
		       estimated_arrival_datetime, actual_arrival_datetime,
		       estimated_duration_minutes, estimated_distance_meters,
		       total_seats, available_seats, price_per_seat,
		       payment_methods_accepted::TEXT[],
		       allow_luggages, allow_pets, allow_food, allow_smoking,
		       status, auto_approve_enabled,
		       canceller_id, cancellation_reason,
		       description, route_polyline, created_at, updated_at
		FROM trips
		WHERE trip_id = $1 AND deleted_at IS NULL`

	trip := &domain.Trip{}
	err := r.pool.QueryRow(ctx, tripQuery, tripID).Scan(
		&trip.TripID, &trip.DriverID, &trip.VehicleID, &trip.RecurringPatternID,
		&trip.DepartureDatetime, &trip.ActualDepartureDatetime,
		&trip.EstimatedArrivalDatetime, &trip.ActualArrivalDatetime,
		&trip.EstimatedDurationMinutes, &trip.EstimatedDistanceMeters,
		&trip.TotalSeats, &trip.AvailableSeats, &trip.PricePerSeat,
		&trip.PaymentMethodsAccepted,
		&trip.AllowLuggages, &trip.AllowPets, &trip.AllowFood, &trip.AllowSmoking,
		&trip.Status, &trip.AutoApproveEnabled,
		&trip.CancellerID, &trip.CancellationReason,
		&trip.Description, &trip.RoutePolyline, &trip.CreatedAt, &trip.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, tripErrors.ErrorTripNotFound
		}
		r.logger.Error("GetTripByID trip query failed", zap.Error(err), zap.String("tripID", tripID))
		return nil, nil, tripErrors.ErrorDataRetrievalFailed
	}

	// Récupérer les waypoints du trajet
	waypointQuery := `
		SELECT waypoint_id, trip_id, sequencer_order, waypoint_type,
		       location_name, location_lng, location_lat, city, country,
		       scheduled_pickup_datetime, actual_arrival_datetime,
		       actual_scheduled_pickup_datetime,
		       minutes_from_departure, price_from_previous,
		       cancelled_at, cancellation_reason,
		       created_at, updated_at
		FROM trips_waypoints
		WHERE trip_id = $1 AND deleted_at IS NULL
		ORDER BY sequencer_order ASC`

	rows, err := r.pool.Query(ctx, waypointQuery, tripID)
	if err != nil {
		r.logger.Error("GetTripByID waypoints query failed", zap.Error(err), zap.String("tripID", tripID))
		return nil, nil, tripErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	var waypoints []*domain.Waypoint
	for rows.Next() {
		wp := &domain.Waypoint{}
		if err := rows.Scan(
			&wp.WaypointID, &wp.TripID, &wp.SequencerOrder, &wp.WaypointType,
			&wp.LocationName, &wp.LocationLng, &wp.LocationLat, &wp.City, &wp.Country,
			&wp.ScheduledPickupDatetime, &wp.ActualArrivalDatetime,
			&wp.ActualScheduledPickupDatetime,
			&wp.MinutesFromDeparture, &wp.PriceFromPrevious,
			&wp.CancelledAt, &wp.CancellationReason,
			&wp.CreatedAt, &wp.UpdatedAt,
		); err != nil {
			r.logger.Error("GetTripByID waypoint scan failed", zap.Error(err))
			return nil, nil, tripErrors.ErrorDataRetrievalFailed
		}
		waypoints = append(waypoints, wp)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("GetTripByID waypoints rows error", zap.Error(err))
		return nil, nil, tripErrors.ErrorDataRetrievalFailed
	}

	return trip, waypoints, nil
}

type tripReadRepositoryImpl struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewTripReadRepository(pool *pgxpool.Pool, logger *zap.Logger) i.TripRepositoryRead {
	return &tripReadRepositoryImpl{pool: pool, logger: logger}
}

// GetDriverTripsPreviews retourne la liste paginée des trajets d'un conducteur
// dont le statut est différent de "completed", avec les noms des points de départ et d'arrivée.
func (r *tripReadRepositoryImpl) GetDriverTripsPreviews(ctx context.Context, driverID string, pageIndex int) ([]*domain.TripPreview, error) {
	r.logger.Debug("GetDriverTripsPreviews", zap.String("driverID", driverID), zap.Int("pageIndex", pageIndex))

	query := `
		SELECT
			t.trip_id,
			t.driver_id,
			t.vehicle_id,
			t.departure_datetime,
			t.total_seats,
			t.available_seats,
			dep.location_name AS departure_location_name,
			arr.location_name AS arrival_location_name
		FROM trips t
		JOIN trips_waypoints dep ON dep.trip_id = t.trip_id AND dep.waypoint_type = 'departure'::waypoint_type
		JOIN trips_waypoints arr ON arr.trip_id = t.trip_id AND arr.waypoint_type = 'arrival'::waypoint_type
		WHERE t.driver_id = $1
		  AND t.status <> 'completed'::trip_status
		  AND t.deleted_at IS NULL
		ORDER BY t.departure_datetime DESC
		LIMIT 10 OFFSET $2`

	return r.scanTripPreviews(ctx, query, driverID, pageIndex)
}

// GetDriverCompletedTripsPreviews retourne la liste paginée des trajets complétés d'un conducteur.
func (r *tripReadRepositoryImpl) GetDriverCompletedTripsPreviews(ctx context.Context, driverID string, pageIndex int) ([]*domain.TripPreview, error) {
	r.logger.Debug("GetDriverCompletedTripsPreviews", zap.String("driverID", driverID), zap.Int("pageIndex", pageIndex))

	query := `
		SELECT
			t.trip_id,
			t.driver_id,
			t.vehicle_id,
			t.departure_datetime,
			t.total_seats,
			t.available_seats,
			dep.location_name AS departure_location_name,
			arr.location_name AS arrival_location_name
		FROM trips t
		JOIN trips_waypoints dep ON dep.trip_id = t.trip_id AND dep.waypoint_type = 'departure'::waypoint_type
		JOIN trips_waypoints arr ON arr.trip_id = t.trip_id AND arr.waypoint_type = 'arrival'::waypoint_type
		WHERE t.driver_id = $1
		  AND t.status = 'completed'::trip_status
		  AND t.deleted_at IS NULL
		ORDER BY t.departure_datetime DESC
		LIMIT 10 OFFSET $2`

	return r.scanTripPreviews(ctx, query, driverID, pageIndex)
}

// scanTripPreviews exécute une requête de previews et scanne les résultats.
func (r *tripReadRepositoryImpl) scanTripPreviews(ctx context.Context, query string, driverID string, pageIndex int) ([]*domain.TripPreview, error) {
	rows, err := r.pool.Query(ctx, query, driverID, pageIndex*10)
	if err != nil {
		r.logger.Error("trip previews query failed", zap.Error(err), zap.String("driverID", driverID))
		return nil, tripErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	var previews []*domain.TripPreview
	for rows.Next() {
		p := &domain.TripPreview{}
		if err := rows.Scan(
			&p.TripID,
			&p.DriverID,
			&p.VehicleID,
			&p.DepartureDatetime,
			&p.TotalSeats,
			&p.AvailableSeats,
			&p.DepartureLocationName,
			&p.ArrivalLocationName,
		); err != nil {
			r.logger.Error("trip previews scan failed", zap.Error(err))
			return nil, tripErrors.ErrorDataRetrievalFailed
		}
		previews = append(previews, p)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("trip previews rows error", zap.Error(err))
		return nil, tripErrors.ErrorDataRetrievalFailed
	}

	return previews, nil
}

// searchWordSimilarityThreshold est le seuil de word_similarity pour le fuzzy matching.
// word_similarity mesure la meilleure correspondance du terme recherché contre une portion
// du texte : « abalpedo » vs « Gare d'Agbalkpédo, Lomé » ≈ 0.33. À 0.30 on tolère les fautes
// de frappe usuelles sans ramener trop de bruit.
const searchWordSimilarityThreshold = 0.30

// searchSortByOrderClauses mappe les valeurs SortBy autorisées vers leur clause ORDER BY.
// Whitelist stricte : l'input utilisateur n'est jamais interpolé dans le SQL.
var searchSortByOrderClauses = map[string]string{
	"relevance":      "relevance_score DESC, departure_datetime ASC",
	"departure_time": "departure_datetime ASC, relevance_score DESC",
	"price":          "segment_price ASC, relevance_score DESC",
}

// SearchScheduledTripSegments recherche les trajets/segments disponibles avec pagination.
// Construit une requête SQL dynamique avec self-join pour générer les segments, fuzzy
// matching accent-insensitive (word_similarity sur location_name + city), zone de départ
// en OU (nom OU rayon autour des coordonnées fournies) et score de pertinence.
//
// Les prédicats fuzzy/spatiaux s'appliquent en filtre sur les segments candidats (entrée
// par l'index partiel idx_trips_scheduled_search + index de join). Si la volumétrie de
// trajets scheduled explose, passer à l'opérateur <% avec
// ALTER DATABASE ... SET pg_trgm.word_similarity_threshold pour exploiter les index GIN
// trigramme (idx_trips_waypoints_location_name_trgm / idx_trips_waypoints_city_trgm).
func (r *tripReadRepositoryImpl) SearchScheduledTripSegments(ctx context.Context, params *i.SearchTripsParams) (*i.SearchTripsResult, error) {
	r.logger.Debug("SearchScheduledTripSegments",
		zap.String("departure", params.DepartureLocationName),
		zap.String("arrival", params.ArrivalLocationName),
		zap.String("sortBy", params.SortBy),
		zap.Int("pageIndex", params.PageIndex),
	)

	pageSize := params.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}

	args := make([]any, 0, 16)
	argIdx := 1

	// Arguments toujours présents : noms recherchés + seuil de similarité
	depArg := argIdx
	args = append(args, params.DepartureLocationName)
	argIdx++
	arrArg := argIdx
	args = append(args, params.ArrivalLocationName)
	argIdx++
	thresholdArg := argIdx
	args = append(args, searchWordSimilarityThreshold)
	argIdx++

	// Expressions géographiques : neutres si pas de coordonnées de zone de départ
	hasCoords := params.PassengerLng != nil && params.PassengerLat != nil
	depGeoWithinExpr := "FALSE"
	geoScoreExpr := "0::float8"
	relevanceExpr := "0.5 * dep_name_score + 0.5 * arr_name_score"
	if hasCoords {
		distanceMeters := params.DistanceRangeMeters
		if distanceMeters <= 0 {
			distanceMeters = 5000
		}
		pointExpr := fmt.Sprintf("ST_SetSRID(ST_MakePoint($%d, $%d), 4326)::geography", argIdx, argIdx+1)
		args = append(args, *params.PassengerLng, *params.PassengerLat)
		argIdx += 2
		radiusArg := argIdx
		args = append(args, distanceMeters)
		argIdx++

		depGeoWithinExpr = fmt.Sprintf("ST_DWithin(dep_wp.position, %s, $%d)", pointExpr, radiusArg)
		// 1 au point exact, 0 au bord du rayon (et au-delà)
		geoScoreExpr = fmt.Sprintf(
			"GREATEST(0::float8, 1 - ST_Distance(dep_wp.position, %s) / $%d)", pointExpr, radiusArg)
		relevanceExpr = "0.4 * dep_name_score + 0.4 * arr_name_score + 0.2 * geo_score"
	}

	// Conditions de la CTE candidates (avant scoring)
	conditions := []string{
		"t.status = 'scheduled'::trip_status",
		"t.available_seats > 0",
		"t.deleted_at IS NULL",
	}

	// Filtre date de départ (UTC)
	if params.TripStartDate != nil && *params.TripStartDate != "" {
		conditions = append(conditions, fmt.Sprintf(
			"(t.departure_datetime AT TIME ZONE 'UTC')::DATE = $%d::DATE", argIdx))
		args = append(args, *params.TripStartDate)
		argIdx++
	}

	// Filtre heure de départ (UTC)
	if params.TripStartHour != nil && *params.TripStartHour != "" {
		conditions = append(conditions, fmt.Sprintf(
			"(t.departure_datetime AT TIME ZONE 'UTC')::TIME >= $%d::TIME", argIdx))
		args = append(args, *params.TripStartHour)
		argIdx++
	}

	// Filtre heure d'arrivée (UTC)
	if params.TripArrivalHour != nil && *params.TripArrivalHour != "" {
		conditions = append(conditions, fmt.Sprintf(
			"(t.estimated_arrival_datetime AT TIME ZONE 'UTC')::TIME <= $%d::TIME", argIdx))
		args = append(args, *params.TripArrivalHour)
		argIdx++
	}

	// Filtres options : uniquement les trajets qui autorisent l'option demandée
	if params.AllowLuggages {
		conditions = append(conditions, "t.allow_luggages = TRUE")
	}
	if params.AllowPets {
		conditions = append(conditions, "t.allow_pets = TRUE")
	}
	if params.AllowFood {
		conditions = append(conditions, "t.allow_food = TRUE")
	}
	if params.AllowSmoking {
		conditions = append(conditions, "t.allow_smoking = TRUE")
	}

	// Conditions de la CTE filtered (sur scores et agrégats)
	depMatchCond := "dep_name_score >= $" + strconv.Itoa(thresholdArg)
	if hasCoords {
		// Zone de départ en OU : le nom matche approximativement OU le départ est dans le rayon
		depMatchCond = fmt.Sprintf("(dep_name_score >= $%d OR dep_geo_within)", thresholdArg)
	}
	filteredConditions := []string{
		depMatchCond,
		fmt.Sprintf("arr_name_score >= $%d", thresholdArg),
	}

	minSeats := params.MinSeats
	if minSeats <= 0 {
		minSeats = 1
	}
	filteredConditions = append(filteredConditions, fmt.Sprintf("available_seats >= $%d", argIdx))
	args = append(args, minSeats)
	argIdx++

	if params.MaxPrice > 0 {
		filteredConditions = append(filteredConditions, fmt.Sprintf("segment_price <= $%d", argIdx))
		args = append(args, params.MaxPrice)
		argIdx++
	}

	// CTE partagée entre count et data : segments candidats + scores, puis filtrage
	// (les agrégats prix/sièges ne sont filtrables qu'après leur calcul, d'où la CTE)
	cteClause := fmt.Sprintf(`
		WITH candidates AS (
			SELECT
				t.trip_id,
				t.driver_id,
				t.vehicle_id,
				t.departure_datetime,
				t.total_seats,
				-- Places disponibles par segment : total - MAX(booked_seats) sur les legs couverts
				t.total_seats - COALESCE((
					SELECT MAX(leg.booked_seats)
					FROM trips_waypoints leg
					WHERE leg.trip_id = t.trip_id
					  AND leg.sequencer_order >= dep_wp.sequencer_order
					  AND leg.sequencer_order < arr_wp.sequencer_order
					  AND leg.cancelled_at IS NULL
					  AND leg.deleted_at IS NULL
				), 0) AS available_seats,
				dep_wp.location_name AS departure_location_name,
				arr_wp.location_name AS arrival_location_name,
				dep_wp.waypoint_id AS departure_waypoint_id,
				arr_wp.waypoint_id AS arrival_waypoint_id,
				-- Prix du segment : somme des price_from_previous entre dep+1 et arr
				COALESCE((
					SELECT SUM(seg.price_from_previous)
					FROM trips_waypoints seg
					WHERE seg.trip_id = t.trip_id
					  AND seg.sequencer_order > dep_wp.sequencer_order
					  AND seg.sequencer_order <= arr_wp.sequencer_order
					  AND seg.cancelled_at IS NULL
					  AND seg.deleted_at IS NULL
				), 0) AS segment_price,
				-- Durée du segment en minutes
				arr_wp.minutes_from_departure - dep_wp.minutes_from_departure AS segment_duration_minutes,
				-- Similarité fuzzy accent-insensitive sur location_name OU city
				GREATEST(
					word_similarity(f_unaccent($%[1]d), f_unaccent(dep_wp.location_name)),
					word_similarity(f_unaccent($%[1]d), f_unaccent(COALESCE(dep_wp.city, '')))
				) AS dep_name_score,
				GREATEST(
					word_similarity(f_unaccent($%[2]d), f_unaccent(arr_wp.location_name)),
					word_similarity(f_unaccent($%[2]d), f_unaccent(COALESCE(arr_wp.city, '')))
				) AS arr_name_score,
				%[3]s AS dep_geo_within,
				%[4]s AS geo_score
			FROM trips t
			JOIN trips_waypoints dep_wp
				ON dep_wp.trip_id = t.trip_id
				AND dep_wp.cancelled_at IS NULL
				AND dep_wp.deleted_at IS NULL
			JOIN trips_waypoints arr_wp
				ON arr_wp.trip_id = t.trip_id
				AND arr_wp.cancelled_at IS NULL
				AND arr_wp.deleted_at IS NULL
				AND arr_wp.sequencer_order > dep_wp.sequencer_order
			WHERE %[5]s
		),
		filtered AS (
			SELECT *, %[6]s AS relevance_score
			FROM candidates
			WHERE %[7]s
		)`,
		depArg, arrArg, depGeoWithinExpr, geoScoreExpr,
		strings.Join(conditions, "\n  AND "),
		relevanceExpr,
		strings.Join(filteredConditions, "\n  AND "),
	)

	// Tri whitelisté (défaut : pertinence) — validé en amont par le service
	orderBy, ok := searchSortByOrderClauses[params.SortBy]
	if !ok {
		orderBy = searchSortByOrderClauses["relevance"]
	}

	countQuery := cteClause + "\nSELECT COUNT(*) FROM filtered"
	dataQuery := cteClause + fmt.Sprintf(`
		SELECT
			trip_id, driver_id, vehicle_id, departure_datetime, total_seats,
			available_seats, departure_location_name, arrival_location_name,
			departure_waypoint_id, arrival_waypoint_id, segment_price,
			segment_duration_minutes, relevance_score
		FROM filtered
		ORDER BY %s
		LIMIT $%d OFFSET $%d`, orderBy, argIdx, argIdx+1)
	dataArgs := append(append([]any{}, args...), pageSize, params.PageIndex*pageSize)

	var totalCount int
	var previews []*domain.TripPreview

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return r.pool.QueryRow(gCtx, countQuery, args...).Scan(&totalCount)
	})

	g.Go(func() error {
		rows, err := r.pool.Query(gCtx, dataQuery, dataArgs...)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			p := &domain.TripPreview{}
			if err := rows.Scan(
				&p.TripID,
				&p.DriverID,
				&p.VehicleID,
				&p.DepartureDatetime,
				&p.TotalSeats,
				&p.AvailableSeats,
				&p.DepartureLocationName,
				&p.ArrivalLocationName,
				&p.DepartureWaypointID,
				&p.ArrivalWaypointID,
				&p.SegmentPrice,
				&p.SegmentDurationMinutes,
				&p.RelevanceScore,
			); err != nil {
				return err
			}
			previews = append(previews, p)
		}
		return rows.Err()
	})

	if err := g.Wait(); err != nil {
		r.logger.Error("SearchScheduledTripSegments failed", zap.Error(err))
		return nil, tripErrors.ErrorDataRetrievalFailed
	}

	return &i.SearchTripsResult{
		Previews:   previews,
		TotalCount: totalCount,
	}, nil
}

// GetWaypointIDByType retourne l'ID du waypoint d'un type donné pour un trajet.
func (r *tripReadRepositoryImpl) GetWaypointIDByType(ctx context.Context, tripID, waypointType string) (string, error) {
	var waypointID string
	err := r.pool.QueryRow(ctx,
		`SELECT waypoint_id FROM trips_waypoints
		 WHERE trip_id = $1 AND waypoint_type::TEXT = $2 AND deleted_at IS NULL
		 LIMIT 1`,
		tripID, waypointType,
	).Scan(&waypointID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", tripErrors.ErrorWaypointNotFound
		}
		r.logger.Error("GetWaypointIDByType failed", zap.Error(err), zap.String("tripID", tripID))
		return "", tripErrors.ErrorDataRetrievalFailed
	}
	return waypointID, nil
}

// GetTripIDByWaypointID retourne le tripID associé à un waypointID.
func (r *tripReadRepositoryImpl) GetTripIDByWaypointID(ctx context.Context, waypointID string) (string, error) {
	var tripID string
	err := r.pool.QueryRow(ctx,
		`SELECT trip_id FROM trips_waypoints WHERE waypoint_id = $1 AND deleted_at IS NULL`,
		waypointID,
	).Scan(&tripID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", tripErrors.ErrorWaypointNotFound
		}
		r.logger.Error("GetTripIDByWaypointID failed", zap.Error(err), zap.String("waypointID", waypointID))
		return "", tripErrors.ErrorDataRetrievalFailed
	}
	return tripID, nil
}

// HasActiveTripAsDriver vérifie si un conducteur a un trajet en cours (status = inProgress).
func (r *tripReadRepositoryImpl) HasActiveTripAsDriver(ctx context.Context, driverID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM trips WHERE driver_id = $1 AND status = 'inProgress' AND deleted_at IS NULL)`,
		driverID,
	).Scan(&exists)
	if err != nil {
		r.logger.Error("HasActiveTripAsDriver failed", zap.Error(err), zap.String("driverID", driverID))
		return false, tripErrors.ErrorInternalServer
	}
	return exists, nil
}

// GetVehicleCompletedTripCount retourne le nombre de trajets complétés pour un véhicule.
func (r *tripReadRepositoryImpl) GetVehicleCompletedTripCount(ctx context.Context, vehicleID string) (int32, error) {
	var count int32
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM trips WHERE vehicle_id = $1 AND status = 'completed' AND deleted_at IS NULL`,
		vehicleID,
	).Scan(&count)
	if err != nil {
		r.logger.Error("GetVehicleCompletedTripCount failed", zap.Error(err), zap.String("vehicleID", vehicleID))
		return 0, tripErrors.ErrorInternalServer
	}
	return count, nil
}
