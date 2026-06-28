package interfaces

import "context"

// BookingService définit le contrat métier pour la gestion des réservations.
type BookingService interface {
	// CreateBooking crée une nouvelle réservation.
	CreateBooking(ctx context.Context, input *CreateBookingInput) (*CreateBookingResult, error)

	// GetBookingDetails retourne les détails complets d'une réservation.
	GetBookingDetails(ctx context.Context, input *GetBookingDetailsInput) (*BookingDetailResult, error)

	// GetPassengerBookings retourne la liste paginée des réservations d'un passager.
	GetPassengerBookings(ctx context.Context, input *GetPassengerBookingsInput) ([]*BookingPreviewResult, error)

	// GetDriverTripBookings retourne les réservations d'un trajet du conducteur (enrichies + compteurs).
	GetDriverTripBookings(ctx context.Context, input *GetDriverTripBookingsInput) (*GetDriverTripBookingsResult, error)

	// GetDriverPendingBookings retourne la liste agrégée paginée des demandes en attente du conducteur tous trajets confondus.
	GetDriverPendingBookings(ctx context.Context, input *GetDriverPendingBookingsInput) ([]*DriverBookingPreviewResult, error)

	// GetActivePassengerSummariesForTrip retourne les passagers actifs d'un trajet enrichis de leurs informations.
	GetActivePassengerSummariesForTrip(ctx context.Context, tripID string) ([]*PassengerSummaryResult, error)

	// ApproveBooking approuve une réservation.
	ApproveBooking(ctx context.Context, input *ApproveBookingInput) error

	// RejectBooking rejette une réservation.
	RejectBooking(ctx context.Context, input *RejectBookingInput) error

	// CancelBooking annule une réservation.
	CancelBooking(ctx context.Context, input *CancelBookingInput) error

	// StartBookingsForWaypoint démarre les réservations approved d'un waypoint.
	StartBookingsForWaypoint(ctx context.Context, input *StartBookingsForWaypointInput) (int, error)

	// CompleteBookingsForWaypoint complète les réservations inProgress d'un waypoint.
	CompleteBookingsForWaypoint(ctx context.Context, input *CompleteBookingsForWaypointInput) (int, error)

	// ReportNoShow signale l'absence d'un passager ou d'un conducteur.
	ReportNoShow(ctx context.Context, input *ReportNoShowInput) error

	// ConfirmPayment confirme le paiement d'une réservation.
	ConfirmPayment(ctx context.Context, input *ConfirmPaymentInput) error

	// FailPayment signale l'échec du paiement et restaure les places.
	FailPayment(ctx context.Context, input *FailPaymentInput) error

	// CancelBookingsForWaypoint annule les réservations actives d'un waypoint supprimé.
	CancelBookingsForWaypoint(ctx context.Context, input *CancelBookingsForWaypointInput) (int, error)

	// CancelBookingsForTrip annule toutes les réservations actives d'un trajet annulé.
	CancelBookingsForTrip(ctx context.Context, input *CancelBookingsForTripInput) (int, error)

	// GetActivePassengerIDsForTrip retourne les IDs distincts des passagers avec une réservation active.
	GetActivePassengerIDsForTrip(ctx context.Context, tripID string) ([]string, error)

	// CheckDeletionEligibility vérifie si un utilisateur peut supprimer son compte côté booking-service.
	CheckDeletionEligibility(ctx context.Context, userID string) (bool, string, error)

	// AnonymizeUserData pseudonymise les références de l'utilisateur dans booking-service.
	AnonymizeUserData(ctx context.Context, userID string) error

	// GetPassengerBookingIDs retourne tous les IDs de réservation d'un passager.
	GetPassengerBookingIDs(ctx context.Context, passengerID string) ([]string, error)

	// ListBookingsAdmin retourne la liste paginée et filtrée des réservations (vue support),
	// enrichie des noms passager/conducteur, + le total filtré.
	ListBookingsAdmin(ctx context.Context, input *ListBookingsAdminInput) (*ListBookingsAdminResult, error)

	// GetBookingDetailAdmin retourne le détail complet d'une réservation sans contrôle
	// d'appartenance (vue support), enrichi des noms passager/conducteur.
	GetBookingDetailAdmin(ctx context.Context, bookingID string) (*BookingDetailAdminResult, error)
}

// =============================================================================
// Input DTOs
// =============================================================================

type SegmentInput struct {
	PickupWaypointID       string
	DropoffWaypointID      string
	PickupLocationName     string
	PickupCity             string
	PickupLat              float64
	PickupLng              float64
	PickupScheduledAt      string
	DropoffLocationName    string
	DropoffCity            string
	DropoffLat             float64
	DropoffLng             float64
	DropoffScheduledAt     string
	SegmentDistanceMeters  int
	SegmentDurationMinutes int
	SegmentPrice           int
}

type CreateBookingInput struct {
	PassengerID        string
	TripID             string
	PickupWaypointID   string
	DropoffWaypointID  string
	SeatsBooked        int
	PaymentMethod      string
	Segments           []SegmentInput
	PassengerMessage   string
	ExtraMinutesDetour int
}

type GetBookingDetailsInput struct {
	BookingID string
	UserID    string
}

type GetPassengerBookingsInput struct {
	PassengerID  string
	PageIndex    int
	StatusFilter string
}

type GetDriverTripBookingsInput struct {
	DriverID  string
	TripID    string
	PageIndex int
}

type GetDriverPendingBookingsInput struct {
	DriverID  string
	PageIndex int
}

type ApproveBookingInput struct {
	DriverID  string
	BookingID string
}

