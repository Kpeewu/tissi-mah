package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/fedapay"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/service"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/payment-service/internal/service/interfaces"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
	"github.com/Kpeewu/tissi-mah/services/payment-service/tests/mocks"
)

// =============================================================================
// Écriture de l'historique — cas non couverts par les scénarios existants
// =============================================================================

func TestPaymentHistory_EdgeCases(t *testing.T) {
	t.Run("declined répété : une seule entrée attempt_failed", func(t *testing.T) {
		d := newCollectTestService()
		payload := []byte(`{"name":"transaction.declined","entity":{"id":101,"status":"declined"}}`)
		alreadyDeclined := pendingPayment()
		alreadyDeclined.ProviderStatus = "declined"
		d.fedapay.On("VerifyWebhookSignature", "sig", payload).Return(true)
		d.paymentWriteRepo.On("SaveWebhookEvent", mock.Anything, mock.Anything).Return(nil)
		d.paymentReadRepo.On("GetByExternalTransactionID", mock.Anything, "101").Return(alreadyDeclined, nil)
		d.paymentWriteRepo.On("UpdateProviderStatus", mock.Anything, "pay-1", "declined").Return(nil)
		d.paymentWriteRepo.On("MarkWebhookEventProcessed", mock.Anything, mock.Anything).Return(nil)

		err := d.svc.ProcessWebhook(context.Background(), &serviceInterfaces.ProcessWebhookInput{
			Signature: "sig", RawPayload: payload,
		})

		require.NoError(t, err)
		assert.Empty(t, d.paymentHistory())
	})

	t.Run("approbation déjà appliquée en concurrence : pas d'entrée held en double", func(t *testing.T) {
		d := newCollectTestService()
		d.paymentReadRepo.On("GetByExternalTransactionID", mock.Anything, "101").Return(pendingPayment(), nil)
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(withStatus(pendingPayment(), domain.PaymentStatusHeld), nil)
		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-1").Return(payableBooking(time.Now().UTC()), nil)
		d.fedapay.On("GetTransaction", 101).Return(&fedapay.Transaction{ID: 101, Status: "approved"}, nil)
		d.paymentWriteRepo.On("UpdatePaymentStatus", mock.Anything, "pay-1", domain.PaymentStatusHeld, "101").
			Return(paymentErrors.ErrorPaymentAlreadyProcessed)

		_, err := d.svc.HandlePaymentCallback(context.Background(), "101")

		require.NoError(t, err)
		assert.Empty(t, d.paymentHistory())
	})

	t.Run("lien de paiement régénéré : payment_url_renewed", func(t *testing.T) {
		d := newCollectTestService()
		withoutURL := pendingPayment()
		withoutURL.PaymentURL = ""
		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-1").Return(payableBooking(time.Now().UTC()), nil)
		d.paymentReadRepo.On("GetPendingByBookingID", mock.Anything, "bk-1").Return(withoutURL, nil)
		d.fedapay.On("GetTransactionToken", 101).Return(&fedapay.TokenResponse{URL: "https://checkout.fedapay.com/pay/new"}, nil)
		d.paymentWriteRepo.On("UpdatePaymentURL", mock.Anything, "pay-1", "https://checkout.fedapay.com/pay/new").Return(nil)

		result, err := d.svc.CreatePayment(passengerCtx(), &serviceInterfaces.CreatePaymentInput{
			BookingID: "bk-1", PaymentMethod: "mobileMoney", PassengerPhoneNumber: "90112233",
		})

		require.NoError(t, err)
		assert.True(t, result.Reused)
		history := d.paymentHistory()
		require.Len(t, history, 1)
		assert.Equal(t, domain.PaymentEventURLRenewed, history[0].Status)
		assert.Equal(t, domain.HistoryInitiatedByPassenger, history[0].InitiatedBy)
		assert.Equal(t, domain.HistorySourceCreatePayment, history[0].Source)
	})

	t.Run("remboursement intégral en échec au plafond : refund_creation_failed", func(t *testing.T) {
		d := newCollectTestService()
		payment := unsyncedHeldPayment("pay-7", "bk-7", "707", 5)
		d.paymentReadRepo.On("GetUnsyncedHeldPayments", mock.Anything, mock.Anything, 100).
			Return([]*domain.Payment{payment}, nil)
		d.refundReadRepo.On("GetByPaymentID", mock.Anything, "pay-7").Return(nil, paymentErrors.ErrorRefundNotFound)
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-7").Return(payment, nil)
		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-7").Return(nil, errors.New("booking down"))
		d.refundWriteRepo.On("CreateRefund", mock.Anything, mock.Anything).Return(paymentErrors.ErrorInternalServer)

		require.NoError(t, service.AsImpl(d.svc).RetryUnsyncedHeldPayments(context.Background()))

		history := d.paymentHistory()
		require.Len(t, history, 1)
		assert.Equal(t, domain.PaymentEventRefundCreationFailed, history[0].Status)
		assert.Contains(t, history[0].Notes, "bookingUnavailable")
		assert.Empty(t, d.refundHistory())
	})

	t.Run("versement manuel : paiements passés en paidOut, origine support", func(t *testing.T) {
		d := newCollectTestService()
		released := withStatus(pendingPayment(), domain.PaymentStatusReleased)
		d.payoutReadRepo.On("IsTripReadyForPayout", mock.Anything, "trip-1").Return(true, nil)
		d.supportClient.On("GetSupportUserByID", mock.Anything, "support-1").Return(&client.SupportUserInfo{
			UserID: "support-1", FirstName: "Awa", LastName: "Diallo", Role: "support", IsActive: true,
		}, nil)
		d.payoutReadRepo.On("GetReleasedPaymentsForTrip", mock.Anything, "trip-1").Return([]*domain.Payment{released}, nil)
		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-1").Return(payableBooking(time.Now().UTC()), nil)
		d.userClient.On("GetUserByUserID", mock.Anything, "driver-1").
			Return(&client.UserInfo{UserID: "driver-1", FirstName: "Kodjo", Name: "Mensah", WithdrawNumber: "90998877"}, nil)
		d.fedapay.On("CreatePayout", 5500, "togocel", "90998877", "Kodjo", "Mensah").Return(&fedapay.PayoutItem{ID: 888}, nil)
		d.payoutWriteRepo.On("CreatePayout", mock.Anything, mock.Anything).Return(nil)
		d.payoutHistoryRepo.On("CreateHistoryEntry", mock.Anything, mock.Anything).Return(nil)
		d.fedapay.On("StartPayouts", []int{888}).Return([]fedapay.PayoutItem{{ID: 888}}, nil)
		d.payoutWriteRepo.On("UpdatePayoutStatus", mock.Anything, mock.Anything, domain.PayoutStatusProcessing, "888").Return(nil)
		d.payoutWriteRepo.On("MarkPaymentsAsPaidOut", mock.Anything, "trip-1").Return([]string{"pay-1", "pay-9"}, nil)

		netAmount, err := d.svc.TriggerManualPayout(context.Background(), "trip-1", "support-1")

		require.NoError(t, err)
		assert.Equal(t, 5500, netAmount)
		assert.Equal(t, []string{"paidOut"}, d.paymentHistoryStatuses("pay-1"))
		assert.Equal(t, []string{"paidOut"}, d.paymentHistoryStatuses("pay-9"))
		for _, e := range d.paymentHistory() {
			assert.Equal(t, domain.HistoryInitiatedBySupport, e.InitiatedBy)
			assert.Equal(t, domain.HistorySourceSupportManualPayout, e.Source)
			assert.Contains(t, e.Notes, "Payout PYO-")
		}
	})
}

