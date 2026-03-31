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

	// GetDriverTripBookings retourne la liste paginée des réservations d'un trajet du conducteur.
	GetDriverTripBookings(ctx context.Context, input *GetDriverTripBookingsInput) ([]*BookingPreviewResult, error)

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
	PassengerID       string
	TripID            string
	PickupWaypointID  string
	DropoffWaypointID string
	SeatsBooked       int
	PaymentMethod     string
	Segments          []SegmentInput
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
