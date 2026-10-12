package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/payment-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/client"
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
	payoutHistoryRepo *mocks.MockPayoutHistoryRepository
	// Historique paiements/remboursements : CreateHistoryEntry accepté par défaut (cf. newHistoryMocks),
	// les tests dédiés inspectent les appels enregistrés.
	paymentHistoryRepo *mocks.MockPaymentHistoryRepository
	refundHistoryRepo  *mocks.MockRefundHistoryRepository
	bookingClient      *mocks.MockBookingClient
	userClient         *mocks.MockUserClient
	supportClient      *mocks.MockSupportClient
	svc                serviceInterfaces.PaymentService
}

func newTestService() *testDeps {
	d := &testDeps{
		paymentReadRepo:    new(mocks.MockPaymentRepositoryRead),
		paymentWriteRepo:   new(mocks.MockPaymentRepositoryWrite),
		refundReadRepo:     new(mocks.MockRefundRepositoryRead),
		refundWriteRepo:    new(mocks.MockRefundRepositoryWrite),
		payoutReadRepo:     new(mocks.MockPayoutRepositoryRead),
		payoutWriteRepo:    new(mocks.MockPayoutRepositoryWrite),
		payoutHistoryRepo:  new(mocks.MockPayoutHistoryRepository),
		paymentHistoryRepo: newPaymentHistoryMock(),
		refundHistoryRepo:  newRefundHistoryMock(),
		bookingClient:      new(mocks.MockBookingClient),
		userClient:         new(mocks.MockUserClient),
		supportClient:      new(mocks.MockSupportClient),
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
		d.payoutHistoryRepo, d.paymentHistoryRepo, d.refundHistoryRepo,
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

// expectBooking configure la réservation bookingID et son passager. L'appelant de passengerCtx()
// est testPassengerID : il n'est pas le passager si passengerID diffère.
func (d *testDeps) expectBooking(bookingID, passengerID string) {
	d.bookingClient.On("GetBookingDetails", mock.Anything, bookingID).
		Return(&client.BookingDetails{BookingID: bookingID, PassengerID: passengerID}, nil)
}

// =============================================================================
// GetPaymentStatus
// =============================================================================

func TestGetPaymentStatus(t *testing.T) {
	t.Run("succes - retourne le statut du paiement", func(t *testing.T) {
		d := newTestService()
		ctx := passengerCtx()

		payment := fixtures.NewTestPayment(
			fixtures.WithPaymentID("pay-123"),
			fixtures.WithPaymentBookingID("booking-456"),
			fixtures.WithPaymentAmount(5000),
			fixtures.WithPaymentStatus(domain.PaymentStatusHeld),
		)
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-123").Return(payment, nil)
		d.expectBooking("booking-456", testPassengerID)

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

	t.Run("erreur - appelant autre que le passager", func(t *testing.T) {
		d := newTestService()

		payment := fixtures.NewTestPayment(fixtures.WithPaymentID("pay-123"), fixtures.WithPaymentBookingID("booking-456"))
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-123").Return(payment, nil)
		d.expectBooking("booking-456", "someone-else")

		result, err := d.svc.GetPaymentStatus(passengerCtx(), "pay-123")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, paymentErrors.ErrorPaymentNotOwned)
		d.assertExpectations(t)
	})

	t.Run("erreur - sans identite", func(t *testing.T) {
		d := newTestService()

		payment := fixtures.NewTestPayment(fixtures.WithPaymentID("pay-123"), fixtures.WithPaymentBookingID("booking-456"))
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-123").Return(payment, nil)
		d.bookingClient.On("GetBookingDetails", mock.Anything, "booking-456").
			Return(&client.BookingDetails{BookingID: "booking-456", PassengerID: testPassengerID}, nil)

		result, err := d.svc.GetPaymentStatus(context.Background(), "pay-123")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, paymentErrors.ErrorUnauthorized)
		d.assertExpectations(t)
	})
}

// =============================================================================
// GetPaymentByBooking
// =============================================================================

func TestGetPaymentByBooking(t *testing.T) {
	t.Run("succes - retourne le paiement par booking", func(t *testing.T) {
		d := newTestService()
		ctx := passengerCtx()

		payment := fixtures.NewTestPayment(
			fixtures.WithPaymentID("pay-abc"),
			fixtures.WithPaymentBookingID("booking-xyz"),
			fixtures.WithPaymentAmount(3000),
			fixtures.WithPaymentStatus(domain.PaymentStatusPending),
		)
		d.paymentReadRepo.On("GetByBookingID", mock.Anything, "booking-xyz").Return(payment, nil)
		d.expectBooking("booking-xyz", testPassengerID)

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
		ctx := passengerCtx()

		d.paymentReadRepo.On("GetByBookingID", mock.Anything, "no-booking").Return(nil, paymentErrors.ErrorPaymentNotFound)
		d.expectBooking("no-booking", testPassengerID)

		result, err := d.svc.GetPaymentByBooking(ctx, "no-booking")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, paymentErrors.ErrorPaymentNotFound)
		d.assertExpectations(t)
	})

	t.Run("erreur - appelant autre que le passager : aucune lecture du paiement", func(t *testing.T) {
		d := newTestService()
		d.expectBooking("booking-xyz", "someone-else")

		result, err := d.svc.GetPaymentByBooking(passengerCtx(), "booking-xyz")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, paymentErrors.ErrorPaymentNotOwned)
		d.paymentReadRepo.AssertNotCalled(t, "GetByBookingID", mock.Anything, mock.Anything)
		d.assertExpectations(t)
	})

	t.Run("erreur - BookingId vide", func(t *testing.T) {
		d := newTestService()

		result, err := d.svc.GetPaymentByBooking(passengerCtx(), "")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, paymentErrors.ErrorInvalidInput)
		d.bookingClient.AssertNotCalled(t, "GetBookingDetails", mock.Anything, mock.Anything)
	})
}

