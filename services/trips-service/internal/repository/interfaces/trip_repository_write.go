package interfaces

import (
	"context"
	"time"

	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/domain"
)

// TripRepositoryWrite définit les opérations d'écriture sur les tables trips et trips_waypoints.
// La création d'un trajet et de ses waypoints est atomique (transaction unique).
type TripRepositoryWrite interface {
	// Create insère un trajet et ses waypoints dans une transaction unique.
	// Retourne le tripID en cas de succès.
	Create(ctx context.Context, trip *domain.Trip, waypoints []*domain.Waypoint) (string, error)

	// CreateRecurringPattern insère un pattern récurrent, ses waypoints de pattern,
	// puis génère les instances de trajet dans l'horizon [startDate, min(endDate, today+horizonDays)].
	// Tout est atomique. Retourne le patternID en cas de succès.
	CreateRecurringPattern(ctx context.Context, pattern *domain.RecurringPattern, patternWaypoints []*domain.PatternWaypoint) (string, error)

	// UpdateDepartureDatetime met à jour la date/heure de départ d'un trajet planifié.
	// Retourne ErrorTripNotFound si le trajet n'existe pas, ErrorUnauthorized si le conducteur
	// n'est pas propriétaire du trajet, ErrorTripNotScheduled si le statut n'est pas "scheduled".
	UpdateDepartureDatetime(ctx context.Context, tripID, driverID string, newDatetime time.Time) error

	// UpdateVehicle met à jour le véhicule associé à un trajet planifié.
	// Retourne ErrorTripNotFound si le trajet n'existe pas, ErrorUnauthorized si le conducteur
	// n'est pas propriétaire du trajet, ErrorTripNotScheduled si le statut n'est pas "scheduled".
	UpdateVehicle(ctx context.Context, tripID, driverID, vehicleID string) error

	// UpdateAllowances met à jour les autorisations d'un trajet planifié.
	// Retourne ErrorTripNotFound si le trajet n'existe pas, ErrorUnauthorized si le conducteur
	// n'est pas propriétaire du trajet, ErrorTripNotScheduled si le statut n'est pas "scheduled",
	// ErrorTripDepartureTooSoon si le départ est dans moins de 24h.
	UpdateAllowances(ctx context.Context, tripID, driverID string, allowPets, allowFood, allowSmoking, allowLuggages bool) error
}
