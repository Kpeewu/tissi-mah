package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/domain"
)

// BookingRepositoryWrite définit les opérations d'écriture sur les tables bookings.
type BookingRepositoryWrite interface {
	// Create insère une réservation avec ses segments et une entrée d'historique dans une transaction unique.
	Create(ctx context.Context, booking *domain.Booking, segments []*domain.Segment, history *domain.StatusHistoryEntry) error

	// Approve approuve une réservation pendingApproval.
	Approve(ctx context.Context, bookingID, driverID string) error

	// Reject rejette une réservation pendingApproval.
	Reject(ctx context.Context, bookingID, driverID, reason string) error

	// Cancel annule une réservation (passager ou conducteur).
	Cancel(ctx context.Context, bookingID, cancellerID, reason string) error

	// StartBookingsForWaypoint démarre les réservations approved d'un waypoint (pickup).
	// Retourne le nombre de bookings démarrés.
	StartBookingsForWaypoint(ctx context.Context, tripID, waypointID string) (int, error)

	// CompleteBookingsForWaypoint complète les réservations inProgress d'un waypoint (dropoff).
	// Retourne le nombre de bookings complétés.
	CompleteBookingsForWaypoint(ctx context.Context, tripID, waypointID string) (int, error)

	// ReportNoShow signale l'absence d'un passager ou d'un conducteur.
	ReportNoShow(ctx context.Context, bookingID, reporterID, noShowType, description string) error

	// ConfirmPayment confirme le paiement d'une réservation.
	// Retourne le nouveau statut et autoApprove du trajet associé.
	ConfirmPayment(ctx context.Context, bookingID, transactionID string) error
}