func TestRefundHistory_EdgeCases(t *testing.T) {
	booking := &client.BookingDetails{BookingID: "book-1", PassengerID: "user-1"}
	refund := func() *domain.Refund {
		return &domain.Refund{
			RefundID: "ref-1", PaymentID: "pay-1", BookingID: "book-1",
			AmountToPassenger: 5500, Status: domain.RefundStatusPending, RetryCount: 1,
		}
	}

	t.Run("virement FedaPay en échec : processing puis retry_scheduled", func(t *testing.T) {
		fedapayMock := new(mocks.MockFedaPayClient)
		d := buildRefundWorkerTestService(t, 3, fedapayMock, nil)
		d.refundReadRepo.On("GetPendingRefundsForPayout", mock.Anything, 20).Return([]*domain.Refund{refund()}, nil)
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").
			Return(&domain.Payment{PaymentID: "pay-1", PassengerPhoneNumber: "90112233"}, nil)
		d.bookingClient.On("GetBookingDetails", mock.Anything, "book-1").Return(booking, nil)
		d.userClient.On("GetUserByUserID", mock.Anything, "user-1").Return(&client.UserInfo{FirstName: "Awa", Name: "Diallo"}, nil)
		d.refundWriteRepo.On("MarkRefundProcessing", mock.Anything, "ref-1").Return(nil)
		fedapayMock.On("CreatePayout", 5500, "togocel", "90112233", "Awa", "Diallo").Return(nil, errors.New("fedapay down"))
		d.refundWriteRepo.On("MarkRefundFailed", mock.Anything, "ref-1", "fedapay: fedapay down", true).Return(nil)

		require.NoError(t, service.AsImpl(d.svc).ProcessPendingRefunds(context.Background()))

		history := d.refundHistory()
		require.Len(t, history, 2)
		assert.Equal(t, "processing", history[0].Status)
		assert.Equal(t, int16(1), history[0].RetryCount)
		assert.Equal(t, domain.RefundEventRetryScheduled, history[1].Status)
		assert.Equal(t, int16(2), history[1].RetryCount)
		assert.Equal(t, "fedapay: fedapay down", history[1].Notes)
	})

	t.Run("échec d'écriture du refund : pas d'entrée", func(t *testing.T) {
		d := newRefundWorkerTestService(t, 3)
		d.refundReadRepo.On("GetPendingRefundsForPayout", mock.Anything, 20).Return([]*domain.Refund{refund()}, nil)
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(nil, paymentErrors.ErrorPaymentNotFound)
		d.refundWriteRepo.On("MarkRefundFailed", mock.Anything, "ref-1", mock.Anything, true).
			Return(paymentErrors.ErrorInternalServer)

		require.NoError(t, service.AsImpl(d.svc).ProcessPendingRefunds(context.Background()))

		assert.Empty(t, d.refundHistory())
	})
}

