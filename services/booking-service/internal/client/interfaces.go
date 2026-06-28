package client

import "context"

// TripClient définit le contrat pour appeler trips-service depuis booking-service.
type TripClient interface {
	// GetTripDetails retourne les détails d'un trajet nécessaires à la création d'une réservation.
	GetTripDetails(ctx context.Context, tripID string) (*TripDetails, error)
	// UpdateAvailableSeats met à jour le nombre de places disponibles d'un trajet (réconciliation).
	UpdateAvailableSeats(ctx context.Context, tripID string, newAvailableSeats int) error
	// IncrementLegBookedSeats incrémente/décrémente booked_seats sur les legs [fromOrder, toOrder).
	IncrementLegBookedSeats(ctx context.Context, tripID string, fromOrder, toOrder, delta int) error
	// SyncLegBookedSeats force booked_seats par leg (réconciliation).
	SyncLegBookedSeats(ctx context.Context, tripID string, legs []LegBookedSeats) error
	Close() error
}

// LegBookedSeats contient le nombre de places réservées pour un leg donné.
type LegBookedSeats struct {
	SequencerOrder int
	BookedSeats    int
}

// TripDetails contient les informations d'un trajet nécessaires au booking-service.
type TripDetails struct {
	TripID             string
	DriverID           string
	Status             string
	AvailableSeats     int
	TotalSeats         int
	PricePerSeat       int
	AutoApproveEnabled bool
	RoutePolyline      string
	Waypoints          []TripWaypoint
}

// TripWaypoint contient les informations d'un waypoint.
type TripWaypoint struct {
	WaypointID              string
	WaypointType            string
	SequencerOrder          int
	LocationName            string
	City                    string
	ScheduledPickupDatetime string
	PriceFromPrevious       int
}

// UserClient définit le contrat pour appeler user-service depuis booking-service.
type UserClient interface {
	// UserExists vérifie qu'un utilisateur existe.
	UserExists(ctx context.Context, userID string) (bool, error)
	// IsPassengerVerified vérifie qu'un utilisateur existe et que son profil passager est vérifié.
	IsPassengerVerified(ctx context.Context, userID string) (bool, error)
	// GetPassengerInfo retourne le nom complet et le statut de vérification d'un passager en un seul appel.
	GetPassengerInfo(ctx context.Context, userID string) (fullName string, isVerified bool, err error)
	Close() error
}

// RatingClient définit le contrat pour appeler rating-service depuis booking-service.
type RatingClient interface {
	// GetUserRatingsAverage retourne la moyenne et le nombre total de notes d'un utilisateur.
	// Retourne (0, 0, nil) si l'utilisateur n'a pas encore de notes.
	GetUserRatingsAverage(ctx context.Context, userID string) (average float64, totalRatings int32, err error)
	Close() error
}

// PaymentClient définit le contrat pour appeler payment-service depuis booking-service.
type PaymentClient interface {
	// RequestRefund demande un remboursement au payment-service.
	RequestRefund(ctx context.Context, input *RefundInput) error
	// ReleasePayment libère un paiement held → released après le délai de contestation.
	ReleasePayment(ctx context.Context, bookingID string) error
	Close() error
}

// RefundInput contient les données nécessaires pour demander un remboursement.
type RefundInput struct {
	BookingID         string
	RefundReason      string // cancelledByDriver | cancelledByPassenger | noShowDriver | noShowPassenger | bookingRejected
	OriginalAmount    int    // Subtotal (hors frais de service)
	ServiceFee        int
	DepartureDatetime string // RFC3339
	ApprovedAt        string // RFC3339, optionnel
	CancelledAt       string // RFC3339
}
