package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/payment-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/config"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/service"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/payment-service/internal/service/interfaces"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
	"github.com/Kpeewu/tissi-mah/services/payment-service/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// --- Helpers ---

type testDeps struct {
	paymentReadRepo   *mocks.MockPaymentRepositoryRead
	paymentWriteRepo  *mocks.MockPaymentRepositoryWrite
	refundReadRepo    *mocks.MockRefundRepositoryRead
	refundWriteRepo   *mocks.MockRefundRepositoryWrite
	payoutReadRepo    *mocks.MockPayoutRepositoryRead
	payoutWriteRepo   *mocks.MockPayoutRepositoryWrite
	payoutHistoryRepo *mocks.MockPayoutHistoryRepositoryWrite
	bookingClient     *mocks.MockBookingClient
	userClient        *mocks.MockUserClient
	supportClient     *mocks.MockSupportClient
	svc               serviceInterfaces.PaymentService
}

func newTestService() *testDeps {
	d := &testDeps{
		paymentReadRepo:   new(mocks.MockPaymentRepositoryRead),
		paymentWriteRepo:  new(mocks.MockPaymentRepositoryWrite),
		refundReadRepo:    new(mocks.MockRefundRepositoryRead),
		refundWriteRepo:   new(mocks.MockRefundRepositoryWrite),
		payoutReadRepo:    new(mocks.MockPayoutRepositoryRead),
		payoutWriteRepo:   new(mocks.MockPayoutRepositoryWrite),
		payoutHistoryRepo: new(mocks.MockPayoutHistoryRepositoryWrite),
		bookingClient:     new(mocks.MockBookingClient),
		userClient:        new(mocks.MockUserClient),
		supportClient:     new(mocks.MockSupportClient),
	}

	cfg := &config.Config{
		Refund: config.RefundConfig{
			CancellationFullRefundHours:    24,
			CancellationGracePeriodMinutes: 30,
			NoShowDriverDelayMinutes:       15,
			NoShowPassengerDelayMinutes:    15,
		},
		Payout: config.PayoutConfig{
			IntervalSeconds:        1800,
			ContestationDelayHours: 2,
			PlatformFeePercent:     10,
		},
	}
	logger := zap.NewNop()

	d.svc = service.NewPaymentService(
		d.paymentReadRepo,
		d.paymentWriteRepo,
		d.refundReadRepo,
		d.refundWriteRepo,
		d.payoutReadRepo,
		d.payoutWriteRepo,
		d.payoutHistoryRepo,
		d.bookingClient,
		d.userClient,
		d.supportClient,
		nil, // fedapayClient — non testable sans mock HTTP
		nil, // cache
		nil, // notifRedis
		cfg,
		logger,
	)

	return d
}

func (d *testDeps) assertExpectations(t *testing.T) {
	d.paymentReadRepo.AssertExpectations(t)
	d.paymentWriteRepo.AssertExpectations(t)
	d.refundReadRepo.AssertExpectations(t)
	d.refundWriteRepo.AssertExpectations(t)
	d.payoutReadRepo.AssertExpectations(t)
	d.payoutWriteRepo.AssertExpectations(t)
	d.payoutHistoryRepo.AssertExpectations(t)
	d.bookingClient.AssertExpectations(t)
	d.userClient.AssertExpectations(t)
	d.supportClient.AssertExpectations(t)
}

// =============================================================================
// GetPaymentStatus
// =============================================================================

func TestGetPaymentStatus(t *testing.T) {
	t.Run("succes - retourne le statut du paiement", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		payment := fixtures.NewTestPayment(
			fixtures.WithPaymentID("pay-123"),
			fixtures.WithPaymentBookingID("booking-456"),
			fixtures.WithPaymentAmount(5000),
			fixtures.WithPaymentStatus(domain.PaymentStatusHeld),
		)
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-123").Return(payment, nil)

		result, err := d.svc.GetPaymentStatus(ctx, "pay-123")

		require.NoError(t, err)
		assert.Equal(t, "pay-123", result.PaymentID)
		assert.Equal(t, "booking-456", result.BookingID)
		assert.Equal(t, 5000, result.Amount)
		assert.Equal(t, "held", result.Status)
		d.assertExpectations(t)
	})

	t.Run("erreur - paiement non trouve", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-unknown").Return(nil, paymentErrors.ErrorPaymentNotFound)

		result, err := d.svc.GetPaymentStatus(ctx, "pay-unknown")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, paymentErrors.ErrorPaymentNotFound)
		d.assertExpectations(t)
	})
}