// =============================================================================
// Consultation de l'historique (support)
// =============================================================================

func TestGetPaymentHistory(t *testing.T) {
	occurredAt := time.Date(2026, 10, 12, 8, 30, 0, 0, time.UTC)
	entries := []*domain.PaymentStatusHistory{
		{HistoryID: "h-1", PaymentID: "pay-1", Status: "pending", ProviderStatus: "pending",
			InitiatedBy: "passenger", Source: "create_payment", OccurredAt: occurredAt, Notes: "créé"},
		{HistoryID: "h-2", PaymentID: "pay-1", Status: "held", ProviderStatus: "approved",
			InitiatedBy: "system", Source: "fedapay_webhook", OccurredAt: occurredAt.Add(time.Minute)},
	}

	t.Run("par paiement", func(t *testing.T) {
		d := newTestService()
		d.paymentHistoryRepo.On("ListByPaymentID", mock.Anything, "pay-1").Return(entries, nil)

		results, err := d.svc.GetPaymentHistory(context.Background(), "pay-1", "bk-ignored")

		require.NoError(t, err)
		require.Len(t, results, 2)
		assert.Equal(t, serviceInterfaces.PaymentHistoryEntryResult{
			HistoryID: "h-1", PaymentID: "pay-1", Status: "pending", ProviderStatus: "pending",
			InitiatedBy: "passenger", Source: "create_payment", OccurredAt: "2026-10-12T08:30:00Z", Notes: "créé",
		}, *results[0])
		assert.Equal(t, "held", results[1].Status)
		d.paymentHistoryRepo.AssertNotCalled(t, "ListByBookingID", mock.Anything, mock.Anything)
	})

	t.Run("par réservation, liste vide", func(t *testing.T) {
		d := newTestService()
		d.paymentHistoryRepo.On("ListByBookingID", mock.Anything, "bk-1").Return([]*domain.PaymentStatusHistory{}, nil)

		results, err := d.svc.GetPaymentHistory(context.Background(), "", "bk-1")

		require.NoError(t, err)
		assert.Empty(t, results)
		assert.NotNil(t, results)
	})

	t.Run("erreur - aucun identifiant", func(t *testing.T) {
		d := newTestService()

		_, err := d.svc.GetPaymentHistory(context.Background(), "", "")

		assert.ErrorIs(t, err, paymentErrors.ErrorInvalidInput)
	})

	t.Run("erreur DB propagée", func(t *testing.T) {
		d := newTestService()
		d.paymentHistoryRepo.On("ListByPaymentID", mock.Anything, "pay-1").Return(nil, paymentErrors.ErrorDataRetrievalFailed)

		_, err := d.svc.GetPaymentHistory(context.Background(), "pay-1", "")

		assert.ErrorIs(t, err, paymentErrors.ErrorDataRetrievalFailed)
	})
}

