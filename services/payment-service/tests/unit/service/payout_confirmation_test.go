package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/pkg/notification"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/cache"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/config"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/fedapay"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/service"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/payment-service/internal/service/interfaces"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
	"github.com/Kpeewu/tissi-mah/services/payment-service/tests/mocks"
)

func processingRefund() *domain.Refund {
	return &domain.Refund{
		RefundID: "ref-1", PaymentID: "pay-1", BookingID: "bk-1",
		RefundReason: domain.RefundReasonCancelledByDriver, AmountToPassenger: 5500,
		Status: domain.RefundStatusProcessing, PaymentProviderReference: "777",
	}
}

func processingPayout() *domain.Payout {
	return &domain.Payout{
		PayoutID: "po-1", PayoutReference: "PYO-20261012-ABC123", TripID: "trip-1",
		Status: domain.PayoutStatusProcessing, PaymentProviderReference: "888",
	}
}

// sendPayoutWebhook rejoue un webhook payout.* signé et retourne l'erreur de ProcessWebhook.
func sendPayoutWebhook(d *collectDeps, name string, payoutID string) error {
	payload := []byte(`{"name":"` + name + `","entity":{"id":` + payoutID + `,"status":"sent"}}`)
	d.fedapay.On("VerifyWebhookSignature", "sig", payload).Return(true)
	d.paymentWriteRepo.On("SaveWebhookEvent", mock.Anything, mock.Anything).Return(nil)
	d.paymentWriteRepo.On("MarkWebhookEventProcessed", mock.Anything, payoutID+":"+name).Return(nil).Maybe()
	return d.svc.ProcessWebhook(context.Background(), &serviceInterfaces.ProcessWebhookInput{
		Signature: "sig", RawPayload: payload,
	})
}

// =============================================================================
// Remboursements
// =============================================================================