// =============================================================================
// GetPaymentByBooking
// =============================================================================

func TestGetPaymentByBooking(t *testing.T) {
	t.Run("succes - retourne le paiement par booking", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		payment := fixtures.NewTestPayment(
			fixtures.WithPaymentID("pay-abc"),
			fixtures.WithPaymentBookingID("booking-xyz"),
			fixtures.WithPaymentAmount(3000),
			fixtures.WithPaymentStatus(domain.PaymentStatusPending),
		)
		d.paymentReadRepo.On("GetByBookingID", mock.Anything, "booking-xyz").Return(payment, nil)

		result, err := d.svc.GetPaymentByBooking(ctx, "booking-xyz")

		require.NoError(t, err)
		assert.Equal(t, "pay-abc", result.PaymentID)
		assert.Equal(t, "booking-xyz", result.BookingID)
		assert.Equal(t, 3000, result.Amount)
		assert.Equal(t, "pending", result.Status)
		d.assertExpectations(t)
	})

	t.Run("erreur - booking sans paiement", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		d.paymentReadRepo.On("GetByBookingID", mock.Anything, "no-booking").Return(nil, paymentErrors.ErrorPaymentNotFound)

		result, err := d.svc.GetPaymentByBooking(ctx, "no-booking")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, paymentErrors.ErrorPaymentNotFound)
		d.assertExpectations(t)
	})
}

// =============================================================================
// ReleasePayment
// =============================================================================

func TestReleasePayment(t *testing.T) {
	t.Run("succes - libere un paiement held", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		payment := fixtures.NewTestPayment(
			fixtures.WithPaymentID("pay-release"),
			fixtures.WithPaymentBookingID("booking-release"),
			fixtures.WithPaymentStatus(domain.PaymentStatusHeld),
		)
		d.paymentReadRepo.On("GetByBookingID", mock.Anything, "booking-release").Return(payment, nil)
		d.paymentWriteRepo.On("UpdatePaymentStatus", mock.Anything, "pay-release", domain.PaymentStatusReleased, "").Return(nil)

		err := d.svc.ReleasePayment(ctx, "booking-release")

		require.NoError(t, err)
		d.assertExpectations(t)
	})

	t.Run("erreur - paiement non held", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		payment := fixtures.NewTestPayment(
			fixtures.WithPaymentStatus(domain.PaymentStatusPending),
		)
		d.paymentReadRepo.On("GetByBookingID", mock.Anything, "booking-pending").Return(payment, nil)

		err := d.svc.ReleasePayment(ctx, "booking-pending")

		assert.ErrorIs(t, err, paymentErrors.ErrorInvalidPaymentStatus)
		d.assertExpectations(t)
	})

	t.Run("erreur - paiement deja released", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		payment := fixtures.NewTestPayment(
			fixtures.WithPaymentStatus(domain.PaymentStatusReleased),
		)
		d.paymentReadRepo.On("GetByBookingID", mock.Anything, "booking-released").Return(payment, nil)

		err := d.svc.ReleasePayment(ctx, "booking-released")

		assert.ErrorIs(t, err, paymentErrors.ErrorInvalidPaymentStatus)
		d.assertExpectations(t)
	})

	t.Run("erreur - booking non trouve", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		d.paymentReadRepo.On("GetByBookingID", mock.Anything, "unknown").Return(nil, paymentErrors.ErrorPaymentNotFound)

		err := d.svc.ReleasePayment(ctx, "unknown")

		assert.ErrorIs(t, err, paymentErrors.ErrorPaymentNotFound)
		d.assertExpectations(t)
	})
}

// =============================================================================
// RequestRefund
// =============================================================================