// =============================================================================
// GetPaymentByBookingInternal
// =============================================================================

func TestGetPaymentByBookingInternal(t *testing.T) {
	t.Run("succes - sans identite ni controle d'appartenance", func(t *testing.T) {
		d := newTestService()

		payment := fixtures.NewTestPayment(
			fixtures.WithPaymentBookingID("booking-xyz"),
			fixtures.WithPaymentStatus(domain.PaymentStatusHeld),
		)
		d.paymentReadRepo.On("GetByBookingID", mock.Anything, "booking-xyz").Return(payment, nil)

		result, err := d.svc.GetPaymentByBookingInternal(context.Background(), "booking-xyz")

		require.NoError(t, err)
		assert.Equal(t, "held", result.Status)
		d.bookingClient.AssertNotCalled(t, "GetBookingDetails", mock.Anything, mock.Anything)
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
		history := d.paymentHistory()
		require.Len(t, history, 1)
		assert.Equal(t, "released", history[0].Status)
		assert.Equal(t, domain.HistoryInitiatedByBookingService, history[0].InitiatedBy)
		assert.Equal(t, domain.HistorySourceBookingService, history[0].Source)
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
		assert.Empty(t, d.paymentHistory())
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

	t.Run("succes - annulation chauffeur cree un refund pending (AmountToPassenger > 0)", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		payment := fixtures.NewTestPayment(
			fixtures.WithPaymentID("pay-refund"),
			fixtures.WithPaymentBookingID("booking-refund"),
			fixtures.WithPaymentStatus(domain.PaymentStatusHeld),
			fixtures.WithPaymentPassengerPhoneNumber("90001122"),
		)
		d.paymentReadRepo.On("GetByBookingID", mock.Anything, "booking-refund").Return(payment, nil)
		d.refundWriteRepo.On("CreateRefund", mock.Anything, mock.MatchedBy(func(r *domain.Refund) bool {
			return r.BookingID == "booking-refund" &&
				r.RefundReason == domain.RefundReasonCancelledByDriver &&
				r.RefundRuleApplied == domain.RefundRuleDriverCancellation &&
				r.RefundAmount == 5500 && // 5000 + 500
				r.AmountToPassenger == 5500 &&
				r.Status == domain.RefundStatusPending &&
				r.CompletedAt == nil &&
				r.PayoutDestination == "90001122" &&
				r.PaymentProvider == "fedapay"
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
		assert.Equal(t, "pending", result.Status)
		assert.Equal(t, 5500, result.RefundAmount)
		d.assertExpectations(t)

		assert.Equal(t, []string{"refunded"}, d.paymentHistoryStatuses("pay-refund"))
		assert.Contains(t, d.paymentHistory()[0].Notes, "cancelledByDriver")
		refundHistory := d.refundHistory()
		require.Len(t, refundHistory, 1)
		assert.Equal(t, result.RefundID, refundHistory[0].RefundID)
		assert.Equal(t, "pending", refundHistory[0].Status)
		assert.Equal(t, domain.HistoryInitiatedByBookingService, refundHistory[0].InitiatedBy)
		assert.Equal(t, domain.HistorySourceBookingService, refundHistory[0].Source)
		assert.Contains(t, refundHistory[0].Notes, "règle driverCancellation (100%)")
	})

	t.Run("succes - remboursement partiel passager apres grace period (pending)", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		payment := fixtures.NewTestPayment(
			fixtures.WithPaymentID("pay-partial"),
			fixtures.WithPaymentBookingID("booking-partial"),
			fixtures.WithPaymentStatus(domain.PaymentStatusHeld),
			fixtures.WithPaymentPassengerPhoneNumber("91112233"),
		)
		d.paymentReadRepo.On("GetByBookingID", mock.Anything, "booking-partial").Return(payment, nil)
		d.refundWriteRepo.On("CreateRefund", mock.Anything, mock.MatchedBy(func(r *domain.Refund) bool {
			return r.RefundPercentage == 50 &&
				r.RefundAmount == 2500 &&
				r.AmountToPassenger == 2500 &&
				r.AmountToDriver == 2500 &&
				r.Status == domain.RefundStatusPending &&
				r.PayoutDestination == "91112233"
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
		assert.Equal(t, "pending", result.Status)
		assert.Equal(t, 2500, result.RefundAmount)
		d.assertExpectations(t)
	})

	t.Run("succes - no-show passager cree un refund completed sans transfert", func(t *testing.T) {
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
				r.AmountToDriver == 5000 &&
				r.Status == domain.RefundStatusCompleted &&
				r.CompletedAt != nil
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
		assert.Equal(t, "completed", result.Status)
		assert.Equal(t, 0, result.RefundAmount)
		d.assertExpectations(t)
		refundHistory := d.refundHistory()
		require.Len(t, refundHistory, 1)
		assert.Equal(t, "completed", refundHistory[0].Status)
		assert.Contains(t, refundHistory[0].Notes, "aucun virement passager requis")
	})

	t.Run("erreur - motif de remboursement inconnu", func(t *testing.T) {
		d := newTestService()

		input := &serviceInterfaces.RequestRefundInput{
			BookingID:         "booking-refund",
			RefundReason:      "unknownReason",
			OriginalAmount:    5000,
			ServiceFee:        500,
			DepartureDatetime: departure.Format(time.RFC3339),
			CancelledAt:       cancelledAt.Format(time.RFC3339),
		}

		result, err := d.svc.RequestRefund(context.Background(), input)

		assert.Nil(t, result)
		assert.ErrorIs(t, err, paymentErrors.ErrorInvalidInput)
		d.paymentReadRepo.AssertNotCalled(t, "GetByBookingID", mock.Anything, mock.Anything)
		d.refundWriteRepo.AssertNotCalled(t, "CreateRefund", mock.Anything, mock.Anything)
	})

	t.Run("idempotent - rejeu sur un paiement deja rembourse retourne le refund existant", func(t *testing.T) {
		d := newTestService()

		payment := fixtures.NewTestPayment(
			fixtures.WithPaymentID("pay-replay"),
			fixtures.WithPaymentBookingID("booking-replay"),
			fixtures.WithPaymentStatus(domain.PaymentStatusRefunded),
		)
		existing := fixtures.NewTestRefund(
			fixtures.WithRefundID("ref-existing"),
			fixtures.WithRefundPaymentID("pay-replay"),
			fixtures.WithRefundAmount(5500),
			fixtures.WithRefundStatus(domain.RefundStatusPending),
		)
		d.paymentReadRepo.On("GetByBookingID", mock.Anything, "booking-replay").Return(payment, nil)
		d.refundReadRepo.On("GetByPaymentID", mock.Anything, "pay-replay").Return(existing, nil)

		result, err := d.svc.RequestRefund(context.Background(), &serviceInterfaces.RequestRefundInput{
			BookingID:         "booking-replay",
			RefundReason:      "tripCancelled",
			OriginalAmount:    5000,
			ServiceFee:        500,
			DepartureDatetime: departure.Format(time.RFC3339),
			CancelledAt:       cancelledAt.Format(time.RFC3339),
		})

		require.NoError(t, err)
		assert.Equal(t, "ref-existing", result.RefundID)
		assert.Equal(t, "pending", result.Status)
		assert.Equal(t, 5500, result.RefundAmount)
		d.refundWriteRepo.AssertNotCalled(t, "CreateRefund", mock.Anything, mock.Anything)
		d.paymentWriteRepo.AssertNotCalled(t, "UpdatePaymentStatus",
			mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		assert.Empty(t, d.paymentHistory(), "rejeu : rien de nouveau à historiser")
		assert.Empty(t, d.refundHistory())
	})

	t.Run("idempotent - demande concurrente : la violation d'unicite retourne le refund gagnant", func(t *testing.T) {
		d := newTestService()

		payment := fixtures.NewTestPayment(
			fixtures.WithPaymentID("pay-race"),
			fixtures.WithPaymentBookingID("booking-race"),
			fixtures.WithPaymentStatus(domain.PaymentStatusHeld),
		)
		winner := fixtures.NewTestRefund(fixtures.WithRefundID("ref-winner"), fixtures.WithRefundPaymentID("pay-race"))
		d.paymentReadRepo.On("GetByBookingID", mock.Anything, "booking-race").Return(payment, nil)
		d.refundReadRepo.On("GetByPaymentID", mock.Anything, "pay-race").Return(nil, paymentErrors.ErrorRefundNotFound).Once()
		d.refundReadRepo.On("GetByPaymentID", mock.Anything, "pay-race").Return(winner, nil)
		d.refundWriteRepo.On("CreateRefund", mock.Anything, mock.Anything).Return(paymentErrors.ErrorRefundAlreadyProcessed)
		d.paymentWriteRepo.On("UpdatePaymentStatus", mock.Anything, "pay-race", domain.PaymentStatusRefunded, "").Return(nil)

		result, err := d.svc.RequestRefund(context.Background(), &serviceInterfaces.RequestRefundInput{
			BookingID:         "booking-race",
			RefundReason:      "tripCancelled",
			OriginalAmount:    5000,
			ServiceFee:        500,
			DepartureDatetime: departure.Format(time.RFC3339),
			CancelledAt:       cancelledAt.Format(time.RFC3339),
		})

		require.NoError(t, err)
		assert.Equal(t, "ref-winner", result.RefundID)
		d.refundWriteRepo.AssertNumberOfCalls(t, "CreateRefund", 1)
		// Le perdant n'historise pas sa création avortée ; il rattrape le passage en refunded
		assert.Empty(t, d.refundHistory())
		assert.Equal(t, []string{"refunded"}, d.paymentHistoryStatuses("pay-race"))
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

	t.Run("depart inconnu : accepte hors annulation passager", func(t *testing.T) {
		d := newTestService()

		payment := fixtures.NewTestPayment(
			fixtures.WithPaymentID("pay-no-departure"),
			fixtures.WithPaymentStatus(domain.PaymentStatusHeld),
		)
		d.paymentReadRepo.On("GetByBookingID", mock.Anything, "booking-no-departure").Return(payment, nil)
		d.refundWriteRepo.On("CreateRefund", mock.Anything, mock.MatchedBy(func(r *domain.Refund) bool {
			return r.RefundReason == domain.RefundReasonWaypointCancelled &&
				r.RefundRuleApplied == domain.RefundRuleDriverCancellation
		})).Return(nil)
		d.paymentWriteRepo.On("UpdatePaymentStatus", mock.Anything, "pay-no-departure", domain.PaymentStatusRefunded, "").
			Return(nil)

		result, err := d.svc.RequestRefund(context.Background(), &serviceInterfaces.RequestRefundInput{
			BookingID:      "booking-no-departure",
			RefundReason:   "waypointCancelled",
			OriginalAmount: 5000,
			ServiceFee:     500,
			CancelledAt:    time.Now().UTC().Format(time.RFC3339),
		})

		require.NoError(t, err)
		assert.NotEmpty(t, result.RefundID)
	})

	t.Run("erreur - depart inconnu pour une annulation passager", func(t *testing.T) {
		d := newTestService()

		payment := fixtures.NewTestPayment(fixtures.WithPaymentStatus(domain.PaymentStatusHeld))
		d.paymentReadRepo.On("GetByBookingID", mock.Anything, "booking-pax-no-departure").Return(payment, nil)

		result, err := d.svc.RequestRefund(context.Background(), &serviceInterfaces.RequestRefundInput{
			BookingID:      "booking-pax-no-departure",
			RefundReason:   "cancelledByPassenger",
			OriginalAmount: 5000,
			ServiceFee:     500,
			CancelledAt:    time.Now().UTC().Format(time.RFC3339),
		})

		assert.Nil(t, result)
		assert.ErrorIs(t, err, paymentErrors.ErrorInvalidInput)
		d.refundWriteRepo.AssertNotCalled(t, "CreateRefund", mock.Anything, mock.Anything)
	})
}

// =============================================================================
// GetRefundStatus
// =============================================================================

func TestGetRefundStatus(t *testing.T) {
	t.Run("succes - retourne le statut du remboursement", func(t *testing.T) {
		d := newTestService()
		ctx := passengerCtx()

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
		d.expectBooking("booking-ref", testPassengerID)

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

	t.Run("erreur - appelant autre que le passager", func(t *testing.T) {
		d := newTestService()

		refund := fixtures.NewTestRefund(fixtures.WithRefundID("ref-123"), fixtures.WithRefundBookingID("booking-ref"))
		d.refundReadRepo.On("GetByID", mock.Anything, "ref-123").Return(refund, nil)
		d.expectBooking("booking-ref", "someone-else")

		result, err := d.svc.GetRefundStatus(passengerCtx(), "ref-123")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, paymentErrors.ErrorPaymentNotOwned)
		d.assertExpectations(t)
	})
}

// =============================================================================
// GetRefundByBooking
// =============================================================================

func TestGetRefundByBooking(t *testing.T) {
	t.Run("succes - retourne le remboursement de la reservation", func(t *testing.T) {
		d := newTestService()
		ctx := passengerCtx()

		refund := fixtures.NewTestRefund(
			fixtures.WithRefundID("ref-456"),
			fixtures.WithRefundBookingID("booking-cancelled"),
			fixtures.WithRefundReason(domain.RefundReasonCancelledByDriver),
			fixtures.WithRefundRule(domain.RefundRuleDriverCancellation),
			fixtures.WithRefundAmount(5500),
			fixtures.WithRefundPercentage(100),
			fixtures.WithAmountToPassenger(5500),
			fixtures.WithAmountToDriver(0),
			fixtures.WithAmountToPlatform(0),
			fixtures.WithRefundStatus(domain.RefundStatusPending),
		)
		d.refundReadRepo.On("GetByBookingID", mock.Anything, "booking-cancelled").Return(refund, nil)
		d.expectBooking("booking-cancelled", testPassengerID)

		result, err := d.svc.GetRefundByBooking(ctx, "booking-cancelled")

		require.NoError(t, err)
		assert.Equal(t, "ref-456", result.RefundID)
		assert.Equal(t, "booking-cancelled", result.BookingID)
		assert.Equal(t, "cancelledByDriver", result.RefundReason)
		assert.Equal(t, "driverCancellation", result.RefundRule)
		assert.Equal(t, 5500, result.RefundAmount)
		assert.Equal(t, 5500, result.AmountToPassenger)
		assert.Equal(t, "pending", result.Status)
		d.assertExpectations(t)
	})

	t.Run("erreur - aucun remboursement pour cette reservation", func(t *testing.T) {
		d := newTestService()
		ctx := passengerCtx()

		d.refundReadRepo.On("GetByBookingID", mock.Anything, "booking-paid").Return(nil, paymentErrors.ErrorRefundNotFound)
		d.expectBooking("booking-paid", testPassengerID)

		result, err := d.svc.GetRefundByBooking(ctx, "booking-paid")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, paymentErrors.ErrorRefundNotFound)
		d.assertExpectations(t)
	})

	t.Run("erreur - BookingId vide", func(t *testing.T) {
		d := newTestService()
		ctx := context.Background()

		result, err := d.svc.GetRefundByBooking(ctx, "")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, paymentErrors.ErrorInvalidInput)
		d.refundReadRepo.AssertNotCalled(t, "GetByBookingID", mock.Anything, mock.Anything)
	})

	t.Run("erreur - appelant autre que le passager : aucune lecture du remboursement", func(t *testing.T) {
		d := newTestService()
		d.expectBooking("booking-cancelled", "someone-else")

		result, err := d.svc.GetRefundByBooking(passengerCtx(), "booking-cancelled")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, paymentErrors.ErrorPaymentNotOwned)
		d.refundReadRepo.AssertNotCalled(t, "GetByBookingID", mock.Anything, mock.Anything)
		d.assertExpectations(t)
	})
}

// =============================================================================
// GetPayoutStatus
// =============================================================================

func TestGetPayoutStatus(t *testing.T) {
	t.Run("succes - retourne le statut du payout", func(t *testing.T) {
		d := newTestService()
		ctx := callerCtx("driver-abc")

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
		ctx := callerCtx("driver-fail")

		payout := fixtures.NewTestPayout(
			fixtures.WithPayoutID("payout-fail"),
			fixtures.WithPayoutDriverID("driver-fail"),
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

	t.Run("erreur - accès refusé, le payout appartient à un autre chauffeur", func(t *testing.T) {
		d := newTestService()
		ctx := callerCtx("intruder")

		payout := fixtures.NewTestPayout(
			fixtures.WithPayoutID("payout-other"),
			fixtures.WithPayoutDriverID("driver-owner"),
		)
		d.payoutReadRepo.On("GetByID", mock.Anything, "payout-other").Return(payout, nil)

		result, err := d.svc.GetPayoutStatus(ctx, "payout-other")

		assert.Nil(t, result)
		assert.ErrorIs(t, err, paymentErrors.ErrorPaymentNotOwned)
		d.assertExpectations(t)
	})
}

// =============================================================================
// GetDriverPayouts
// =============================================================================

func TestGetDriverPayouts(t *testing.T) {
	t.Run("succes - retourne la liste des payouts", func(t *testing.T) {
		d := newTestService()
		ctx := callerCtx("driver-1")

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

		results, err := d.svc.GetDriverPayouts(ctx, 0)

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
		ctx := callerCtx("driver-empty")

		d.payoutReadRepo.On("GetDriverPayouts", mock.Anything, "driver-empty", 0).Return([]*domain.Payout{}, nil)

		results, err := d.svc.GetDriverPayouts(ctx, 0)

		require.NoError(t, err)
		assert.Empty(t, results)
		d.assertExpectations(t)
	})

	t.Run("erreur - appelant non résolu : aucune lecture des payouts", func(t *testing.T) {
		d := newTestService()

		results, err := d.svc.GetDriverPayouts(context.Background(), 0)

		assert.Nil(t, results)
		assert.ErrorIs(t, err, paymentErrors.ErrorUnauthorized)
		d.payoutReadRepo.AssertNotCalled(t, "GetDriverPayouts", mock.Anything, mock.Anything, mock.Anything)
	})
}