func TestGetRefundHistory(t *testing.T) {
	t.Run("par remboursement", func(t *testing.T) {
		d := newTestService()
		d.refundHistoryRepo.On("ListByRefundID", mock.Anything, "ref-1").Return([]*domain.RefundStatusHistory{
			{HistoryID: "h-1", RefundID: "ref-1", Status: "retry_scheduled", RetryCount: 2,
				InitiatedBy: "system", Source: "refund_worker", OccurredAt: time.Date(2026, 10, 12, 1, 0, 0, 0, time.UTC)},
		}, nil)

		results, err := d.svc.GetRefundHistory(context.Background(), "ref-1", "")

		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, 2, results[0].RetryCount)
		assert.Equal(t, "refund_worker", results[0].Source)
		assert.Equal(t, "2026-10-12T01:00:00Z", results[0].OccurredAt)
	})

	t.Run("par réservation", func(t *testing.T) {
		d := newTestService()
		d.refundHistoryRepo.On("ListByBookingID", mock.Anything, "bk-1").Return([]*domain.RefundStatusHistory{}, nil)

		results, err := d.svc.GetRefundHistory(context.Background(), "", "bk-1")

		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("erreur - aucun identifiant", func(t *testing.T) {
		d := newTestService()

		_, err := d.svc.GetRefundHistory(context.Background(), "", "")

		assert.ErrorIs(t, err, paymentErrors.ErrorInvalidInput)
	})
}

func TestGetPayoutHistory(t *testing.T) {
	t.Run("retourne l'historique avec l'agent support", func(t *testing.T) {
		d := newTestService()
		d.payoutHistoryRepo.On("ListByPayoutID", mock.Anything, "po-1").Return([]*domain.PayoutStatusHistory{
			{HistoryID: "h-1", PayoutID: "po-1", Status: "launched_by_support", InitiatedBy: "support",
				SupportUserID: "support-1", SupportFirstName: "Awa", SupportLastName: "Diallo",
				OccurredAt: time.Date(2026, 10, 12, 2, 0, 0, 0, time.UTC)},
		}, nil)

		results, err := d.svc.GetPayoutHistory(context.Background(), "po-1")

		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, "launched_by_support", results[0].Status)
		assert.Equal(t, "Awa", results[0].SupportFirstName)
		d.assertExpectations(t)
	})

	t.Run("erreur - identifiant manquant", func(t *testing.T) {
		d := newTestService()

		_, err := d.svc.GetPayoutHistory(context.Background(), "")

		assert.ErrorIs(t, err, paymentErrors.ErrorInvalidInput)
	})
}