func TestRequestRefund(t *testing.T) {
	departure := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	cancelledAt := departure.Add(-25 * time.Hour) // plus de 24h avant → full refund

	t.Run("succes - remboursement complet annulation chauffeur", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		payment := fixtures.NewTestPayment(
			fixtures.WithPaymentID("pay-refund"),
			fixtures.WithPaymentBookingID("booking-refund"),
			fixtures.WithPaymentStatus(domain.PaymentStatusHeld),
		)
		d.paymentReadRepo.On("GetByBookingID", mock.Anything, "booking-refund").Return(payment, nil)
		d.refundWriteRepo.On("CreateRefund", mock.Anything, mock.MatchedBy(func(r *domain.Refund) bool {
			return r.BookingID == "booking-refund" &&
				r.RefundReason == domain.RefundReasonCancelledByDriver &&
				r.RefundRuleApplied == domain.RefundRuleDriverCancellation &&
				r.RefundAmount == 5500 && // 5000 + 500
				r.AmountToPassenger == 5500
		})).Return(nil)
		d.paymentWriteRepo.On("UpdatePaymentStatus", mock.Anything, "pay-refund", domain.PaymentStatusRefunded, "").Return(nil)

		input := &serviceInterfaces.RequestRefundInput{
			BookingID:         "booking-refund",
			RefundReason:      "cancelledByDriver",
			OriginalAmount:    5000,
			ServiceFee:        500,
			DepartureDatetime: departure.Format(time.RFC3339),
			CancelledAt:       cancelledAt.Format(time.RFC3339),
		}

		result, err := d.svc.RequestRefund(ctx, input)

		require.NoError(t, err)
		assert.Equal(t, "completed", result.Status)
		assert.Equal(t, 5500, result.RefundAmount)
		d.assertExpectations(t)
	})

	t.Run("succes - remboursement partiel passager apres grace period", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		payment := fixtures.NewTestPayment(
			fixtures.WithPaymentID("pay-partial"),
			fixtures.WithPaymentBookingID("booking-partial"),
			fixtures.WithPaymentStatus(domain.PaymentStatusHeld),
		)
		d.paymentReadRepo.On("GetByBookingID", mock.Anything, "booking-partial").Return(payment, nil)
		d.refundWriteRepo.On("CreateRefund", mock.Anything, mock.MatchedBy(func(r *domain.Refund) bool {
			return r.RefundPercentage == 50 &&
				r.RefundAmount == 2500 &&
				r.AmountToPassenger == 2500 &&
				r.AmountToDriver == 2500
		})).Return(nil)
		d.paymentWriteRepo.On("UpdatePaymentStatus", mock.Anything, "pay-partial", domain.PaymentStatusRefunded, "").Return(nil)

		// Depart dans 2h → annulation est dans les 24h avant depart
		nearDeparture := time.Now().UTC().Add(2 * time.Hour)
		approvedAt := time.Now().UTC().Add(-2 * time.Hour)
		cancelledAtLate := approvedAt.Add(45 * time.Minute) // 45min apres approbation → over 30min

		input := &serviceInterfaces.RequestRefundInput{
			BookingID:         "booking-partial",
			RefundReason:      "cancelledByPassenger",
			OriginalAmount:    5000,
			ServiceFee:        500,
			DepartureDatetime: nearDeparture.Format(time.RFC3339),
			ApprovedAt:        approvedAt.Format(time.RFC3339),
			CancelledAt:       cancelledAtLate.Format(time.RFC3339),
		}

		result, err := d.svc.RequestRefund(ctx, input)

		require.NoError(t, err)
		assert.Equal(t, 2500, result.RefundAmount)
		d.assertExpectations(t)
	})

	t.Run("succes - no-show passager → pas de remboursement", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		payment := fixtures.NewTestPayment(
			fixtures.WithPaymentID("pay-noshow"),
			fixtures.WithPaymentBookingID("booking-noshow"),
			fixtures.WithPaymentStatus(domain.PaymentStatusHeld),
		)
		d.paymentReadRepo.On("GetByBookingID", mock.Anything, "booking-noshow").Return(payment, nil)
		d.refundWriteRepo.On("CreateRefund", mock.Anything, mock.MatchedBy(func(r *domain.Refund) bool {
			return r.RefundAmount == 0 &&
				r.AmountToPassenger == 0 &&
				r.AmountToDriver == 5000
		})).Return(nil)
		d.paymentWriteRepo.On("UpdatePaymentStatus", mock.Anything, "pay-noshow", domain.PaymentStatusRefunded, "").Return(nil)

		input := &serviceInterfaces.RequestRefundInput{
			BookingID:         "booking-noshow",
			RefundReason:      "noShowPassenger",
			OriginalAmount:    5000,
			ServiceFee:        500,
			DepartureDatetime: departure.Format(time.RFC3339),
			CancelledAt:       time.Now().UTC().Format(time.RFC3339),
		}

		result, err := d.svc.RequestRefund(ctx, input)

		require.NoError(t, err)
		assert.Equal(t, 0, result.RefundAmount)
		d.assertExpectations(t)
	})

	t.Run("erreur - paiement non held/released", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		payment := fixtures.NewTestPayment(
			fixtures.WithPaymentStatus(domain.PaymentStatusPending),
		)
		d.paymentReadRepo.On("GetByBookingID", mock.Anything, "booking-pending").Return(payment, nil)

		input := &serviceInterfaces.RequestRefundInput{
			BookingID:         "booking-pending",
			RefundReason:      "cancelledByDriver",
			OriginalAmount:    5000,
			ServiceFee:        500,
			DepartureDatetime: departure.Format(time.RFC3339),
			CancelledAt:       time.Now().UTC().Format(time.RFC3339),
		}

		result, err := d.svc.RequestRefund(ctx, input)

		assert.Nil(t, result)
		assert.ErrorIs(t, err, paymentErrors.ErrorInvalidPaymentStatus)
		d.assertExpectations(t)
	})

	t.Run("erreur - date de depart invalide", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		payment := fixtures.NewTestPayment(
			fixtures.WithPaymentStatus(domain.PaymentStatusHeld),
		)
		d.paymentReadRepo.On("GetByBookingID", mock.Anything, "booking-bad-date").Return(payment, nil)

		input := &serviceInterfaces.RequestRefundInput{
			BookingID:         "booking-bad-date",
			RefundReason:      "cancelledByDriver",
			OriginalAmount:    5000,
			ServiceFee:        500,
			DepartureDatetime: "not-a-date",
			CancelledAt:       time.Now().UTC().Format(time.RFC3339),
		}

		result, err := d.svc.RequestRefund(ctx, input)

		assert.Nil(t, result)
		assert.ErrorIs(t, err, paymentErrors.ErrorInvalidInput)
		d.assertExpectations(t)
	})
}

