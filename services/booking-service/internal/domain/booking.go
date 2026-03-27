package domain

import "time"

// BookingStatus représente le statut d'une réservation.
type BookingStatus string

const (
	BookingStatusCreated         BookingStatus = "created"
	BookingStatusPaymentPending  BookingStatus = "paymentPending"
	BookingStatusPendingApproval BookingStatus = "pendingApproval"
	BookingStatusApproved        BookingStatus = "approved"
	BookingStatusRejected        BookingStatus = "rejected"
	BookingStatusCancelled       BookingStatus = "cancelled"
	BookingStatusInProgress      BookingStatus = "inProgress"
	BookingStatusCompleted       BookingStatus = "completed"
	BookingStatusNoShow          BookingStatus = "noShow"
	BookingStatusExpired         BookingStatus = "expired"
)

// PaymentMethod représente le mode de paiement d'une réservation.
type PaymentMethod string

const (
	PaymentMobileMoney PaymentMethod = "mobileMoney"
	PaymentCard        PaymentMethod = "card"
	PaymentPaypal      PaymentMethod = "paypal"
	PaymentCash        PaymentMethod = "cash"
)

// Booking représente une réservation dans le domaine métier.
type Booking struct {
	BookingID          string
	BookingReference   string
	TripID             string
	PassengerID        string
	DriverID           string
	PickupWaypointID   string
	DropoffWaypointID  string
	SeatsBooked        int16
	PricePerSeat       int
	Subtotal           int
	ServiceFee         int
	TotalAmount        int
	PaymentMethod      PaymentMethod
	Status             BookingStatus
	PaymentCompletedAt *time.Time
	ApprovedAt         *time.Time
	RejectedAt         *time.Time
	CancelledAt        *time.Time
	CompletedAt        *time.Time
	CancellerID        *string
	CancellationReason *string
	NoShowType         *string
	NoShowReportedBy   *string
	NoShowReportedAt   *time.Time
	NoShowDescription  *string
	PaymentReleasedAt  *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
