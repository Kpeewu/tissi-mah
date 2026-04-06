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
	// Retourne la liste des passengerIDs des bookings démarrés.
	StartBookingsForWaypoint(ctx context.Context, tripID, waypointID string) ([]string, error)

	// CompleteBookingsForWaypoint complète les réservations inProgress d'un waypoint (dropoff).
	// Retourne la liste des passengerIDs des bookings complétés.
	CompleteBookingsForWaypoint(ctx context.Context, tripID, waypointID string) ([]string, error)

	// ReportNoShow signale l'absence d'un passager ou d'un conducteur.
	ReportNoShow(ctx context.Context, bookingID, reporterID, noShowType, description string) error

	// ConfirmPayment confirme le paiement d'une réservation.
	// Retourne le nouveau statut et autoApprove du trajet associé.
	ConfirmPayment(ctx context.Context, bookingID, transactionID string) error

	// FailPayment marque le paiement d'une réservation comme échoué (paymentPending → paymentFailed).
	FailPayment(ctx context.Context, bookingID, reason string) error

	// CancelBookingsForTrip annule toutes les réservations actives d'un trajet annulé.
	// Retourne la liste des bookings annulés pour restaurer les places et évaluer les remboursements.
	CancelBookingsForTrip(ctx context.Context, tripID string) ([]*domain.Booking, error)

	// CancelBookingsForWaypoint annule les réservations actives d'un waypoint supprimé.
	// Retourne la liste des bookings annulés pour restaurer les places et évaluer les remboursements.
	CancelBookingsForWaypoint(ctx context.Context, tripID, waypointID string) ([]*domain.Booking, error)

	// MarkPaymentReleased marque le paiement d'un booking comme libéré.
	MarkPaymentReleased(ctx context.Context, bookingID string) error
}
