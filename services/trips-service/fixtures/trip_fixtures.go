package fixtures

import (
	"context"
	"time"

	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// =============================================================================
// Options fonctionnelles — Trip
// =============================================================================

// TripOption est une option fonctionnelle pour NewTestTrip.
type TripOption func(*domain.Trip)

// WithDriverID définit le DriverID du trajet.
func WithDriverID(id string) TripOption {
	return func(t *domain.Trip) { t.DriverID = id }
}

// WithVehicleID définit le VehicleID du trajet.
func WithVehicleID(id string) TripOption {
	return func(t *domain.Trip) { t.VehicleID = id }
}

// WithStatus définit le statut du trajet.
func WithStatus(s domain.TripStatus) TripOption {
	return func(t *domain.Trip) { t.Status = s }
}

// WithTotalSeats définit le nombre de places totales du trajet.
func WithTotalSeats(n int16) TripOption {
	return func(t *domain.Trip) {
		t.TotalSeats = n
		t.AvailableSeats = n
	}
}

// NewTestTrip crée un trajet de test avec des valeurs par défaut cohérentes.
// Par défaut : IDs générés par UUID, départ dans 1h, arrivée dans 3h,
// statut scheduled, 4 places, méthode de paiement "cash".
func NewTestTrip(opts ...TripOption) *domain.Trip {
	now := time.Now().UTC()
	departure := now.Add(1 * time.Hour)
	arrival := now.Add(3 * time.Hour)

	t := &domain.Trip{
		TripID:                   uuid.New().String(),
		DriverID:                 uuid.New().String(),
		VehicleID:                uuid.New().String(),
		DepartureDatetime:        departure,
		EstimatedArrivalDatetime: arrival,
		EstimatedDurationMinutes: 120,
		EstimatedDistanceMeters:  50000,
		TotalSeats:               4,
		AvailableSeats:           4,
		PricePerSeat:             1000,
		PaymentMethodsAccepted:   []string{"cash"},
		AllowLuggages:            false,
		AllowPets:                false,
		AllowFood:                false,
		AllowSmoking:             false,
		Status:                   domain.TripStatusScheduled,
		AutoApproveEnabled:       false,
		Description:              "trajet de test",
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

// =============================================================================
// Options fonctionnelles — Waypoint
// =============================================================================

// WaypointOption est une option fonctionnelle pour NewTestWaypoint.
type WaypointOption func(*domain.Waypoint)

// WithWaypointType définit le type du waypoint.
func WithWaypointType(wt domain.WaypointType) WaypointOption {
	return func(w *domain.Waypoint) { w.WaypointType = wt }
}

// WithSequencerOrder définit l'ordre du waypoint dans la séquence.
func WithSequencerOrder(order int16) WaypointOption {
	return func(w *domain.Waypoint) { w.SequencerOrder = order }
}

// WithCity définit la ville du waypoint.
func WithCity(city string) WaypointOption {
	return func(w *domain.Waypoint) { w.City = city }
}

// NewTestWaypoint crée un waypoint de test avec des valeurs par défaut.
// Par défaut : ID UUID, type=stop, ordre=2, Lomé, Togo.
func NewTestWaypoint(tripID string, opts ...WaypointOption) *domain.Waypoint {
	w := &domain.Waypoint{
		WaypointID:           uuid.New().String(),
		TripID:               tripID,
		SequencerOrder:       2,
		WaypointType:         domain.WaypointTypeStop,
		LocationName:         "Arrêt test",
		LocationLng:          1.2228,
		LocationLat:          6.1375,
		City:                 "Lomé",
		Country:              "TG",
		MinutesFromDeparture: 60,
		PriceFromPrevious:    500,
	}

	for _, opt := range opts {
		opt(w)
	}

	return w
}

// NewTestDepartureWaypoint crée un waypoint de départ pour le tripID fourni.
// SequencerOrder=1, WaypointType=departure.
func NewTestDepartureWaypoint(tripID string) *domain.Waypoint {
	return NewTestWaypoint(tripID,
		WithWaypointType(domain.WaypointTypeDeparture),
		WithSequencerOrder(1),
		WithCity("Lomé"),
	)
}

// NewTestArrivalWaypoint crée un waypoint d'arrivée pour le tripID fourni.
// SequencerOrder=3, WaypointType=arrival.
func NewTestArrivalWaypoint(tripID string) *domain.Waypoint {
	return NewTestWaypoint(tripID,
		WithWaypointType(domain.WaypointTypeArrival),
		WithSequencerOrder(3),
		WithCity("Kpalimé"),
	)
}

// =============================================================================
// Helpers d'insertion — base de données
// =============================================================================

// InsertTrip insère un trajet dans la base de données de test.
// N'insère pas les waypoints — utiliser InsertWaypoint séparément.
func InsertTrip(ctx context.Context, pool *pgxpool.Pool, trip *domain.Trip) error {
	query := `
		INSERT INTO trips (
			trip_id, driver_id, vehicle_id,
			recurring_pattern_id,
			departure_datetime, estimated_arrival_datetime,
			estimated_duration_minutes, estimated_distance_meters,
			total_seats, available_seats, price_per_seat,
			payment_methods_accepted,
			allow_luggages, allow_pets, allow_food, allow_smoking,
			status, auto_approve_enabled, description
		) VALUES (
			$1, $2, $3,
			$4,
			$5, $6,
			$7, $8,
			$9, $10, $11,
			$12::text[]::payment_method[],
			$13, $14, $15, $16,
			$17::trip_status, $18, $19
		)`

	_, err := pool.Exec(ctx, query,
		trip.TripID, trip.DriverID, trip.VehicleID,
		trip.RecurringPatternID,
		trip.DepartureDatetime, trip.EstimatedArrivalDatetime,
		trip.EstimatedDurationMinutes, trip.EstimatedDistanceMeters,
		trip.TotalSeats, trip.AvailableSeats, trip.PricePerSeat,
		trip.PaymentMethodsAccepted,
		trip.AllowLuggages, trip.AllowPets, trip.AllowFood, trip.AllowSmoking,
		string(trip.Status), trip.AutoApproveEnabled, trip.Description,
	)
	return err
}

// InsertWaypoint insère un waypoint dans la base de données de test.
// La colonne position est calculée via ST_MakePoint(lng, lat).
func InsertWaypoint(ctx context.Context, pool *pgxpool.Pool, wp *domain.Waypoint) error {
	query := `
		INSERT INTO trips_waypoints (
			waypoint_id, trip_id, sequencer_order,
			waypoint_type, location_name,
			location_lng, location_lat,
			position,
			city, country,
			scheduled_pickup_datetime,
			minutes_from_departure, price_from_previous
		) VALUES (
			$1, $2, $3,
			$4::waypoint_type, $5,
			$6, $7,
			ST_SetSRID(ST_MakePoint($13::float8, $14::float8), 4326)::geography,
			$8, $9,
			$10,
			$11, $12
		)`

	_, err := pool.Exec(ctx, query,
		wp.WaypointID, wp.TripID, wp.SequencerOrder,
		string(wp.WaypointType), wp.LocationName,
		wp.LocationLng, wp.LocationLat,
		wp.City, wp.Country,
		wp.ScheduledPickupDatetime,
		wp.MinutesFromDeparture, wp.PriceFromPrevious,
		wp.LocationLng, wp.LocationLat, // $13=lng, $14=lat pour ST_MakePoint
	)
	return err
}