type RejectBookingInput struct {
	DriverID  string
	BookingID string
	Reason    string
}

type CancelBookingInput struct {
	UserID    string
	BookingID string
	Reason    string
}

type StartBookingsForWaypointInput struct {
	TripID     string
	WaypointID string
}

type CompleteBookingsForWaypointInput struct {
	TripID     string
	WaypointID string
}

type ReportNoShowInput struct {
	BookingID   string
	ReporterID  string
	NoShowType  string
	Description string
}

type ConfirmPaymentInput struct {
	BookingID     string
	TransactionID string
}

type FailPaymentInput struct {
	BookingID string
	Reason    string
}

type CancelBookingsForWaypointInput struct {
	TripID     string
	WaypointID string
}

type CancelBookingsForTripInput struct {
	TripID string
}

// =============================================================================
// Result DTOs
// =============================================================================

type CreateBookingResult struct {
	BookingID        string
	BookingReference string
	Status           string
	TotalAmount      int
}

type SegmentDetailResult struct {
	SegmentID              string
	PickupWaypointID       string
	DropoffWaypointID      string
	PickupLocationName     string
	PickupCity             string
	PickupLat              float64
	PickupLng              float64
	PickupScheduledAt      string
	PickupActualAt         string
	DropoffLocationName    string
	DropoffCity            string
	DropoffLat             float64
	DropoffLng             float64
	DropoffScheduledAt     string
	DropoffActualAt        string
	SegmentDistanceMeters  int
	SegmentDurationMinutes int
	SegmentPrice           int
}

type StatusHistoryResult struct {
	HistoryID      string
	PreviousStatus string
	NewStatus      string
	ChangedBy      string
	ChangedByName  string // nom lisible de l'auteur (vide pour les transitions système)
	ChangedByType  string
	ChangeReason   string
	Metadata       string
	CreatedAt      string
}

type BookingDetailResult struct {
	BookingID          string
	BookingReference   string
	TripID             string
	PassengerID        string
	DriverID           string
	PickupWaypointID   string
	DropoffWaypointID  string
	SeatsBooked        int
	PricePerSeat       int
	Subtotal           int
	ServiceFee         int
	TotalAmount        int
	PaymentMethod      string
	Status             string
	PaymentCompletedAt string
	ApprovedAt         string
	RejectedAt         string
	CancelledAt        string
	CompletedAt        string
	CancellerID        string
	CancellationReason string
	NoShowType         string
	NoShowReportedBy   string
	NoShowReportedAt   string
	NoShowDescription  string
	CreatedAt          string
	UpdatedAt          string
	Segments           []SegmentDetailResult
	History            []StatusHistoryResult
	PassengerMessage   string
	RoutePolyline      string // Google encoded polyline du trajet (vide si indisponible)
}

type BookingPreviewResult struct {
	BookingID           string
	BookingReference    string
	TripID              string
	Status              string
	SeatsBooked         int
	TotalAmount         int
	PickupLocationName  string
	DropoffLocationName string
	DepartureDate       string
	DepartureTime       string
}

// DriverBookingPreviewResult est la vue enrichie d'une réservation pour le conducteur.
type DriverBookingPreviewResult struct {
	BookingID           string
	BookingReference    string
	TripID              string
	Status              string
	SeatsBooked         int
	TotalAmount         int
	PickupLocationName  string
	DropoffLocationName string
	DepartureDate       string
	DepartureTime       string
	PassengerName       string
	PassengerRating     float64
	PassengerTripCount  int
	IsPassengerVerified bool
	PassengerMessage    string
	PaymentMethod       string
	CreatedAt           string
	ExtraMinutesDetour  int
}

// BookingCountsResult regroupe les compteurs de réservations par statut.
type BookingCountsResult struct {
	Pending   int32
	Approved  int32
	Rejected  int32
	Cancelled int32
}

// GetDriverTripBookingsResult contient la réponse enrichie de GetDriverTripBookings.
type GetDriverTripBookingsResult struct {
	Bookings       []*BookingPreviewResult
	DriverBookings []*DriverBookingPreviewResult
	Counts         *BookingCountsResult
}

// PassengerSummaryResult est la vue enrichie d'un passager actif pour le conducteur.
type PassengerSummaryResult struct {
	PassengerID   string
	PassengerName string
	SeatsBooked   int
	PaymentMethod string
	PaymentStatus string
	Rating        float64
	IsVerified    bool
	BookingID     string
}

// ListBookingsAdminInput regroupe les filtres + pagination de la vue support.
// DateFrom/DateTo sont au format RFC3339 (vides = pas de borne).
type ListBookingsAdminInput struct {
	Status           string
	BookingReference string
	DateFrom         string
	DateTo           string
	PageIndex        int
	PageSize         int
}

// AdminBookingPreviewResult est la vue support enrichie d'une réservation.
type AdminBookingPreviewResult struct {
	BookingID           string
	BookingReference    string
	TripID              string
	PassengerID         string
	DriverID            string
	PassengerName       string
	DriverName          string
	Status              string
	SeatsBooked         int
	TotalAmount         int
	PaymentMethod       string
	PickupLocationName  string
	DropoffLocationName string
	DepartureDatetime   string
	CreatedAt           string
}

// ListBookingsAdminResult contient la page enrichie + le total filtré.
type ListBookingsAdminResult struct {
	Bookings []*AdminBookingPreviewResult
	Total    int
}

// BookingDetailAdminResult est le détail support d'une réservation enrichi des noms.
type BookingDetailAdminResult struct {
	Booking       *BookingDetailResult
	PassengerName string
	DriverName    string
}
