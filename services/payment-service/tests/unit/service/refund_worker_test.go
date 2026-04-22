package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/config"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/fedapay"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/service"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/payment-service/internal/service/interfaces"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
	"github.com/Kpeewu/tissi-mah/services/payment-service/tests/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// newRefundWorkerTestService construit un service avec un fedapay client non-nil (zero-value)
// afin de franchir le garde-fou early-return dans ProcessPendingRefunds. Les tests ne
// doivent exercer que les chemins qui échouent AVANT l'appel FedaPay — sinon nil deref.
func newRefundWorkerTestService(t *testing.T, refundMaxRetries int) *testDeps {
	t.Helper()
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
		Payout: config.PayoutConfig{IntervalSeconds: 1800},
		RefundWorker: config.RefundWorkerConfig{
			IntervalSeconds: 60,
			MaxRetries:      refundMaxRetries,
			BatchSize:       20,
		},
		Worker: config.WorkerConfig{RefundMaxConcurrent: 2},
	}

	// fedapay.Client zero-value → non-nil, mais aucun test ici ne doit invoquer de méthode dessus.
	fedapayClient := &fedapay.Client{}
	logger := zap.NewNop()

	d.svc = service.NewPaymentService(
		d.paymentReadRepo, d.paymentWriteRepo,
		d.refundReadRepo, d.refundWriteRepo,
		d.payoutReadRepo, d.payoutWriteRepo,
		d.payoutHistoryRepo,
		d.bookingClient, d.userClient, d.supportClient,
		fedapayClient,
		nil, // cache
		nil, // notifRedis
		cfg,
		logger,
	)
	return d
}

// =============================================================================
// ProcessPendingRefunds — garde-fous
// =============================================================================

func TestProcessPendingRefunds_NoRefundsToProcess(t *testing.T) {
	t.Run("liste vide - ne fait aucun traitement", func(t *testing.T) {
		d := newRefundWorkerTestService(t, 3)
		d.refundReadRepo.On("GetPendingRefundsForPayout", mock.Anything, 20).
			Return([]*domain.Refund{}, nil)

		impl := service.AsImpl(d.svc)
		require.NotNil(t, impl)

		err := impl.ProcessPendingRefunds(context.Background())

		require.NoError(t, err)
		d.assertExpectations(t)
	})

	t.Run("erreur DB sur lecture batch - propage", func(t *testing.T) {
		d := newRefundWorkerTestService(t, 3)
		d.refundReadRepo.On("GetPendingRefundsForPayout", mock.Anything, 20).
			Return(nil, paymentErrors.ErrorDataRetrievalFailed)

		impl := service.AsImpl(d.svc)
		err := impl.ProcessPendingRefunds(context.Background())

		require.Error(t, err)
		require.ErrorIs(t, err, paymentErrors.ErrorDataRetrievalFailed)
		d.assertExpectations(t)
	})
}

// =============================================================================
// processRefund — chemins d'échec avant FedaPay
// =============================================================================

func TestProcessPendingRefunds_MaxRetriesExceeded(t *testing.T) {
	d := newRefundWorkerTestService(t, 3)
	ctx := context.Background()

	refund := &domain.Refund{
		RefundID:          "ref-maxretries",
		PaymentID:         "pay-1",
		BookingID:         "book-1",
		AmountToPassenger: 5000,
		Status:            domain.RefundStatusPending,
		RetryCount:        3, // == MaxRetries
	}

	d.refundReadRepo.On("GetPendingRefundsForPayout", mock.Anything, 20).
		Return([]*domain.Refund{refund}, nil)
	// MarkRefundFailed appelé SANS incrémenter (sortie définitive de la file).
	d.refundWriteRepo.On("MarkRefundFailed", mock.Anything, "ref-maxretries",
		mock.MatchedBy(func(reason string) bool {
			return len(reason) > 0 && reason[:len("max retries")] == "max retries"
		}),
		false,
	).Return(nil)

	impl := service.AsImpl(d.svc)
	err := impl.ProcessPendingRefunds(ctx)

	require.NoError(t, err)
	// Aucun appel à paymentReadRepo, bookingClient, userClient — max retries court-circuite.
	d.paymentReadRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
	d.bookingClient.AssertNotCalled(t, "GetBookingDetails", mock.Anything, mock.Anything)
	d.assertExpectations(t)
}

