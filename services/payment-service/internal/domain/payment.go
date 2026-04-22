package domain

import "time"

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
	PassengerPhoneNumber  string // Numéro utilisé au paiement, réutilisé pour le refund FedaPay.
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

const (
	RefundReasonCancelledByDriver    RefundReason = "cancelledByDriver"
	RefundReasonCancelledByPassenger RefundReason = "cancelledByPassenger"
	RefundReasonNoShowDriver         RefundReason = "noShowDriver"
	RefundReasonNoShowPassenger      RefundReason = "noShowPassenger"
	RefundReasonTripCancelled        RefundReason = "tripCancelled"
	RefundReasonBookingRejected      RefundReason = "bookingRejected"
	RefundReasonDispute              RefundReason = "dispute"
	RefundReasonOther                RefundReason = "other"
)

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
