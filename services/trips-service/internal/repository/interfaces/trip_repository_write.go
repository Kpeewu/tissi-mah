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

	// UpdateAutoApprove active ou désactive l'approbation automatique d'un trajet.
	// Retourne ErrorTripNotFound si le trajet n'existe pas, ErrorUnauthorized si le conducteur
	// n'est pas propriétaire du trajet, ErrorTripNotScheduled si le statut n'est pas "scheduled"
	// ou "inProgress".
	UpdateAutoApprove(ctx context.Context, tripID, driverID string, autoApprove bool) error

	// StartTrip passe un trajet planifié au statut "inProgress" de façon atomique.
	// Définit actual_departure_datetime sur le trajet et actual_scheduled_pickup_datetime
	// sur le waypoint de départ.
	// Retourne ErrorDriverAlreadyHasActiveTrip si le conducteur a déjà un trajet inProgress,
	// ErrorTripNotFound si le trajet n'existe pas, ErrorUnauthorized si le conducteur n'est
	// pas propriétaire du trajet, ErrorTripNotScheduled si le statut n'est pas "scheduled".
	StartTrip(ctx context.Context, tripID, driverID string) error

	// EndTrip passe un trajet en cours au statut "completed" de façon atomique.
	// Définit actual_arrival_datetime sur le trajet et actual_scheduled_pickup_datetime
	// sur le waypoint d'arrivée.
	// Retourne ErrorTripNotFound si le trajet n'existe pas, ErrorUnauthorized si le conducteur
	// n'est pas propriétaire du trajet, ErrorTripNotInProgress si le statut n'est pas "inProgress".
	EndTrip(ctx context.Context, tripID, driverID string) error

	// ConfirmWaypointArrival enregistre l'arrivée du conducteur à un waypoint de type "stop".
	// Vérifie que le trip est inProgress, que le conducteur en est propriétaire, que le waypoint
	// est bien un "stop", qu'aucun autre stop n'est déjà actif (arrivé mais non parti), et que
	// le waypoint précédent dans l'ordre a bien été confirmé.
	// Retourne ErrorWaypointNotFound, ErrorUnauthorized, ErrorTripNotInProgress,
	// ErrorWaypointNotAStop, ErrorWaypointAlreadyArrived, ErrorAnotherStopAlreadyActive,
	// ErrorPreviousWaypointNotConfirmed selon le cas.
	ConfirmWaypointArrival(ctx context.Context, waypointID, driverID string) error

	// ConfirmWaypointDeparture enregistre le départ du conducteur d'un waypoint de type "stop".
	// L'arrivée doit avoir été confirmée au préalable (actual_arrival_datetime IS NOT NULL).
	// Retourne ErrorWaypointNotFound, ErrorUnauthorized, ErrorTripNotInProgress,
	// ErrorWaypointNotAStop, ErrorWaypointNotArrived, ErrorWaypointAlreadyDeparted selon le cas.
	ConfirmWaypointDeparture(ctx context.Context, waypointID, driverID string) error

	// UpdateAvailableSeats met à jour le nombre de places disponibles d'un trajet.
	// Utilisé par le job de réconciliation du booking-service.
	UpdateAvailableSeats(ctx context.Context, tripID string, newAvailableSeats int16) error

	// CancelTrip annule un trajet planifié (scheduled → cancelled).
	// Définit canceller_id, cancellation_reason et le statut à "cancelled".
	// Retourne ErrorTripNotFound si le trajet n'existe pas, ErrorUnauthorized si le conducteur
	// n'est pas propriétaire du trajet, ErrorTripNotScheduled si le statut n'est pas "scheduled".
	CancelTrip(ctx context.Context, tripID, driverID, reason string) error

	// CancelWaypoint annule un waypoint de type "stop" d'un trajet planifié (soft-delete).
	// Définit cancelled_at = NOW() et cancellation_reason sur le waypoint.
	// Retourne ErrorWaypointNotFound si le waypoint n'existe pas, ErrorUnauthorized si le conducteur
	// n'est pas propriétaire du trajet, ErrorTripNotScheduled si le statut n'est pas "scheduled",
	// ErrorWaypointNotAStop si le waypoint n'est pas de type "stop",
	// ErrorWaypointAlreadyCancelled si le waypoint est déjà annulé.
	CancelWaypoint(ctx context.Context, waypointID, driverID, reason string) error

	// IncrementLegBookedSeats incrémente booked_seats sur les waypoints du segment [fromOrder, toOrder).
	// delta peut être positif (réservation) ou négatif (annulation).
	IncrementLegBookedSeats(ctx context.Context, tripID string, fromOrder, toOrder int, delta int) error

	// SyncLegBookedSeats force la valeur de booked_seats pour chaque leg et met à jour
	// t.available_seats (cache dénormalisé) dans une transaction unique.
	SyncLegBookedSeats(ctx context.Context, tripID string, legs []LegBookedSeats) error
}

// LegBookedSeats contient le nombre de places réservées pour un leg donné.
type LegBookedSeats struct {
	SequencerOrder int
	BookedSeats    int
}
