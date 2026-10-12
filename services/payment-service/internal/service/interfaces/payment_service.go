package interfaces

import "context"

// PaymentService définit l'interface du service de paiement.
type PaymentService interface {
	// CreatePayment crée la collecte FedaPay (paiement avec redirection) et retourne le lien de paiement.
	// Réutilise le paiement pending existant de la réservation s'il y en a un.
	CreatePayment(ctx context.Context, input *CreatePaymentInput) (*CreatePaymentResult, error)
	// VerifyPayment resynchronise un paiement avec FedaPay (appelé par le mobile au retour de la page de paiement).
	VerifyPayment(ctx context.Context, paymentID string) (*PaymentSyncResult, error)
	// CancelPayment abandonne un paiement pending (suppression FedaPay + libération des places).
	CancelPayment(ctx context.Context, paymentID string) (*PaymentSyncResult, error)
	// HandlePaymentCallback traite le retour de la page FedaPay (statut revérifié auprès de FedaPay).
	HandlePaymentCallback(ctx context.Context, fedapayTransactionID string) (*PaymentSyncResult, error)
	// CancelPaymentForBooking annule le paiement pending d'une réservation annulée (interne, booking-service).
	CancelPaymentForBooking(ctx context.Context, bookingID string, reason string) (*PaymentSyncResult, error)
	GetPaymentStatus(ctx context.Context, paymentID string) (*PaymentStatusResult, error)
	GetPaymentByBooking(ctx context.Context, bookingID string) (*PaymentByBookingResult, error)
	// GetPaymentByBookingInternal retourne le paiement d'une réservation sans contrôle d'appartenance
	// (interne, booking-service).
	GetPaymentByBookingInternal(ctx context.Context, bookingID string) (*PaymentByBookingResult, error)
	ProcessWebhook(ctx context.Context, input *ProcessWebhookInput) error
	RequestRefund(ctx context.Context, input *RequestRefundInput) (*RequestRefundResult, error)
	GetRefundStatus(ctx context.Context, refundID string) (*RefundStatusResult, error)
	// GetRefundByBooking retourne le remboursement le plus récent d'une réservation (ErrorRefundNotFound sinon).
	GetRefundByBooking(ctx context.Context, bookingID string) (*RefundStatusResult, error)
	ReleasePayment(ctx context.Context, bookingID string) error
	GetPayoutStatus(ctx context.Context, payoutID string) (*PayoutStatusResult, error)
	GetDriverPayouts(ctx context.Context, driverID string, pageIndex int) ([]*PayoutPreviewResult, error)
	TriggerManualPayout(ctx context.Context, tripID string, supportUserID string) (int, error)

	// GetPaymentHistory retourne l'historique d'un paiement (paymentID) ou de tous les paiements
	// d'une réservation (bookingID), du plus ancien au plus récent. Réservé au support.
	GetPaymentHistory(ctx context.Context, paymentID string, bookingID string) ([]*PaymentHistoryEntryResult, error)
	// GetRefundHistory retourne l'historique d'un remboursement (refundID) ou des remboursements
	// d'une réservation (bookingID). Réservé au support.
	GetRefundHistory(ctx context.Context, refundID string, bookingID string) ([]*RefundHistoryEntryResult, error)
	// GetPayoutHistory retourne l'historique d'un payout chauffeur. Réservé au support.
	GetPayoutHistory(ctx context.Context, payoutID string) ([]*PayoutHistoryEntryResult, error)

	// CheckDeletionEligibility vérifie si un utilisateur peut supprimer son compte côté payment-service.
	// Bloquant si le chauffeur a un payout en attente (pending/scheduled/processing).
	CheckDeletionEligibility(ctx context.Context, userID string) (bool, string, error)

	// AnonymizeUserData anonymise les données liées à l'utilisateur dans payment-service.
	// bookingIDs contient les IDs des réservations du passager pour anonymiser passenger_phone_number.
	AnonymizeUserData(ctx context.Context, userID string, bookingIDs []string) error
}

