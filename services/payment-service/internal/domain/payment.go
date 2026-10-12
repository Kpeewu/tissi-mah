package domain

import (
	"slices"
	"time"
)

// =============================================================================
// Payment
// =============================================================================

type PaymentStatus string

const (
	PaymentStatusPending  PaymentStatus = "pending"
	PaymentStatusHeld     PaymentStatus = "held"
	PaymentStatusReleased PaymentStatus = "released"
	PaymentStatusPaidOut  PaymentStatus = "paidOut"
	PaymentStatusFailed   PaymentStatus = "failed"
	PaymentStatusRefunded PaymentStatus = "refunded"
)

type PaymentMethod string

const (
	PaymentMobileMoney PaymentMethod = "mobileMoney"
	PaymentCard        PaymentMethod = "card"
	PaymentPaypal      PaymentMethod = "paypal"
)

type Payment struct {
	PaymentID             string
	BookingID             string
	TripID                string
	Amount                int
	PaymentMethod         PaymentMethod
	PaymentProvider       string
	Status                PaymentStatus
	ExternalTransactionID string
	PaymentReference      string
	PassengerPhoneNumber  string // Numéro togocel de remboursement fourni par le passager (le paiement se fait sur la page FedaPay).
	PaymentURL            string // Lien de la page de paiement FedaPay (redirection)
	ProviderStatus        string // Dernier statut FedaPay observé (pending, approved, declined, canceled, expired, deleted)
	ExpiresAt             time.Time
	BookingSyncAttempts   int // Reprises du rattachement à la réservation échouées (remboursement au plafond)
	CreatedAt             time.Time
	CompletedAt           *time.Time
	FailedAt              *time.Time
	FailureReason         *string
	Metadata              *string // JSONB
	UpdatedAt             time.Time
}

// =============================================================================
// Refund
// =============================================================================

type RefundReason string

// Toute nouvelle valeur doit aussi être ajoutée à refundReasons et à l'enum SQL refund_reason (migration).
const (
	RefundReasonCancelledByDriver    RefundReason = "cancelledByDriver"
	RefundReasonCancelledByPassenger RefundReason = "cancelledByPassenger"
	RefundReasonNoShowDriver         RefundReason = "noShowDriver"
	RefundReasonNoShowPassenger      RefundReason = "noShowPassenger"
	RefundReasonTripCancelled        RefundReason = "tripCancelled"
	RefundReasonWaypointCancelled    RefundReason = "waypointCancelled" // arrêt du passager supprimé par le chauffeur
	RefundReasonBookingRejected      RefundReason = "bookingRejected"
	RefundReasonDispute              RefundReason = "dispute"
	RefundReasonSeatsUnavailable     RefundReason = "seatsUnavailable"   // approuvé après libération, trajet complet
	RefundReasonBookingUnavailable   RefundReason = "bookingUnavailable" // approuvé, réservation annulée/rejetée
	RefundReasonOther                RefundReason = "other"
)

// refundReasons liste les valeurs de l'enum SQL refund_reason (chaque valeur est vérifiée
// contre la base migrée par les tests d'intégration).
var refundReasons = []RefundReason{
	RefundReasonCancelledByDriver,
	RefundReasonCancelledByPassenger,
	RefundReasonNoShowDriver,
	RefundReasonNoShowPassenger,
	RefundReasonTripCancelled,
	RefundReasonWaypointCancelled,
	RefundReasonBookingRejected,
	RefundReasonDispute,
	RefundReasonSeatsUnavailable,
	RefundReasonBookingUnavailable,
	RefundReasonOther,
}

// RefundReasons retourne tous les motifs de remboursement connus.
func RefundReasons() []RefundReason {
	return slices.Clone(refundReasons)
}

// IsValid indique si le motif existe dans l'enum refund_reason.
func (r RefundReason) IsValid() bool {
	return slices.Contains(refundReasons, r)
}

type RefundRule string

const (
	RefundRuleCancelled24hBefore            RefundRule = "cancelled24hBefore"
	RefundRuleCancelled30minAfterApproval   RefundRule = "cancelled30minAfterApproval"
	RefundRuleCancelledOver30minAfterApproval RefundRule = "cancelledOver30minAfterApproval"
	RefundRuleDriverCancellation            RefundRule = "driverCancellation"
	RefundRuleNoShowDriver                  RefundRule = "noShowDriver"
	RefundRuleNoShowPassenger               RefundRule = "noShowPassenger"
)

type RefundStatus string

const (
	RefundStatusPending    RefundStatus = "pending"
	RefundStatusProcessing RefundStatus = "processing"
	RefundStatusCompleted  RefundStatus = "completed"
	RefundStatusFailed     RefundStatus = "failed"
)

type Refund struct {
	RefundID                 string
	RefundReference          string
	PaymentID                string
	BookingID                string
	RefundReason             RefundReason
	RefundRuleApplied        RefundRule
	OriginalAmount           int
	RefundPercentage         int16
	RefundAmount             int
	ServiceFeeRefunded       bool
	AmountToPassenger        int
	AmountToDriver           int
	AmountToPlatform         int
	Status                   RefundStatus
	RefundMethod             string
	ProcessedAt              *time.Time
	CompletedAt              *time.Time
	EstimatedCompletion      *time.Time
	PassengerNotified        bool
	NotificationSentAt       *time.Time
	Notes                    *string
	PayoutDestination        string // Numéro FedaPay destinataire (copie depuis Payment).
	PaymentProvider          string // "fedapay" par défaut.
	PaymentProviderReference string // ID FedaPay du payout de remboursement.
	FailureReason            *string
	RetryCount               int16
	LastRetryAt              *time.Time
	UpdatedAt                time.Time
}