func TestRefundPayoutConfirmation(t *testing.T) {
	t.Run("webhook + FedaPay sent → completed, REFUND_COMPLETED, historique", func(t *testing.T) {
		rdb := newNotifRedis(t)
		d := buildCollectTestService(rdb)
		d.refundReadRepo.On("GetByProviderReference", mock.Anything, "777").Return(processingRefund(), nil)
		d.fedapay.On("GetPayout", 777).Return(&fedapay.PayoutItem{ID: 777, Status: "sent"}, nil)
		d.refundWriteRepo.On("MarkRefundCompleted", mock.Anything, "ref-1", "777").Return(nil)
		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-1").Return(payableBooking(time.Now().UTC()), nil)

		require.NoError(t, sendPayoutWebhook(d, "payout.sent", "777"))

		event := findEvent(publishedEvents(t, rdb), notification.RefundCompleted)
		require.NotNil(t, event, "REFUND_COMPLETED doit être publié à la confirmation")
		assert.Equal(t, testPassengerID, event.UserID)
		assert.Equal(t, "777", event.Payload["payment_provider_reference"])
		history := d.refundHistory()
		require.Len(t, history, 1)
		assert.Equal(t, "completed", history[0].Status)
		assert.Equal(t, domain.HistorySourceFedaPayWebhook, history[0].Source)
		assert.Equal(t, "Virement FedaPay 777 confirmé", history[0].Notes)
		d.paymentWriteRepo.AssertCalled(t, "MarkWebhookEventProcessed", mock.Anything, "777:payout.sent")
	})

	t.Run("FedaPay failed → pending pour nouvelle tentative", func(t *testing.T) {
		d := newCollectTestService()
		d.refundReadRepo.On("GetByProviderReference", mock.Anything, "777").Return(processingRefund(), nil)
		d.fedapay.On("GetPayout", 777).Return(&fedapay.PayoutItem{ID: 777, Status: "failed", FailReason: "invalid number"}, nil)
		d.refundWriteRepo.On("MarkRefundPayoutFailed", mock.Anything, "ref-1", "fedapay payout failed: invalid number").Return(nil)

		require.NoError(t, sendPayoutWebhook(d, "payout.failed", "777"))

		history := d.refundHistory()
		require.Len(t, history, 1)
		assert.Equal(t, domain.RefundEventRetryScheduled, history[0].Status)
		assert.Equal(t, int16(1), history[0].RetryCount)
		d.refundWriteRepo.AssertNotCalled(t, "MarkRefundCompleted", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("statut non final → rien", func(t *testing.T) {
		d := newCollectTestService()
		d.refundReadRepo.On("GetByProviderReference", mock.Anything, "777").Return(processingRefund(), nil)
		d.fedapay.On("GetPayout", 777).Return(&fedapay.PayoutItem{ID: 777, Status: "started"}, nil)

		require.NoError(t, sendPayoutWebhook(d, "payout.started", "777"))

		d.refundWriteRepo.AssertNotCalled(t, "MarkRefundCompleted", mock.Anything, mock.Anything, mock.Anything)
		d.refundWriteRepo.AssertNotCalled(t, "MarkRefundPayoutFailed", mock.Anything, mock.Anything, mock.Anything)
		assert.Empty(t, d.refundHistory())
	})

	t.Run("déjà confirmé (réconciliation passée avant) → ni notification ni historique", func(t *testing.T) {
		rdb := newNotifRedis(t)
		d := buildCollectTestService(rdb)
		d.refundReadRepo.On("GetByProviderReference", mock.Anything, "777").Return(processingRefund(), nil)
		d.fedapay.On("GetPayout", 777).Return(&fedapay.PayoutItem{ID: 777, Status: "sent"}, nil)
		d.refundWriteRepo.On("MarkRefundCompleted", mock.Anything, "ref-1", "777").Return(paymentErrors.ErrorRefundAlreadyProcessed)

		require.NoError(t, sendPayoutWebhook(d, "payout.sent", "777"))

		assert.Empty(t, publishedEvents(t, rdb))
		assert.Empty(t, d.refundHistory())
	})

	t.Run("GetPayout en erreur → erreur remontée, webhook rejouable", func(t *testing.T) {
		d := newCollectTestService()
		d.refundReadRepo.On("GetByProviderReference", mock.Anything, "777").Return(processingRefund(), nil)
		d.fedapay.On("GetPayout", 777).Return(nil, errors.New("fedapay down"))

		err := sendPayoutWebhook(d, "payout.sent", "777")

		assert.ErrorIs(t, err, paymentErrors.ErrorFedaPayAPIError)
		d.paymentWriteRepo.AssertNotCalled(t, "MarkWebhookEventProcessed", mock.Anything, mock.Anything)
	})
}

// =============================================================================
// Payouts chauffeurs
// =============================================================================

func TestDriverPayoutConfirmation(t *testing.T) {
	t.Run("FedaPay sent → completed + historique payout", func(t *testing.T) {
		d := newCollectTestService()
		d.payoutReadRepo.On("GetByProviderReference", mock.Anything, "888").Return(processingPayout(), nil)
		d.fedapay.On("GetPayout", 888).Return(&fedapay.PayoutItem{ID: 888, Status: "sent"}, nil)
		d.payoutWriteRepo.On("ConfirmPayoutCompleted", mock.Anything, "po-1").Return(nil)
		d.payoutHistoryRepo.On("CreateHistoryEntry", mock.Anything, mock.MatchedBy(func(e *domain.PayoutStatusHistory) bool {
			return e.PayoutID == "po-1" && e.Status == "completed" && e.Notes == "Confirmé par FedaPay"
		})).Return(nil)

		require.NoError(t, sendPayoutWebhook(d, "payout.completed", "888"))

		d.payoutHistoryRepo.AssertExpectations(t)
		d.payoutWriteRepo.AssertNotCalled(t, "FailProcessingPayout", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("FedaPay failed → payout failed, paiements remis en released", func(t *testing.T) {
		d := newCollectTestService()
		d.payoutReadRepo.On("GetByProviderReference", mock.Anything, "888").Return(processingPayout(), nil)
		d.fedapay.On("GetPayout", 888).Return(&fedapay.PayoutItem{ID: 888, Status: "failed", FailReason: "account closed"}, nil)
		d.payoutWriteRepo.On("FailProcessingPayout", mock.Anything, "po-1", "trip-1", "fedapay payout failed: account closed").
			Return([]string{"pay-1", "pay-2"}, nil)
		d.payoutHistoryRepo.On("CreateHistoryEntry", mock.Anything, mock.MatchedBy(func(e *domain.PayoutStatusHistory) bool {
			return e.PayoutID == "po-1" && e.Status == "failed" && e.Notes == "fedapay payout failed: account closed"
		})).Return(nil)

		require.NoError(t, sendPayoutWebhook(d, "payout.failed", "888"))

		d.payoutHistoryRepo.AssertExpectations(t)
		assert.Equal(t, []string{"released"}, d.paymentHistoryStatuses("pay-1"))
		assert.Equal(t, []string{"released"}, d.paymentHistoryStatuses("pay-2"))
		assert.Contains(t, d.paymentHistory()[0].Notes, "PYO-20261012-ABC123")
	})

	t.Run("déjà traité → pas d'historique", func(t *testing.T) {
		d := newCollectTestService()
		d.payoutReadRepo.On("GetByProviderReference", mock.Anything, "888").Return(processingPayout(), nil)
		d.fedapay.On("GetPayout", 888).Return(&fedapay.PayoutItem{ID: 888, Status: "sent"}, nil)
		d.payoutWriteRepo.On("ConfirmPayoutCompleted", mock.Anything, "po-1").Return(paymentErrors.ErrorPayoutAlreadyProcessed)

		require.NoError(t, sendPayoutWebhook(d, "payout.completed", "888"))

		d.payoutHistoryRepo.AssertNotCalled(t, "CreateHistoryEntry", mock.Anything, mock.Anything)
	})

	t.Run("référence inconnue → ignorée sans appel FedaPay", func(t *testing.T) {
		d := newCollectTestService()
		d.payoutReadRepo.On("GetByProviderReference", mock.Anything, "999").Return(nil, paymentErrors.ErrorPayoutNotFound)

		require.NoError(t, sendPayoutWebhook(d, "payout.sent", "999"))

		d.fedapay.AssertNotCalled(t, "GetPayout", mock.Anything)
	})
}

// =============================================================================
// Réconciliation (webhooks perdus)
// =============================================================================

// newReconcileTestService construit le service avec un cache Redis (verrou de réconciliation).
func newReconcileTestService(t *testing.T) (*collectDeps, *redis.Client) {
	t.Helper()
	rdb := newNotifRedis(t)
	d := &collectDeps{
		testDeps: &testDeps{
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
		},
		fedapay: new(mocks.MockFedaPayClient),
	}
	d.svc = service.NewPaymentService(
		d.paymentReadRepo, d.paymentWriteRepo,
		d.refundReadRepo, d.refundWriteRepo,
		d.payoutReadRepo, d.payoutWriteRepo,
		d.payoutHistoryRepo, d.paymentHistoryRepo, d.refundHistoryRepo,
		d.bookingClient, d.userClient, d.supportClient,
		d.fedapay,
		cache.NewPaymentCache(rdb, zap.NewNop()),
		nil, // notifRedis
		&config.Config{},
		zap.NewNop(),
	)
	return d, rdb
}

func TestReconcileProviderPayouts(t *testing.T) {
	t.Run("confirme refunds et payouts, une erreur FedaPay n'interrompt pas le lot", func(t *testing.T) {
		d, _ := newReconcileTestService(t)
		stuck := processingRefund()
		stuck.RefundID, stuck.PaymentProviderReference = "ref-2", "778"
		d.refundReadRepo.On("GetProcessingRefundsToReconcile", mock.Anything, mock.Anything, 50).
			Return([]*domain.Refund{stuck, processingRefund()}, nil)
		d.fedapay.On("GetPayout", 778).Return(nil, errors.New("fedapay down"))
		d.fedapay.On("GetPayout", 777).Return(&fedapay.PayoutItem{ID: 777, Status: "sent"}, nil)
		d.refundWriteRepo.On("MarkRefundCompleted", mock.Anything, "ref-1", "777").Return(nil)
		d.payoutReadRepo.On("GetProcessingPayoutsToReconcile", mock.Anything, mock.Anything, 50).
			Return([]*domain.Payout{processingPayout()}, nil)
		d.fedapay.On("GetPayout", 888).Return(&fedapay.PayoutItem{ID: 888, Status: "sent"}, nil)
		d.payoutWriteRepo.On("ConfirmPayoutCompleted", mock.Anything, "po-1").Return(nil)
		d.payoutHistoryRepo.On("CreateHistoryEntry", mock.Anything, mock.Anything).Return(nil)

		require.NoError(t, service.AsImpl(d.svc).ReconcileProviderPayouts(context.Background()))

		d.refundWriteRepo.AssertExpectations(t)
		d.payoutWriteRepo.AssertExpectations(t)
		history := d.refundHistory()
		require.Len(t, history, 1)
		assert.Equal(t, "ref-1", history[0].RefundID)
		assert.Equal(t, domain.HistorySourcePayoutReconciler, history[0].Source)
	})

	t.Run("verrou déjà pris → aucune lecture", func(t *testing.T) {
		d, rdb := newReconcileTestService(t)
		require.NoError(t, rdb.Set(context.Background(), "payment:reconcile:lock", "locked", 0).Err())

		require.NoError(t, service.AsImpl(d.svc).ReconcileProviderPayouts(context.Background()))

		d.refundReadRepo.AssertNotCalled(t, "GetProcessingRefundsToReconcile", mock.Anything, mock.Anything, mock.Anything)
		d.payoutReadRepo.AssertNotCalled(t, "GetProcessingPayoutsToReconcile", mock.Anything, mock.Anything, mock.Anything)
	})
}
