package interfaces

import (
	"context"
	"time"

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

	// ContestNoShow enregistre la contestation d'un no-show, si elle intervient dans contestWindow
	// après la déclaration et avant toute demande de remboursement.
	ContestNoShow(ctx context.Context, bookingID, contesterID, reason string, contestWindow time.Duration) error

	// ResolveNoShowContest enregistre la décision du support sur un no-show contesté et pose le
	// motif de remboursement retenu (réservation payée en ligne uniquement).
	ResolveNoShowContest(ctx context.Context, bookingID, supportUID, refundReason, note string) error

	// ConfirmPayment confirme le paiement d'une réservation.
	// Retourne le nouveau statut et autoApprove du trajet associé.
	// expectedStatus → newStatus (paymentPending, ou paymentFailed pour une réactivation après approbation tardive).
	ConfirmPayment(ctx context.Context, bookingID, transactionID string, expectedStatus, newStatus domain.BookingStatus) error

	// FailPayment marque le paiement d'une réservation comme échoué (paymentPending → paymentFailed).
	FailPayment(ctx context.Context, bookingID, reason string) error

	// CancelBookingsForTrip annule toutes les réservations actives d'un trajet annulé.
	// Retourne la liste des bookings annulés (statut d'avant annulation + données de paiement) pour
	// restaurer les places, supprimer les collectes en cours et évaluer les remboursements.
	CancelBookingsForTrip(ctx context.Context, tripID string) ([]*domain.Booking, error)

	// CancelBookingsForWaypoint annule les réservations actives d'un waypoint supprimé.
	// Retourne la liste des bookings annulés (statut d'avant annulation + données de paiement) pour
	// restaurer les places, supprimer les collectes en cours et évaluer les remboursements.
	CancelBookingsForWaypoint(ctx context.Context, tripID, waypointID string) ([]*domain.Booking, error)

	// MarkPaymentReleased marque le paiement d'un booking comme libéré.
	MarkPaymentReleased(ctx context.Context, bookingID string) error

	// MarkRefundRequested enregistre l'acquittement de la demande de remboursement par payment-service.
	MarkRefundRequested(ctx context.Context, bookingID string) error

	// IncrementRefundRequestAttempts compte un refus définitif de payment-service et retourne le total.
	IncrementRefundRequestAttempts(ctx context.Context, bookingID string) (int, error)

	// AnonymizeUserRefs pseudonymise passenger_id, driver_id, canceller_id et no_show_reported_by
	// dans bookings, et changed_by dans bookings_status_history pour l'utilisateur donné.
	AnonymizeUserRefs(ctx context.Context, userID string) error
}