// =============================================================================
// GetRefundStatus
// =============================================================================

func TestGetRefundStatus(t *testing.T) {
	t.Run("succes - retourne le statut du remboursement", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		refund := fixtures.NewTestRefund(
			fixtures.WithRefundID("ref-123"),
			fixtures.WithRefundBookingID("booking-ref"),
			fixtures.WithRefundReason(domain.RefundReasonCancelledByDriver),
			fixtures.WithRefundRule(domain.RefundRuleDriverCancellation),
			fixtures.WithRefundAmount(5500),
			fixtures.WithRefundPercentage(100),
			fixtures.WithAmountToPassenger(5500),
			fixtures.WithAmountToDriver(0),
			fixtures.WithAmountToPlatform(0),
			fixtures.WithRefundStatus(domain.RefundStatusCompleted),
		)
		d.refundReadRepo.On("GetByID", mock.Anything, "ref-123").Return(refund, nil)

		result, err := d.svc.GetRefundStatus(ctx, "ref-123")

		require.NoError(t, err)
		assert.Equal(t, "ref-123", result.RefundID)
		assert.Equal(t, "booking-ref", result.BookingID)
		assert.Equal(t, "cancelledByDriver", result.RefundReason)
		assert.Equal(t, "driverCancellation", result.RefundRule)
		assert.Equal(t, 5500, result.RefundAmount)
		assert.Equal(t, 100, result.RefundPercentage)
		assert.Equal(t, 5500, result.AmountToPassenger)
		assert.Equal(t, "completed", result.Status)
		d.assertExpectations(t)
	})

	t.Run("erreur - remboursement non trouve", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		d.refundReadRepo.On("GetByID", mock.Anything, "ref-unknown").Return(nil, paymentErrors.ErrorRefundNotFound)

		result, err := d.svc.GetRefundStatus(ctx, "ref-unknown")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, paymentErrors.ErrorRefundNotFound)
		d.assertExpectations(t)
	})
}