// =============================================================================
// Payout
// =============================================================================

type PayoutStatus string

const (
	PayoutStatusPending    PayoutStatus = "pending"
	PayoutStatusScheduled  PayoutStatus = "scheduled"
	PayoutStatusProcessing PayoutStatus = "processing"
	PayoutStatusCompleted  PayoutStatus = "completed"
	PayoutStatusFailed     PayoutStatus = "failed"
	PayoutStatusCancelled  PayoutStatus = "cancelled"
)

type Payout struct {
	PayoutID                 string
	PayoutReference          string
	DriverID                 string
	TripID                   string
	GrossAmount              int
	PlatformFee              int
	NetAmount                int
	PayoutMethod             string
	PayoutDestination        string
	DestinationName          string
	Status                   PayoutStatus
	ScheduledAt              *time.Time
	CompletedAt              *time.Time
	FailedAt                 *time.Time
	CancelledAt              *time.Time
	PaymentProvider          string
	PaymentProviderReference string
	FailureReason            *string
	RetryCount               int16
	LastRetryAt              *time.Time
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

// =============================================================================
// WebhookEvent
// =============================================================================

type WebhookEvent struct {
	EventID        string
	FedapayEventID string
	EventType      string
	Payload        string // JSONB
	ProcessedAt    *time.Time
	CreatedAt      time.Time
}

// =============================================================================
// PayoutStatusHistory
// =============================================================================

type PayoutStatusHistory struct {
	HistoryID        string
	PayoutID         string
	Status           string // "scheduled" | "processing" | "failed" | "completed" | "launched_by_support"
	InitiatedBy      string // "system" | "support"
	SupportUserID    string
	SupportFirstName string
	SupportLastName  string
	OccurredAt       time.Time
	Notes            string
}

// =============================================================================
// Historique des paiements et remboursements
// =============================================================================

// Événements d'historique sans changement de statut interne (les transitions de statut
// utilisent directement PaymentStatus / RefundStatus).
const (
	PaymentEventAttemptFailed        = "attempt_failed"         // tentative refusée/annulée sur la page FedaPay (paiement toujours pending)
	PaymentEventURLRenewed           = "payment_url_renewed"    // lien de paiement FedaPay régénéré
	PaymentEventBookingSynced        = "booking_synced"         // paiement held rattaché à sa réservation
	PaymentEventBookingSyncFailed    = "booking_sync_failed"    // rattachement à retenter
	PaymentEventBookingSyncAbandoned = "booking_sync_abandoned" // plafond de reprises atteint : remboursement intégral
	PaymentEventRefundCreationFailed = "refund_creation_failed" // remboursement automatique en échec (retenté)

	RefundEventRetryScheduled = "retry_scheduled" // échec récupérable : le refund repasse pending
)

// Acteurs à l'origine d'une entrée d'historique (initiated_by).
const (
	HistoryInitiatedBySystem         = "system"
	HistoryInitiatedByPassenger      = "passenger"
	HistoryInitiatedByBookingService = "booking_service"
	HistoryInitiatedBySupport        = "support"
)

// Canaux à l'origine d'une entrée d'historique (source).
const (
	HistorySourceUnknown             = "unknown"
	HistorySourceCreatePayment       = "create_payment"
	HistorySourceVerifyPayment       = "verify_payment"
	HistorySourceCancelPayment       = "cancel_payment"
	HistorySourceFedaPayCallback     = "fedapay_callback"
	HistorySourceFedaPayWebhook      = "fedapay_webhook"
	HistorySourceBookingService      = "booking_service"
	HistorySourceExpirationWorker    = "expiration_worker"
	HistorySourceBookingSyncWorker   = "booking_sync_worker"
	HistorySourceRefundWorker        = "refund_worker"
	HistorySourcePayoutWorker        = "payout_worker"
	HistorySourceSupportManualPayout = "support_manual_payout"
)

// PaymentStatusHistory est une entrée de payment_status_history.
type PaymentStatusHistory struct {
	HistoryID      string
	PaymentID      string
	Status         string // PaymentStatus atteint ou PaymentEvent*
	ProviderStatus string // statut FedaPay observé (vide si non pertinent)
	InitiatedBy    string
	Source         string
	OccurredAt     time.Time
	Notes          string
}

// RefundStatusHistory est une entrée de refund_status_history.
type RefundStatusHistory struct {
	HistoryID   string
	RefundID    string
	Status      string // RefundStatus atteint ou RefundEventRetryScheduled
	RetryCount  int16
	InitiatedBy string
	Source      string
	OccurredAt  time.Time
	Notes       string
}

// =============================================================================
// PayoutBatch
// =============================================================================

type PayoutBatch struct {
	BatchID         string
	Status          string
	TotalPayouts    int
	SuccessfulCount int
	FailedCount     int
	TotalAmount     int
	StartedAt       *time.Time
	CompletedAt     *time.Time
	CreatedAt       time.Time
}
