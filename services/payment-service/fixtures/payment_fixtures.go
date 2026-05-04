package fixtures

import (
	"context"
	"time"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// =============================================================================
// Payment
// =============================================================================

type PaymentOption func(*domain.Payment)

func WithPaymentID(id string) PaymentOption {
	return func(p *domain.Payment) { p.PaymentID = id }
}

func WithPaymentBookingID(id string) PaymentOption {
	return func(p *domain.Payment) { p.BookingID = id }
}

func WithPaymentTripID(id string) PaymentOption {
	return func(p *domain.Payment) { p.TripID = id }
}

func WithPaymentAmount(amount int) PaymentOption {
	return func(p *domain.Payment) { p.Amount = amount }
}

func WithPaymentMethod(method domain.PaymentMethod) PaymentOption {
	return func(p *domain.Payment) { p.PaymentMethod = method }
}

func WithPaymentStatus(status domain.PaymentStatus) PaymentOption {
	return func(p *domain.Payment) { p.Status = status }
}

func WithExternalTransactionID(id string) PaymentOption {
	return func(p *domain.Payment) { p.ExternalTransactionID = id }
}

func WithPaymentReference(ref string) PaymentOption {
	return func(p *domain.Payment) { p.PaymentReference = ref }
}

func WithPaymentCompletedAt(t time.Time) PaymentOption {
	return func(p *domain.Payment) { p.CompletedAt = &t }
}

func WithPaymentFailedAt(t time.Time) PaymentOption {
	return func(p *domain.Payment) { p.FailedAt = &t }
}

func WithPaymentFailureReason(reason string) PaymentOption {
	return func(p *domain.Payment) { p.FailureReason = &reason }
}

func WithPaymentPassengerPhoneNumber(phone string) PaymentOption {
	return func(p *domain.Payment) { p.PassengerPhoneNumber = phone }
}

// NewTestPayment cree un Payment avec des valeurs par defaut
func NewTestPayment(opts ...PaymentOption) *domain.Payment {
	now := time.Now().UTC()
	id := uuid.New().String()

	payment := &domain.Payment{
		PaymentID:             id,
		BookingID:             uuid.New().String(),
		TripID:                uuid.New().String(),
		Amount:                5000,
		PaymentMethod:         domain.PaymentMobileMoney,
		PaymentProvider:       "fedapay",
		Status:                domain.PaymentStatusPending,
		ExternalTransactionID: "12345",
		PaymentReference:      "PAY-" + id[:8],
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	for _, opt := range opts {
		opt(payment)
	}

	return payment
}

// =============================================================================
// Refund
// =============================================================================

type RefundOption func(*domain.Refund)

func WithRefundID(id string) RefundOption {
	return func(r *domain.Refund) { r.RefundID = id }
}

func WithRefundBookingID(id string) RefundOption {
	return func(r *domain.Refund) { r.BookingID = id }
}

func WithRefundPaymentID(id string) RefundOption {
	return func(r *domain.Refund) { r.PaymentID = id }
}

func WithRefundReason(reason domain.RefundReason) RefundOption {
	return func(r *domain.Refund) { r.RefundReason = reason }
}

func WithRefundRule(rule domain.RefundRule) RefundOption {
	return func(r *domain.Refund) { r.RefundRuleApplied = rule }
}

func WithRefundAmount(amount int) RefundOption {
	return func(r *domain.Refund) { r.RefundAmount = amount }
}

func WithRefundStatus(status domain.RefundStatus) RefundOption {
	return func(r *domain.Refund) { r.Status = status }
}

func WithRefundPercentage(pct int16) RefundOption {
	return func(r *domain.Refund) { r.RefundPercentage = pct }
}

func WithAmountToPassenger(amount int) RefundOption {
	return func(r *domain.Refund) { r.AmountToPassenger = amount }
}

func WithAmountToDriver(amount int) RefundOption {
	return func(r *domain.Refund) { r.AmountToDriver = amount }
}

func WithAmountToPlatform(amount int) RefundOption {
	return func(r *domain.Refund) { r.AmountToPlatform = amount }
}

// NewTestRefund cree un Refund avec des valeurs par defaut
func NewTestRefund(opts ...RefundOption) *domain.Refund {
	now := time.Now().UTC()
	id := uuid.New().String()

	refund := &domain.Refund{
		RefundID:          id,
		RefundReference:   "REF-" + id[:8],
		PaymentID:         uuid.New().String(),
		BookingID:         uuid.New().String(),
		RefundReason:      domain.RefundReasonCancelledByDriver,
		RefundRuleApplied: domain.RefundRuleDriverCancellation,
		OriginalAmount:    5000,
		RefundPercentage:  100,
		RefundAmount:      5500,
		ServiceFeeRefunded: true,
		AmountToPassenger: 5500,
		AmountToDriver:    0,
		AmountToPlatform:  0,
		Status:            domain.RefundStatusCompleted,
		RefundMethod:      "mobileMoney",
		ProcessedAt:       &now,
		UpdatedAt:         now,
	}

	for _, opt := range opts {
		opt(refund)
	}

	return refund
}

// =============================================================================
// Payout
// =============================================================================

type PayoutOption func(*domain.Payout)

func WithPayoutID(id string) PayoutOption {
	return func(p *domain.Payout) { p.PayoutID = id }
}

func WithPayoutDriverID(id string) PayoutOption {
	return func(p *domain.Payout) { p.DriverID = id }
}

func WithPayoutTripID(id string) PayoutOption {
	return func(p *domain.Payout) { p.TripID = id }
}

func WithPayoutNetAmount(amount int) PayoutOption {
	return func(p *domain.Payout) { p.NetAmount = amount }
}

func WithPayoutStatus(status domain.PayoutStatus) PayoutOption {
	return func(p *domain.Payout) { p.Status = status }
}

func WithPayoutFailureReason(reason string) PayoutOption {
	return func(p *domain.Payout) { p.FailureReason = &reason }
}

func WithPayoutCompletedAt(t time.Time) PayoutOption {
	return func(p *domain.Payout) { p.CompletedAt = &t }
}

func WithPayoutScheduledAt(t time.Time) PayoutOption {
	return func(p *domain.Payout) { p.ScheduledAt = &t }
}

// NewTestPayout cree un Payout avec des valeurs par defaut
func NewTestPayout(opts ...PayoutOption) *domain.Payout {
	now := time.Now().UTC()
	id := uuid.New().String()

	payout := &domain.Payout{
		PayoutID:          id,
		PayoutReference:   "PO-" + id[:8],
		DriverID:          uuid.New().String(),
		TripID:            uuid.New().String(),
		GrossAmount:       5000,
		PlatformFee:       500,
		NetAmount:         4500,
		PayoutMethod:      "mobileMoney",
		PayoutDestination: "+22890001234",
		DestinationName:   "Kofi Mensah",
		Status:            domain.PayoutStatusPending,
		PaymentProvider:   "fedapay",
		RetryCount:        0,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	for _, opt := range opts {
		opt(payout)
	}

	return payout
}

// =============================================================================
// DB Insert helpers (pour tests d'integration)
// =============================================================================

// InsertPayment insere un Payment dans la base de donnees de test
func InsertPayment(ctx context.Context, pool *pgxpool.Pool, p *domain.Payment) error {
	query := `INSERT INTO payments (
		payment_id, booking_id, trip_id, amount, payment_method, payment_provider,
		status, external_transaction_id, payment_reference, metadata,
		completed_at, failed_at, failure_reason
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

	_, err := pool.Exec(ctx, query,
		p.PaymentID, p.BookingID, p.TripID, p.Amount,
		p.PaymentMethod, p.PaymentProvider,
		p.Status, p.ExternalTransactionID, p.PaymentReference,
		p.Metadata, p.CompletedAt, p.FailedAt, p.FailureReason,
	)
	return err
}

// InsertRefund insere un Refund dans la base de donnees de test
func InsertRefund(ctx context.Context, pool *pgxpool.Pool, r *domain.Refund) error {
	query := `INSERT INTO refunds (
		refund_id, refund_reference, payment_id, booking_id,
		refund_reason, refund_rule_applied, original_amount, refund_percentage,
		refund_amount, service_fee_refunded, amount_to_passenger, amount_to_driver,
		amount_to_platform, status, refund_method, processed_at, completed_at, notes,
		payout_destination, payment_provider, payment_provider_reference
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)`

	provider := r.PaymentProvider
	if provider == "" {
		provider = "fedapay"
	}

	_, err := pool.Exec(ctx, query,
		r.RefundID, r.RefundReference, r.PaymentID, r.BookingID,
		r.RefundReason, r.RefundRuleApplied, r.OriginalAmount, r.RefundPercentage,
		r.RefundAmount, r.ServiceFeeRefunded, r.AmountToPassenger, r.AmountToDriver,
		r.AmountToPlatform, r.Status, r.RefundMethod, r.ProcessedAt, r.CompletedAt, r.Notes,
		r.PayoutDestination, provider, r.PaymentProviderReference,
	)
	return err
}

// InsertPayout insere un Payout dans la base de donnees de test
func InsertPayout(ctx context.Context, pool *pgxpool.Pool, p *domain.Payout) error {
	query := `INSERT INTO payouts (
		payout_id, payout_reference, driver_id, trip_id, gross_amount, platform_fee,
		net_amount, payout_method, payout_destination, destination_name,
		status, payment_provider, payment_provider_reference, retry_count,
		scheduled_at, completed_at, failed_at, cancelled_at, failure_reason
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)`

	_, err := pool.Exec(ctx, query,
		p.PayoutID, p.PayoutReference, p.DriverID, p.TripID, p.GrossAmount, p.PlatformFee,
		p.NetAmount, p.PayoutMethod, p.PayoutDestination, p.DestinationName,
		p.Status, p.PaymentProvider, p.PaymentProviderReference, p.RetryCount,
		p.ScheduledAt, p.CompletedAt, p.FailedAt, p.CancelledAt, p.FailureReason,
	)
	return err
}