// =============================================================================
// GetPayoutStatus
// =============================================================================

func TestGetPayoutStatus(t *testing.T) {
	t.Run("succes - retourne le statut du payout", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		payout := fixtures.NewTestPayout(
			fixtures.WithPayoutID("payout-123"),
			fixtures.WithPayoutDriverID("driver-abc"),
			fixtures.WithPayoutTripID("trip-xyz"),
			fixtures.WithPayoutNetAmount(4500),
			fixtures.WithPayoutStatus(domain.PayoutStatusCompleted),
		)
		d.payoutReadRepo.On("GetByID", mock.Anything, "payout-123").Return(payout, nil)

		result, err := d.svc.GetPayoutStatus(ctx, "payout-123")

		require.NoError(t, err)
		assert.Equal(t, "payout-123", result.PayoutID)
		assert.Equal(t, "driver-abc", result.DriverID)
		assert.Equal(t, "trip-xyz", result.TripID)
		assert.Equal(t, 4500, result.NetAmount)
		assert.Equal(t, "completed", result.Status)
		assert.Equal(t, "", result.FailureReason)
		d.assertExpectations(t)
	})

	t.Run("succes - payout echoue avec raison", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		payout := fixtures.NewTestPayout(
			fixtures.WithPayoutID("payout-fail"),
			fixtures.WithPayoutStatus(domain.PayoutStatusFailed),
			fixtures.WithPayoutFailureReason("Solde insuffisant"),
		)
		d.payoutReadRepo.On("GetByID", mock.Anything, "payout-fail").Return(payout, nil)

		result, err := d.svc.GetPayoutStatus(ctx, "payout-fail")

		require.NoError(t, err)
		assert.Equal(t, "failed", result.Status)
		assert.Equal(t, "Solde insuffisant", result.FailureReason)
		d.assertExpectations(t)
	})

	t.Run("erreur - payout non trouve", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		d.payoutReadRepo.On("GetByID", mock.Anything, "payout-unknown").Return(nil, paymentErrors.ErrorPayoutNotFound)

		result, err := d.svc.GetPayoutStatus(ctx, "payout-unknown")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, paymentErrors.ErrorPayoutNotFound)
		d.assertExpectations(t)
	})
}

// =============================================================================
// GetDriverPayouts
// =============================================================================

func TestGetDriverPayouts(t *testing.T) {
	t.Run("succes - retourne la liste des payouts", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		payouts := []*domain.Payout{
			fixtures.NewTestPayout(
				fixtures.WithPayoutID("po-1"),
				fixtures.WithPayoutDriverID("driver-1"),
				fixtures.WithPayoutNetAmount(4500),
				fixtures.WithPayoutStatus(domain.PayoutStatusCompleted),
			),
			fixtures.NewTestPayout(
				fixtures.WithPayoutID("po-2"),
				fixtures.WithPayoutDriverID("driver-1"),
				fixtures.WithPayoutNetAmount(3000),
				fixtures.WithPayoutStatus(domain.PayoutStatusPending),
			),
		}
		d.payoutReadRepo.On("GetDriverPayouts", mock.Anything, "driver-1", 0).Return(payouts, nil)

		results, err := d.svc.GetDriverPayouts(ctx, "driver-1", 0)

		require.NoError(t, err)
		assert.Len(t, results, 2)
		assert.Equal(t, "po-1", results[0].PayoutID)
		assert.Equal(t, 4500, results[0].NetAmount)
		assert.Equal(t, "completed", results[0].Status)
		assert.Equal(t, "po-2", results[1].PayoutID)
		assert.Equal(t, 3000, results[1].NetAmount)
		d.assertExpectations(t)
	})

	t.Run("succes - liste vide", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		d.payoutReadRepo.On("GetDriverPayouts", mock.Anything, "driver-empty", 0).Return([]*domain.Payout{}, nil)

		results, err := d.svc.GetDriverPayouts(ctx, "driver-empty", 0)

		require.NoError(t, err)
		assert.Empty(t, results)
		d.assertExpectations(t)
	})
}
