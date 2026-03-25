package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/domain"
)

// BookingRepositoryRead définit les opérations de lecture sur les tables bookings.
type BookingRepositoryRead interface {
	// GetByID retourne une réservation par son ID (sans segments ni historique).
	GetByID(ctx context.Context, bookingID string) (*domain.Booking, error)

	// GetByIDWithDetails retourne une réservation avec ses segments et son historique de statuts.
	GetByIDWithDetails(ctx context.Context, bookingID string) (*domain.Booking, []*domain.Segment, []*domain.StatusHistoryEntry, error)

	// GetPassengerBookings retourne la liste paginée des réservations d'un passager.
	GetPassengerBookings(ctx context.Context, passengerID string, pageIndex int, statusFilter string) ([]*domain.BookingPreview, error)

	// GetDriverTripBookings retourne la liste paginée des réservations pour un trajet du conducteur.
	GetDriverTripBookings(ctx context.Context, driverID, tripID string, pageIndex int) ([]*domain.BookingPreview, error)

	// HasActiveBooking vérifie si un passager a déjà une réservation active pour un trajet.
	HasActiveBooking(ctx context.Context, passengerID, tripID string) (bool, error)

	// GetActiveBookingsSeatsForTrip retourne le total des places réservées pour un trajet
	// parmi les bookings actifs (paymentPending, pendingApproval, approved, inProgress, created).
	GetActiveBookingsSeatsForTrip(ctx context.Context, tripID string) (int, error)

	// GetActiveTripsWithBookings retourne la liste des tripIDs ayant des bookings actifs.
	GetActiveTripsWithBookings(ctx context.Context) ([]string, error)
}
