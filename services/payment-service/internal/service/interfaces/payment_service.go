package interfaces

import "context"

// PaymentService définit l'interface du service de paiement.
type PaymentService interface {
	CreatePayment(ctx context.Context, input *CreatePaymentInput) (*CreatePaymentResult, error)
	GetPaymentStatus(ctx context.Context, paymentID string) (*PaymentStatusResult, error)
	GetPaymentByBooking(ctx context.Context, bookingID string) (*PaymentByBookingResult, error)
	ProcessWebhook(ctx context.Context, input *ProcessWebhookInput) error
	RequestRefund(ctx context.Context, input *RequestRefundInput) (*RequestRefundResult, error)
	GetRefundStatus(ctx context.Context, refundID string) (*RefundStatusResult, error)
	ReleasePayment(ctx context.Context, bookingID string) error
	GetPayoutStatus(ctx context.Context, payoutID string) (*PayoutStatusResult, error)
	GetDriverPayouts(ctx context.Context, driverID string, pageIndex int) ([]*PayoutPreviewResult, error)
	TriggerManualPayout(ctx context.Context, tripID string, supportUserID string) (int, error)
}

// =============================================================================
// DTOs
// =============================================================================

type CreatePaymentInput struct {
	BookingID           string
	TripID              string
	PaymentMethod       string
	Amount              int
	PassengerPhoneNumber string
	MobileMoneyMode     string
}

type CreatePaymentResult struct {
	PaymentID        string
	PaymentReference string
	Status           string
	ErrorMessage     string
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
}

type PaymentByBookingResult struct {
	PaymentID        string
	BookingID        string
	Amount           int
	PaymentMethod    string
	Status           string
	PaymentReference string
	CreatedAt        string
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
	RefundID         string
	BookingID        string
	RefundReason     string
	RefundRule       string
	OriginalAmount   int
	RefundPercentage int
	RefundAmount     int
	AmountToPassenger int
	AmountToDriver   int
	AmountToPlatform int
	Status           string
	ProcessedAt      string
	CompletedAt      string
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
