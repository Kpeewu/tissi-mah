package service_test

import (
	"context"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/fedapay"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/service"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// failedPayoutWithRef simule un payout DB en statut failed avec une référence FedaPay existante
// (cas d'un timeout sur StartPayouts lors de la tentative précédente).
func failedPayoutWithRef(tripID, fedaRef string) *domain.Payout {
	return &domain.Payout{
		PayoutID:                 "po-timeout-1",
		TripID:                   tripID,
		Status:                   domain.PayoutStatusFailed,
		PaymentProviderReference: fedaRef,
		NetAmount:                5000,
	}
}

// setupPayoutForTrip configure les mocks nécessaires pour atteindre la vérification
// anti-double-virement dans processPayoutForTrip (via TriggerManualPayout).
func setupPayoutForTrip(d *collectDeps) {
	d.payoutReadRepo.On("IsTripReadyForPayout", mock.Anything, "trip-timeout").Return(true, nil)
	d.supportClient.On("GetSupportUserByID", mock.Anything, "support-1").
		Return(&client.SupportUserInfo{
			UserID: "support-1", Role: "admin", IsActive: true,
		}, nil)
	releasedPayment := &domain.Payment{
		PaymentID: "pay-t1", TripID: "trip-timeout", BookingID: "bk-t1", Amount: 5500, ServiceFeeAmount: 500,
		Status: domain.PaymentStatusReleased,
	}
	d.payoutReadRepo.On("GetReleasedPaymentsForTrip", mock.Anything, "trip-timeout").
		Return([]*domain.Payment{releasedPayment}, nil)
	d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-t1").
		Return(&client.BookingDetails{DriverID: "driver-t1"}, nil)
	d.userClient.On("GetUserByUserID", mock.Anything, "driver-t1").
		Return(&client.UserInfo{WithdrawNumber: "90000001", FirstName: "Kofi", Name: "Mensah"}, nil)
	d.payoutHistoryRepo.On("CreateHistoryEntry", mock.Anything, mock.Anything).Return(nil).Maybe()
}

// =============================================================================
// processPayoutForTrip — protection contre double virement
// =============================================================================

func TestProcessPayoutForTrip_ExistingFedaPayPayout(t *testing.T) {
	ctx := context.Background()

	t.Run("FedaPay sent → réconciliation immédiate, pas de second CreatePayout", func(t *testing.T) {
		d := newCollectTestService()
		setupPayoutForTrip(d)
		d.payoutReadRepo.On("GetByTripID", mock.Anything, "trip-timeout").
			Return(failedPayoutWithRef("trip-timeout", "42"), nil)
		d.fedapay.On("GetPayout", 42).Return(&fedapay.PayoutItem{ID: 42, Status: "sent"}, nil)
		d.payoutWriteRepo.On("ConfirmPayoutCompleted", mock.Anything, "po-timeout-1").Return(nil)
		d.payoutWriteRepo.On("MarkPaymentsAsPaidOut", mock.Anything, "trip-timeout").
			Return([]string{"pay-t1"}, nil)

		_, err := d.svc.TriggerManualPayout(ctx, "trip-timeout", "support-1")

		require.NoError(t, err)
		d.fedapay.AssertNotCalled(t, "CreatePayout", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		d.payoutWriteRepo.AssertCalled(t, "ConfirmPayoutCompleted", mock.Anything, "po-timeout-1")
	})

	t.Run("FedaPay processing → laisse le réconciliateur finir, pas de CreatePayout", func(t *testing.T) {
		d := newCollectTestService()
		setupPayoutForTrip(d)
		d.payoutReadRepo.On("GetByTripID", mock.Anything, "trip-timeout").
			Return(failedPayoutWithRef("trip-timeout", "42"), nil)
		d.fedapay.On("GetPayout", 42).Return(&fedapay.PayoutItem{ID: 42, Status: "processing"}, nil)
		d.payoutWriteRepo.On("UpdatePayoutStatus", mock.Anything, "po-timeout-1",
			domain.PayoutStatusProcessing, "42").Return(nil)

		_, err := d.svc.TriggerManualPayout(ctx, "trip-timeout", "support-1")

		require.NoError(t, err)
		d.fedapay.AssertNotCalled(t, "CreatePayout", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("FedaPay pending → StartPayouts uniquement, pas de CreatePayout", func(t *testing.T) {
		d := newCollectTestService()
		setupPayoutForTrip(d)
		d.payoutReadRepo.On("GetByTripID", mock.Anything, "trip-timeout").
			Return(failedPayoutWithRef("trip-timeout", "42"), nil)
		d.fedapay.On("GetPayout", 42).Return(&fedapay.PayoutItem{ID: 42, Status: "pending"}, nil)
		d.fedapay.On("StartPayouts", []int{42}).Return(nil, nil)
		d.payoutWriteRepo.On("UpdatePayoutStatus", mock.Anything, "po-timeout-1",
			domain.PayoutStatusProcessing, "42").Return(nil)
		d.payoutWriteRepo.On("MarkPaymentsAsPaidOut", mock.Anything, "trip-timeout").
			Return([]string{"pay-t1"}, nil)

		_, err := d.svc.TriggerManualPayout(ctx, "trip-timeout", "support-1")

		require.NoError(t, err)
		d.fedapay.AssertNotCalled(t, "CreatePayout", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		d.fedapay.AssertCalled(t, "StartPayouts", []int{42})
	})

	t.Run("FedaPay failed → CreatePayout normal (nouveau virement légitime)", func(t *testing.T) {
		d := newCollectTestService()
		setupPayoutForTrip(d)
		d.payoutReadRepo.On("GetByTripID", mock.Anything, "trip-timeout").
			Return(failedPayoutWithRef("trip-timeout", "42"), nil)
		d.fedapay.On("GetPayout", 42).Return(&fedapay.PayoutItem{ID: 42, Status: "failed"}, nil)
		// CreatePayout est appelé (nouveau virement sur payout FedaPay définitivement échoué)
		d.fedapay.On("CreatePayout", 5000, "togocel", "90000001", "Kofi", "Mensah").
			Return(&fedapay.PayoutItem{ID: 99}, nil)
		d.payoutWriteRepo.On("CreatePayout", mock.Anything, mock.Anything).Return(nil)
		d.fedapay.On("StartPayouts", []int{99}).Return(nil, nil)
		d.payoutWriteRepo.On("UpdatePayoutStatus", mock.Anything, mock.Anything,
			domain.PayoutStatusProcessing, "99").Return(nil)
		d.payoutWriteRepo.On("MarkPaymentsAsPaidOut", mock.Anything, "trip-timeout").
			Return([]string{"pay-t1"}, nil)

		_, err := d.svc.TriggerManualPayout(ctx, "trip-timeout", "support-1")

		require.NoError(t, err)
		d.fedapay.AssertCalled(t, "CreatePayout", 5000, "togocel", "90000001", "Kofi", "Mensah")
	})

	t.Run("GetPayout en erreur → erreur remontée, aucun virement créé", func(t *testing.T) {
		d := newCollectTestService()
		setupPayoutForTrip(d)
		d.payoutReadRepo.On("GetByTripID", mock.Anything, "trip-timeout").
			Return(failedPayoutWithRef("trip-timeout", "42"), nil)
		d.fedapay.On("GetPayout", 42).Return(nil, paymentErrors.ErrorFedaPayAPIError)

		_, err := d.svc.TriggerManualPayout(ctx, "trip-timeout", "support-1")

		require.ErrorIs(t, err, paymentErrors.ErrorFedaPayAPIError)
		d.fedapay.AssertNotCalled(t, "CreatePayout", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
}

// =============================================================================
// processRefund — protection contre double virement passager
// =============================================================================

func TestProcessRefund_ExistingFedaPayPayout(t *testing.T) {
	ctx := context.Background()

	// refundWithRef simule un refund pending dont une tentative précédente a déjà créé un payout FedaPay.
	refundWithRef := &domain.Refund{
		RefundID: "ref-double-1", PaymentID: "pay-r1", BookingID: "bk-r1",
		AmountToPassenger:        5500,
		Status:                   domain.RefundStatusPending,
		PaymentProviderReference: "42",
		RetryCount:               1,
	}

	setupCommonRefundMocks := func(d *collectDeps) {
		d.refundReadRepo.On("GetPendingRefundsForPayout", mock.Anything, mock.AnythingOfType("int")).
			Return([]*domain.Refund{refundWithRef}, nil)
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-r1").
			Return(&domain.Payment{PassengerPhoneNumber: "91000001"}, nil)
		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-r1").
			Return(&client.BookingDetails{PassengerID: "pass-r1"}, nil)
		d.userClient.On("GetUserByUserID", mock.Anything, "pass-r1").
			Return(&client.UserInfo{FirstName: "Ama", Name: "Kofi"}, nil)
		d.refundWriteRepo.On("MarkRefundProcessing", mock.Anything, "ref-double-1").Return(nil)
	}

	t.Run("FedaPay sent → MarkRefundCompleted immédiat, pas de CreatePayout", func(t *testing.T) {
		d := newCollectTestService()
		setupCommonRefundMocks(d)
		d.fedapay.On("GetPayout", 42).Return(&fedapay.PayoutItem{ID: 42, Status: "sent"}, nil)
		d.refundWriteRepo.On("MarkRefundCompleted", mock.Anything, "ref-double-1", "42").Return(nil)

		require.NoError(t, service.AsImpl(d.svc).ProcessPendingRefunds(ctx))

		d.fedapay.AssertNotCalled(t, "CreatePayout", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		d.refundWriteRepo.AssertCalled(t, "MarkRefundCompleted", mock.Anything, "ref-double-1", "42")
	})

	t.Run("FedaPay processing → rien à faire, pas de CreatePayout", func(t *testing.T) {
		d := newCollectTestService()
		setupCommonRefundMocks(d)
		d.fedapay.On("GetPayout", 42).Return(&fedapay.PayoutItem{ID: 42, Status: "processing"}, nil)

		require.NoError(t, service.AsImpl(d.svc).ProcessPendingRefunds(ctx))

		d.fedapay.AssertNotCalled(t, "CreatePayout", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		d.refundWriteRepo.AssertNotCalled(t, "MarkRefundCompleted", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("FedaPay pending → StartPayouts uniquement, pas de CreatePayout", func(t *testing.T) {
		d := newCollectTestService()
		setupCommonRefundMocks(d)
		d.fedapay.On("GetPayout", 42).Return(&fedapay.PayoutItem{ID: 42, Status: "pending"}, nil)
		d.fedapay.On("StartPayouts", []int{42}).Return(nil, nil)

		require.NoError(t, service.AsImpl(d.svc).ProcessPendingRefunds(ctx))

		d.fedapay.AssertNotCalled(t, "CreatePayout", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		d.fedapay.AssertCalled(t, "StartPayouts", []int{42})
	})
}

// =============================================================================
// processDriverRefundPayout — protection contre double virement chauffeur
// =============================================================================

func TestProcessDriverRefundPayout_ExistingFedaPayPayout(t *testing.T) {
	ctx := context.Background()

	driverPayoutPending := "pending"
	refundDriverWithRef := &domain.Refund{
		RefundID: "ref-drv-1", BookingID: "bk-drv-1",
		AmountToDriver:        5000,
		DriverPayoutStatus:    &driverPayoutPending,
		DriverPayoutProviderRef: "42",
		DriverPayoutRetryCount:  1,
	}

	setupDriverMocks := func(d *collectDeps) {
		d.refundReadRepo.On("GetPendingDriverRefundPayouts", mock.Anything, mock.AnythingOfType("int")).
			Return([]*domain.Refund{refundDriverWithRef}, nil)
		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-drv-1").
			Return(&client.BookingDetails{DriverID: "driver-drv-1"}, nil)
		d.userClient.On("GetUserByUserID", mock.Anything, "driver-drv-1").
			Return(&client.UserInfo{WithdrawNumber: "92000001", FirstName: "Kwame", Name: "Asante"}, nil)
		d.refundWriteRepo.On("MarkDriverPayoutProcessing", mock.Anything, "ref-drv-1").Return(nil)
	}

	t.Run("FedaPay sent → MarkDriverPayoutCompleted, pas de CreatePayout", func(t *testing.T) {
		d := newCollectTestService()
		setupDriverMocks(d)
		d.fedapay.On("GetPayout", 42).Return(&fedapay.PayoutItem{ID: 42, Status: "sent"}, nil)
		d.refundWriteRepo.On("MarkDriverPayoutCompleted", mock.Anything, "ref-drv-1").Return(nil)

		require.NoError(t, service.AsImpl(d.svc).ProcessPendingDriverRefundPayouts(ctx))

		d.fedapay.AssertNotCalled(t, "CreatePayout", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		d.refundWriteRepo.AssertCalled(t, "MarkDriverPayoutCompleted", mock.Anything, "ref-drv-1")
	})

	t.Run("FedaPay pending → StartPayouts uniquement, pas de CreatePayout", func(t *testing.T) {
		d := newCollectTestService()
		setupDriverMocks(d)
		d.fedapay.On("GetPayout", 42).Return(&fedapay.PayoutItem{ID: 42, Status: "pending"}, nil)
		d.fedapay.On("StartPayouts", []int{42}).Return(nil, nil)

		require.NoError(t, service.AsImpl(d.svc).ProcessPendingDriverRefundPayouts(ctx))

		d.fedapay.AssertNotCalled(t, "CreatePayout", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		d.fedapay.AssertCalled(t, "StartPayouts", []int{42})
	})

	t.Run("FedaPay failed → CreatePayout normal (nouveau virement légitime)", func(t *testing.T) {
		d := newCollectTestService()
		setupDriverMocks(d)
		d.fedapay.On("GetPayout", 42).Return(&fedapay.PayoutItem{ID: 42, Status: "failed"}, nil)
		d.fedapay.On("CreatePayout", 5000, "togocel", "92000001", "Kwame", "Asante").
			Return(&fedapay.PayoutItem{ID: 99}, nil)
		d.refundWriteRepo.On("SetDriverPayoutProviderReference", mock.Anything, "ref-drv-1", "99").Return(nil)
		d.fedapay.On("StartPayouts", []int{99}).Return(nil, nil)

		require.NoError(t, service.AsImpl(d.svc).ProcessPendingDriverRefundPayouts(ctx))

		d.fedapay.AssertCalled(t, "CreatePayout", 5000, "togocel", "92000001", "Kwame", "Asante")
	})
}