func TestProcessPendingRefunds_PaymentLookupFails(t *testing.T) {
	d := newRefundWorkerTestService(t, 3)
	ctx := context.Background()

	refund := &domain.Refund{
		RefundID:          "ref-no-payment",
		PaymentID:         "pay-missing",
		BookingID:         "book-1",
		AmountToPassenger: 5000,
		Status:            domain.RefundStatusPending,
	}

	d.refundReadRepo.On("GetPendingRefundsForPayout", mock.Anything, 20).
		Return([]*domain.Refund{refund}, nil)
	d.paymentReadRepo.On("GetByID", mock.Anything, "pay-missing").
		Return(nil, paymentErrors.ErrorPaymentNotFound)
	// Echec sur lecture paiement est retryable.
	d.refundWriteRepo.On("MarkRefundFailed", mock.Anything, "ref-no-payment",
		mock.MatchedBy(func(reason string) bool {
			return len(reason) >= len("payment not found") && reason[:len("payment not found")] == "payment not found"
		}),
		true,
	).Return(nil)

	impl := service.AsImpl(d.svc)
	err := impl.ProcessPendingRefunds(ctx)

	require.NoError(t, err)
	d.bookingClient.AssertNotCalled(t, "GetBookingDetails", mock.Anything, mock.Anything)
	d.assertExpectations(t)
}

func TestProcessPendingRefunds_EmptyPassengerPhoneNumber(t *testing.T) {
	d := newRefundWorkerTestService(t, 3)
	ctx := context.Background()

	refund := &domain.Refund{
		RefundID:          "ref-no-phone",
		PaymentID:         "pay-1",
		BookingID:         "book-1",
		AmountToPassenger: 5000,
		Status:            domain.RefundStatusPending,
		PayoutDestination: "", // aucun fallback
	}
	payment := &domain.Payment{
		PaymentID:            "pay-1",
		BookingID:            "book-1",
		Amount:               5000,
		PassengerPhoneNumber: "", // absent
		Status:               domain.PaymentStatusRefunded,
	}

	d.refundReadRepo.On("GetPendingRefundsForPayout", mock.Anything, 20).
		Return([]*domain.Refund{refund}, nil)
	d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(payment, nil)
	// Numéro manquant est irrécupérable → MarkRefundFailed SANS incrément retry.
	d.refundWriteRepo.On("MarkRefundFailed", mock.Anything, "ref-no-phone",
		"missing passenger phone number", false,
	).Return(nil)

	impl := service.AsImpl(d.svc)
	err := impl.ProcessPendingRefunds(ctx)

	require.NoError(t, err)
	d.bookingClient.AssertNotCalled(t, "GetBookingDetails", mock.Anything, mock.Anything)
	d.assertExpectations(t)
}

func TestProcessPendingRefunds_UsesPayoutDestinationFallback(t *testing.T) {
	// Un refund créé avec l'ancien code peut avoir PayoutDestination rempli mais
	// Payment.PassengerPhoneNumber vide. Le worker doit prendre le fallback sans failer.
	d := newRefundWorkerTestService(t, 3)
	ctx := context.Background()

	refund := &domain.Refund{
		RefundID:          "ref-fallback",
		PaymentID:         "pay-1",
		BookingID:         "book-1",
		AmountToPassenger: 5000,
		Status:            domain.RefundStatusPending,
		PayoutDestination: "90009999", // fallback
	}
	payment := &domain.Payment{
		PaymentID:            "pay-1",
		BookingID:            "book-1",
		Amount:               5000,
		PassengerPhoneNumber: "", // absent sur Payment, mais refund a le fallback
	}

	d.refundReadRepo.On("GetPendingRefundsForPayout", mock.Anything, 20).
		Return([]*domain.Refund{refund}, nil)
	d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(payment, nil)
	// On court-circuite au lookup booking pour éviter l'appel FedaPay dans ce test unitaire.
	d.bookingClient.On("GetBookingDetails", mock.Anything, "book-1").
		Return(nil, errors.New("booking-service unavailable"))
	d.refundWriteRepo.On("MarkRefundFailed", mock.Anything, "ref-fallback",
		mock.MatchedBy(func(reason string) bool {
			return len(reason) >= len("booking lookup failed") && reason[:len("booking lookup failed")] == "booking lookup failed"
		}),
		true,
	).Return(nil)

	impl := service.AsImpl(d.svc)
	err := impl.ProcessPendingRefunds(ctx)

	require.NoError(t, err)
	// La clé : MarkRefundFailed(missing phone) NE DOIT PAS être appelé ici — le fallback PayoutDestination a suffi.
	d.assertExpectations(t)
}