// =============================================================================
// DTOs
// =============================================================================

type CreatePaymentInput struct {
	BookingID            string
	TripID               string
	PaymentMethod        string
	Amount               int    // déprécié : ignoré, le montant est celui de la réservation
	PassengerPhoneNumber string // numéro togocel de remboursement
	MobileMoneyMode      string // déprécié : ignoré, l'opérateur est choisi sur la page FedaPay
}

type CreatePaymentResult struct {
	PaymentID        string
	PaymentReference string
	Status           string
	ErrorMessage     string
	PaymentURL       string
	ExpiresAt        string // RFC3339
	Reused           bool
	Amount           int
}

// PaymentSyncResult est l'état d'un paiement (et de sa réservation) après synchronisation avec FedaPay.
type PaymentSyncResult struct {
	PaymentID      string
	BookingID      string
	Status         string
	ProviderStatus string
	IsFinal        bool
	PaymentURL     string
	ExpiresAt      string // RFC3339
	BookingStatus  string
	SeatsReleased  bool
	Amount         int
}

type ProcessWebhookInput struct {
	Signature  string
	RawPayload []byte
}

type PaymentStatusResult struct {
	PaymentID        string
	BookingID        string
	Amount           int
	PaymentMethod    string
	Status           string
	PaymentReference string
	CreatedAt        string
	CompletedAt      string
	PaymentURL       string
	ProviderStatus   string
	ExpiresAt        string
}

type PaymentByBookingResult struct {
	PaymentID        string
	BookingID        string
	Amount           int
	PaymentMethod    string
	Status           string
	PaymentReference string
	CreatedAt        string
	CompletedAt      string // RFC3339, vide tant que le paiement n'est pas confirmé
	PaymentURL       string
	ProviderStatus   string
	ExpiresAt        string
}

type RequestRefundInput struct {
	BookingID         string
	RefundReason      string
	OriginalAmount    int
	ServiceFee        int
	DepartureDatetime string
	ApprovedAt        string
	CancelledAt       string
}

type RequestRefundResult struct {
	RefundID        string
	RefundReference string
	Status          string
	RefundAmount    int
}

type RefundStatusResult struct {
	RefundID          string
	BookingID         string
	RefundReason      string
	RefundRule        string
	OriginalAmount    int
	RefundPercentage  int
	RefundAmount      int
	AmountToPassenger int
	AmountToDriver    int
	AmountToPlatform  int
	Status            string
	ProcessedAt       string
	CompletedAt       string
}

type PayoutStatusResult struct {
	PayoutID      string
	DriverID      string
	TripID        string
	GrossAmount   int
	PlatformFee   int
	NetAmount     int
	Status        string
	ScheduledAt   string
	CompletedAt   string
	FailureReason string
}

type PayoutPreviewResult struct {
	PayoutID        string
	PayoutReference string
	TripID          string
	NetAmount       int
	Status          string
	CompletedAt     string
	CreatedAt       string
}

// PaymentHistoryEntryResult est une entrée de payment_status_history.
type PaymentHistoryEntryResult struct {
	HistoryID      string
	PaymentID      string
	Status         string
	ProviderStatus string
	InitiatedBy    string
	Source         string
	OccurredAt     string // RFC3339
	Notes          string
}

// RefundHistoryEntryResult est une entrée de refund_status_history.
type RefundHistoryEntryResult struct {
	HistoryID   string
	RefundID    string
	Status      string
	RetryCount  int
	InitiatedBy string
	Source      string
	OccurredAt  string // RFC3339
	Notes       string
}

// PayoutHistoryEntryResult est une entrée de payout_status_history.
type PayoutHistoryEntryResult struct {
	HistoryID        string
	PayoutID         string
	Status           string
	InitiatedBy      string
	SupportUserID    string
	SupportFirstName string
	SupportLastName  string
	OccurredAt       string // RFC3339
	Notes            string
}