func TestProcessPendingRefunds_BookingLookupFails(t *testing.T) {
	d := newRefundWorkerTestService(t, 3)
	ctx := context.Background()

	refund := &domain.Refund{
		RefundID:          "ref-no-booking",
		PaymentID:         "pay-1",
		BookingID:         "book-missing",
		AmountToPassenger: 5000,
		Status:            domain.RefundStatusPending,
	}
	payment := &domain.Payment{
		PaymentID:            "pay-1",
		BookingID:            "book-missing",
		Amount:               5000,
		PassengerPhoneNumber: "90001122",
	}

	d.refundReadRepo.On("GetPendingRefundsForPayout", mock.Anything, 20).
		Return([]*domain.Refund{refund}, nil)
	d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(payment, nil)
	d.bookingClient.On("GetBookingDetails", mock.Anything, "book-missing").
		Return(nil, errors.New("not found"))
	d.refundWriteRepo.On("MarkRefundFailed", mock.Anything, "ref-no-booking",
		mock.MatchedBy(func(reason string) bool {
			return len(reason) >= len("booking lookup failed") && reason[:len("booking lookup failed")] == "booking lookup failed"
		}),
		true,
	).Return(nil)

	impl := service.AsImpl(d.svc)
	err := impl.ProcessPendingRefunds(ctx)

	require.NoError(t, err)
	d.userClient.AssertNotCalled(t, "GetUserByUserID", mock.Anything, mock.Anything)
	d.assertExpectations(t)
}

func TestProcessPendingRefunds_UserLookupFails(t *testing.T) {
	d := newRefundWorkerTestService(t, 3)
	ctx := context.Background()

	refund := &domain.Refund{
		RefundID:          "ref-no-user",
		PaymentID:         "pay-1",
		BookingID:         "book-1",
		AmountToPassenger: 5000,
		Status:            domain.RefundStatusPending,
	}
	payment := &domain.Payment{
		PaymentID:            "pay-1",
		BookingID:            "book-1",
		Amount:               5000,
		PassengerPhoneNumber: "90001122",
	}
	booking := &client.BookingDetails{
		BookingID:   "book-1",
		PassengerID: "user-missing",
	}

	d.refundReadRepo.On("GetPendingRefundsForPayout", mock.Anything, 20).
		Return([]*domain.Refund{refund}, nil)
	d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(payment, nil)
	d.bookingClient.On("GetBookingDetails", mock.Anything, "book-1").Return(booking, nil)
	d.userClient.On("GetUserByUserID", mock.Anything, "user-missing").
		Return(nil, errors.New("user-service down"))
	d.refundWriteRepo.On("MarkRefundFailed", mock.Anything, "ref-no-user",
		mock.MatchedBy(func(reason string) bool {
			return len(reason) >= len("user lookup failed") && reason[:len("user lookup failed")] == "user lookup failed"
		}),
		true,
	).Return(nil)

	impl := service.AsImpl(d.svc)
	err := impl.ProcessPendingRefunds(ctx)

	require.NoError(t, err)
	d.refundWriteRepo.AssertNotCalled(t, "MarkRefundProcessing", mock.Anything, mock.Anything)
	d.assertExpectations(t)
}

func TestProcessPendingRefunds_MarkProcessingAlreadyProcessed(t *testing.T) {
	// Si MarkRefundProcessing retourne ErrorRefundAlreadyProcessed (course avec un autre worker),
	// le refund est skip silencieusement, aucun appel FedaPay n'est fait.
	d := newRefundWorkerTestService(t, 3)
	ctx := context.Background()

	refund := &domain.Refund{
		RefundID:          "ref-already",
		PaymentID:         "pay-1",
		BookingID:         "book-1",
		AmountToPassenger: 5000,
		Status:            domain.RefundStatusPending,
	}
	payment := &domain.Payment{
		PaymentID:            "pay-1",
		BookingID:            "book-1",
		Amount:               5000,
		PassengerPhoneNumber: "90001122",
	}
	booking := &client.BookingDetails{BookingID: "book-1", PassengerID: "user-1"}
	user := &client.UserInfo{UserID: "user-1", FirstName: "Awa", Name: "Diallo"}

	d.refundReadRepo.On("GetPendingRefundsForPayout", mock.Anything, 20).
		Return([]*domain.Refund{refund}, nil)
	d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(payment, nil)
	d.bookingClient.On("GetBookingDetails", mock.Anything, "book-1").Return(booking, nil)
	d.userClient.On("GetUserByUserID", mock.Anything, "user-1").Return(user, nil)
	d.refundWriteRepo.On("MarkRefundProcessing", mock.Anything, "ref-already").
		Return(paymentErrors.ErrorRefundAlreadyProcessed)

	impl := service.AsImpl(d.svc)
	err := impl.ProcessPendingRefunds(ctx)

	require.NoError(t, err)
	// Aucun MarkRefundFailed ni MarkRefundCompleted ne doit être appelé en cas de concurrence.
	d.refundWriteRepo.AssertNotCalled(t, "MarkRefundFailed", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	d.refundWriteRepo.AssertNotCalled(t, "MarkRefundCompleted", mock.Anything, mock.Anything, mock.Anything)
	d.assertExpectations(t)
}

// =============================================================================
// Check de signature — RequestRefundInput est accessible depuis le package service.
// (Permet de détecter un drift d'API.)
// =============================================================================

var _ = serviceInterfaces.RequestRefundInput{}
