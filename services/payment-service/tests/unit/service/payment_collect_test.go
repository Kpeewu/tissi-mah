package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/pkg/notification"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/config"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/fedapay"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/middleware"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/service"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/payment-service/internal/service/interfaces"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
	"github.com/Kpeewu/tissi-mah/services/payment-service/tests/mocks"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// --- Helpers ---

const (
	testFirebaseUID = "fb-passenger"
	testPassengerID = "user-passenger"
	testCallbackURL = "https://api.test/api/v1/payment/callback"
)

type collectDeps struct {
	*testDeps
	fedapay *mocks.MockFedaPayClient
}

func newCollectTestService() *collectDeps {
	return buildCollectTestService(nil)
}

// buildCollectTestService construit le service ; notifRedis non nil permet de capturer les notifications.
func buildCollectTestService(notifRedis *redis.Client) *collectDeps {
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

	cfg := &config.Config{
		FedaPay:    config.FedaPayConfig{CallbackURL: testCallbackURL},
		Expiration: config.ExpirationConfig{IntervalSeconds: 60, PaymentTimeoutMinutes: 10},
		Refund: config.RefundConfig{
			CancellationFullRefundHours:    24,
			CancellationGracePeriodMinutes: 30,
		},
	}

	d.svc = service.NewPaymentService(
		d.paymentReadRepo, d.paymentWriteRepo,
		d.refundReadRepo, d.refundWriteRepo,
		d.payoutReadRepo, d.payoutWriteRepo,
		d.payoutHistoryRepo, d.paymentHistoryRepo, d.refundHistoryRepo,
		d.bookingClient, d.userClient, d.supportClient,
		d.fedapay,
		nil, // cache
		notifRedis,
		cfg,
		zap.NewNop(),
	)
	return d
}

func passengerCtx() context.Context {
	return context.WithValue(context.Background(), middleware.FirebaseIDKey, testFirebaseUID)
}

func payableBooking(createdAt time.Time) *client.BookingDetails {
	return &client.BookingDetails{
		BookingID:        "bk-1",
		BookingReference: "BK-REF-1",
		TripID:           "trip-1",
		PassengerID:      testPassengerID,
		DriverID:         "driver-1",
		Status:           "paymentPending",
		PaymentMethod:    "mobileMoney",
		Subtotal:         5000,
		ServiceFee:       500,
		TotalAmount:      5500,
		CreatedAt:        createdAt,
	}
}

func pendingPayment() *domain.Payment {
	return &domain.Payment{
		PaymentID:             "pay-1",
		BookingID:             "bk-1",
		TripID:                "trip-1",
		Amount:                5500,
		PaymentMethod:         domain.PaymentMobileMoney,
		Status:                domain.PaymentStatusPending,
		ExternalTransactionID: "101",
		PaymentReference:      "PAY-20261011-ABCDEF",
		PassengerPhoneNumber:  "90112233",
		PaymentURL:            "https://checkout.fedapay.com/pay/tok",
		ProviderStatus:        "pending",
		ExpiresAt:             time.Now().UTC().Add(5 * time.Minute),
	}
}

func withStatus(p *domain.Payment, status domain.PaymentStatus) *domain.Payment {
	cp := *p
	cp.Status = status
	return &cp
}

// expectOwner configure la résolution Firebase UID → UserID du passager.
func (d *collectDeps) expectOwner() {
	d.userClient.On("GetUserIDByFirebaseID", mock.Anything, testFirebaseUID).Return(testPassengerID, nil)
}

// =============================================================================
// CreatePayment
// =============================================================================

func TestCreatePayment_Redirect(t *testing.T) {
	t.Run("succes - crée la collecte et retourne le lien de paiement", func(t *testing.T) {
		d := newCollectTestService()
		createdAt := time.Now().UTC().Add(-2 * time.Minute)
		booking := payableBooking(createdAt)

		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-1").Return(booking, nil)
		d.expectOwner()
		d.paymentReadRepo.On("GetPendingByBookingID", mock.Anything, "bk-1").Return(nil, paymentErrors.ErrorPaymentNotFound)
		d.paymentReadRepo.On("HasActivePayment", mock.Anything, "bk-1").Return(false, nil)
		d.userClient.On("GetUserByUserID", mock.Anything, testPassengerID).Return(&client.UserInfo{FirstName: "Ama", Name: "Koffi"}, nil)
		d.fedapay.On("CreateTransaction", mock.MatchedBy(func(p fedapay.CreateTransactionParams) bool {
			return p.Amount == 5500 && // montant de la réservation, pas celui du client
				p.CallbackURL == testCallbackURL &&
				strings.HasPrefix(p.MerchantReference, "PAY-") &&
				p.Customer != nil && p.Customer.PhoneNumber == nil && // numéro choisi sur la page FedaPay
				p.CustomMetadata["booking_id"] == "bk-1"
		})).Return(&fedapay.Transaction{ID: 101, Status: "pending"}, nil)
		d.fedapay.On("GetTransactionToken", 101).Return(&fedapay.TokenResponse{Token: "tok", URL: "https://checkout.fedapay.com/pay/tok"}, nil)
		d.paymentWriteRepo.On("CreatePayment", mock.Anything, mock.MatchedBy(func(p *domain.Payment) bool {
			return p.Amount == 5500 &&
				p.ExternalTransactionID == "101" &&
				p.PassengerPhoneNumber == "90112233" &&
				p.PaymentURL == "https://checkout.fedapay.com/pay/tok" &&
				p.ExpiresAt.Equal(createdAt.Add(10*time.Minute))
		})).Return(nil)

		result, err := d.svc.CreatePayment(passengerCtx(), &serviceInterfaces.CreatePaymentInput{
			BookingID:            "bk-1",
			TripID:               "trip-1",
			PaymentMethod:        "mobileMoney",
			Amount:               1, // ignoré
			PassengerPhoneNumber: "+228 90 11 22 33",
		})

		require.NoError(t, err)
		assert.Equal(t, "https://checkout.fedapay.com/pay/tok", result.PaymentURL)
		assert.Equal(t, "pending", result.Status)
		assert.Equal(t, 5500, result.Amount)
		assert.False(t, result.Reused)
		assert.Equal(t, createdAt.Add(10*time.Minute).Format(time.RFC3339), result.ExpiresAt)
		d.fedapay.AssertExpectations(t)
		d.paymentWriteRepo.AssertExpectations(t)
		history := d.paymentHistory()
		require.Len(t, history, 1)
		assert.Equal(t, "pending", history[0].Status)
		assert.Equal(t, "pending", history[0].ProviderStatus)
		assert.Equal(t, domain.HistoryInitiatedByPassenger, history[0].InitiatedBy)
		assert.Equal(t, domain.HistorySourceCreatePayment, history[0].Source)
		assert.Contains(t, history[0].Notes, "Transaction FedaPay 101")
	})

	t.Run("succes - réutilise le paiement pending existant", func(t *testing.T) {
		d := newCollectTestService()
		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-1").Return(payableBooking(time.Now().UTC()), nil)
		d.expectOwner()
		d.paymentReadRepo.On("GetPendingByBookingID", mock.Anything, "bk-1").Return(pendingPayment(), nil)

		result, err := d.svc.CreatePayment(passengerCtx(), &serviceInterfaces.CreatePaymentInput{
			BookingID: "bk-1", PaymentMethod: "mobileMoney", PassengerPhoneNumber: "90112233",
		})

		require.NoError(t, err)
		assert.True(t, result.Reused)
		assert.Equal(t, "pay-1", result.PaymentID)
		assert.Equal(t, "https://checkout.fedapay.com/pay/tok", result.PaymentURL)
		d.fedapay.AssertNotCalled(t, "CreateTransaction", mock.Anything)
		assert.Empty(t, d.paymentHistory(), "réutilisation sans régénération : rien à historiser")
	})

	t.Run("erreur - l'utilisateur n'est pas le passager", func(t *testing.T) {
		d := newCollectTestService()
		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-1").Return(payableBooking(time.Now().UTC()), nil)
		d.userClient.On("GetUserIDByFirebaseID", mock.Anything, testFirebaseUID).Return("someone-else", nil)

		_, err := d.svc.CreatePayment(passengerCtx(), &serviceInterfaces.CreatePaymentInput{
			BookingID: "bk-1", PaymentMethod: "mobileMoney", PassengerPhoneNumber: "90112233",
		})

		assert.ErrorIs(t, err, paymentErrors.ErrorPaymentNotOwned)
		d.fedapay.AssertNotCalled(t, "CreateTransaction", mock.Anything)
	})

	t.Run("erreur - fenêtre de paiement de 10 min dépassée", func(t *testing.T) {
		d := newCollectTestService()
		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-1").Return(payableBooking(time.Now().UTC().Add(-11*time.Minute)), nil)
		d.expectOwner()

		_, err := d.svc.CreatePayment(passengerCtx(), &serviceInterfaces.CreatePaymentInput{
			BookingID: "bk-1", PaymentMethod: "mobileMoney", PassengerPhoneNumber: "90112233",
		})

		assert.ErrorIs(t, err, paymentErrors.ErrorBookingPaymentExpired)
	})

	t.Run("erreur - réservation qui n'est plus en paymentPending", func(t *testing.T) {
		d := newCollectTestService()
		booking := payableBooking(time.Now().UTC())
		booking.Status = "paymentFailed"
		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-1").Return(booking, nil)
		d.expectOwner()

		_, err := d.svc.CreatePayment(passengerCtx(), &serviceInterfaces.CreatePaymentInput{
			BookingID: "bk-1", PaymentMethod: "mobileMoney", PassengerPhoneNumber: "90112233",
		})

		assert.ErrorIs(t, err, paymentErrors.ErrorBookingNotPayable)
	})

	t.Run("erreur - numéro de remboursement invalide", func(t *testing.T) {
		d := newCollectTestService()

		_, err := d.svc.CreatePayment(passengerCtx(), &serviceInterfaces.CreatePaymentInput{
			BookingID: "bk-1", PaymentMethod: "mobileMoney", PassengerPhoneNumber: "12ab",
		})

		assert.ErrorIs(t, err, paymentErrors.ErrorInvalidInput)
	})

	t.Run("erreur - échec du token : la transaction orpheline est supprimée", func(t *testing.T) {
		d := newCollectTestService()
		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-1").Return(payableBooking(time.Now().UTC()), nil)
		d.expectOwner()
		d.paymentReadRepo.On("GetPendingByBookingID", mock.Anything, "bk-1").Return(nil, paymentErrors.ErrorPaymentNotFound)
		d.paymentReadRepo.On("HasActivePayment", mock.Anything, "bk-1").Return(false, nil)
		d.userClient.On("GetUserByUserID", mock.Anything, testPassengerID).Return(&client.UserInfo{}, nil)
		d.fedapay.On("CreateTransaction", mock.Anything).Return(&fedapay.Transaction{ID: 102}, nil)
		d.fedapay.On("GetTransactionToken", 102).Return(nil, errors.New("boom"))
		d.fedapay.On("CancelTransaction", 102).Return(nil)

		_, err := d.svc.CreatePayment(passengerCtx(), &serviceInterfaces.CreatePaymentInput{
			BookingID: "bk-1", PaymentMethod: "mobileMoney", PassengerPhoneNumber: "90112233",
		})

		assert.ErrorIs(t, err, paymentErrors.ErrorFedaPayAPIError)
		d.fedapay.AssertCalled(t, "CancelTransaction", 102)
		d.paymentWriteRepo.AssertNotCalled(t, "CreatePayment", mock.Anything, mock.Anything)
	})

	t.Run("erreur - échec DB : la transaction orpheline est supprimée", func(t *testing.T) {
		d := newCollectTestService()
		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-1").Return(payableBooking(time.Now().UTC()), nil)
		d.expectOwner()
		d.paymentReadRepo.On("GetPendingByBookingID", mock.Anything, "bk-1").Return(nil, paymentErrors.ErrorPaymentNotFound)
		d.paymentReadRepo.On("HasActivePayment", mock.Anything, "bk-1").Return(false, nil)
		d.userClient.On("GetUserByUserID", mock.Anything, testPassengerID).Return(&client.UserInfo{}, nil)
		d.fedapay.On("CreateTransaction", mock.Anything).Return(&fedapay.Transaction{ID: 103}, nil)
		d.fedapay.On("GetTransactionToken", 103).Return(&fedapay.TokenResponse{URL: "https://pay"}, nil)
		d.paymentWriteRepo.On("CreatePayment", mock.Anything, mock.Anything).Return(paymentErrors.ErrorInternalServer)
		d.fedapay.On("CancelTransaction", 103).Return(nil)

		_, err := d.svc.CreatePayment(passengerCtx(), &serviceInterfaces.CreatePaymentInput{
			BookingID: "bk-1", PaymentMethod: "mobileMoney", PassengerPhoneNumber: "90112233",
		})

		assert.ErrorIs(t, err, paymentErrors.ErrorInternalServer)
		d.fedapay.AssertCalled(t, "CancelTransaction", 103)
		assert.Empty(t, d.paymentHistory())
	})
}

// =============================================================================
// VerifyPayment — application du statut FedaPay
// =============================================================================

func TestVerifyPayment(t *testing.T) {
	t.Run("approved → held et réservation confirmée", func(t *testing.T) {
		d := newCollectTestService()
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(pendingPayment(), nil).Once()
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(withStatus(pendingPayment(), domain.PaymentStatusHeld), nil)
		booking := payableBooking(time.Now().UTC())
		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-1").Return(booking, nil)
		d.expectOwner()
		d.fedapay.On("GetTransaction", 101).Return(&fedapay.Transaction{ID: 101, Status: "approved"}, nil)
		d.paymentWriteRepo.On("UpdatePaymentStatus", mock.Anything, "pay-1", domain.PaymentStatusHeld, "101").Return(nil)
		d.paymentWriteRepo.On("UpdateProviderStatus", mock.Anything, "pay-1", "approved").Return(nil)
		d.bookingClient.On("ConfirmPayment", mock.Anything, "bk-1", "101").Return("approved", nil)
		d.paymentWriteRepo.On("MarkBookingSynced", mock.Anything, "pay-1").Return(nil)

		result, err := d.svc.VerifyPayment(passengerCtx(), "pay-1")

		require.NoError(t, err)
		assert.Equal(t, "held", result.Status)
		assert.True(t, result.IsFinal)
		assert.Empty(t, result.PaymentURL)
		d.bookingClient.AssertCalled(t, "ConfirmPayment", mock.Anything, "bk-1", "101")
		d.bookingClient.AssertNotCalled(t, "FailPayment", mock.Anything, mock.Anything, mock.Anything)
		assert.Equal(t, []string{"held", domain.PaymentEventBookingSynced}, d.paymentHistoryStatuses("pay-1"))
		for _, e := range d.paymentHistory() {
			assert.Equal(t, domain.HistoryInitiatedByPassenger, e.InitiatedBy)
			assert.Equal(t, domain.HistorySourceVerifyPayment, e.Source)
		}
		assert.Equal(t, "approved", d.paymentHistory()[0].ProviderStatus)
	})

	t.Run("declined → reste pending (nouvelle tentative possible), places non libérées", func(t *testing.T) {
		d := newCollectTestService()
		declined := pendingPayment()
		declined.ProviderStatus = "declined"
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(pendingPayment(), nil).Once()
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(declined, nil)
		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-1").Return(payableBooking(time.Now().UTC()), nil)
		d.expectOwner()
		d.fedapay.On("GetTransaction", 101).Return(&fedapay.Transaction{ID: 101, Status: "declined"}, nil)
		d.paymentWriteRepo.On("UpdateProviderStatus", mock.Anything, "pay-1", "declined").Return(nil)

		result, err := d.svc.VerifyPayment(passengerCtx(), "pay-1")

		require.NoError(t, err)
		assert.Equal(t, "pending", result.Status)
		assert.Equal(t, "declined", result.ProviderStatus)
		assert.False(t, result.IsFinal)
		assert.NotEmpty(t, result.PaymentURL)
		d.paymentWriteRepo.AssertNotCalled(t, "MarkPaymentFailed", mock.Anything, mock.Anything, mock.Anything)
		d.bookingClient.AssertNotCalled(t, "FailPayment", mock.Anything, mock.Anything, mock.Anything)
		history := d.paymentHistory()
		require.Len(t, history, 1)
		assert.Equal(t, domain.PaymentEventAttemptFailed, history[0].Status)
		assert.Equal(t, "declined", history[0].ProviderStatus)
	})

	t.Run("expired → failed et places libérées", func(t *testing.T) {
		d := newCollectTestService()
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(pendingPayment(), nil).Once()
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(withStatus(pendingPayment(), domain.PaymentStatusFailed), nil)
		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-1").Return(payableBooking(time.Now().UTC()), nil)
		d.expectOwner()
		d.fedapay.On("GetTransaction", 101).Return(&fedapay.Transaction{ID: 101, Status: "expired"}, nil)
		d.paymentWriteRepo.On("MarkPaymentFailed", mock.Anything, "pay-1", mock.Anything).Return(nil)
		d.paymentWriteRepo.On("UpdateProviderStatus", mock.Anything, "pay-1", "expired").Return(nil)
		d.bookingClient.On("FailPayment", mock.Anything, "bk-1", mock.Anything).Return(nil)

		result, err := d.svc.VerifyPayment(passengerCtx(), "pay-1")

		require.NoError(t, err)
		assert.Equal(t, "failed", result.Status)
		assert.True(t, result.SeatsReleased)
		d.bookingClient.AssertCalled(t, "FailPayment", mock.Anything, "bk-1", mock.Anything)
		history := d.paymentHistory()
		require.Len(t, history, 1)
		assert.Equal(t, "failed", history[0].Status)
		assert.Equal(t, "expired", history[0].ProviderStatus)
		assert.Equal(t, "FedaPay: transaction expired", history[0].Notes)
	})

	t.Run("approbation tardive sur paiement failed sans places → remboursement intégral", func(t *testing.T) {
		d := newCollectTestService()
		failed := withStatus(pendingPayment(), domain.PaymentStatusFailed)
		held := withStatus(pendingPayment(), domain.PaymentStatusHeld)
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(failed, nil).Once()
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(held, nil).Once()
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(withStatus(pendingPayment(), domain.PaymentStatusRefunded), nil)
		booking := payableBooking(time.Now().UTC())
		booking.Status = "paymentFailed"
		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-1").Return(booking, nil)
		d.expectOwner()
		d.fedapay.On("GetTransaction", 101).Return(&fedapay.Transaction{ID: 101, Status: "approved"}, nil)
		d.paymentWriteRepo.On("MarkFailedPaymentHeld", mock.Anything, "pay-1").Return(nil)
		d.bookingClient.On("ConfirmPayment", mock.Anything, "bk-1", "101").Return("", paymentErrors.ErrorNoSeatsAvailable)
		d.refundWriteRepo.On("CreateRefund", mock.Anything, mock.MatchedBy(func(r *domain.Refund) bool {
			return r.RefundReason == domain.RefundReasonSeatsUnavailable &&
				r.AmountToPassenger == 5500 && // sous-total + frais de service
				r.PayoutDestination == "90112233"
		})).Return(nil)
		d.paymentWriteRepo.On("UpdatePaymentStatus", mock.Anything, "pay-1", domain.PaymentStatusRefunded, "").Return(nil)

		result, err := d.svc.VerifyPayment(passengerCtx(), "pay-1")

		require.NoError(t, err)
		assert.Equal(t, "refunded", result.Status)
		d.refundWriteRepo.AssertExpectations(t)
		assert.Equal(t, []string{"held", "refunded"}, d.paymentHistoryStatuses("pay-1"))
		assert.Contains(t, d.paymentHistory()[0].Notes, "tardive")
		assert.Equal(t, []string{"pending"}, d.refundHistoryStatuses())
	})

	t.Run("approbation tardive, booking-service indisponible → held non rattaché", func(t *testing.T) {
		rdb := newNotifRedis(t)
		d := buildCollectTestService(rdb)
		failed := withStatus(pendingPayment(), domain.PaymentStatusFailed)
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(failed, nil).Once()
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").
			Return(withStatus(pendingPayment(), domain.PaymentStatusHeld), nil)
		booking := payableBooking(time.Now().UTC())
		booking.Status = "paymentFailed"
		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-1").Return(booking, nil)
		d.expectOwner()
		d.fedapay.On("GetTransaction", 101).Return(&fedapay.Transaction{ID: 101, Status: "approved"}, nil)
		d.paymentWriteRepo.On("MarkFailedPaymentHeld", mock.Anything, "pay-1").Return(nil)
		d.bookingClient.On("ConfirmPayment", mock.Anything, "bk-1", "101").
			Return("", errors.New("booking-service unavailable"))

		result, err := d.svc.VerifyPayment(passengerCtx(), "pay-1")

		require.NoError(t, err)
		assert.Equal(t, "held", result.Status)
		d.paymentWriteRepo.AssertNotCalled(t, "MarkBookingSynced", mock.Anything, mock.Anything)
		d.refundWriteRepo.AssertNotCalled(t, "CreateRefund", mock.Anything, mock.Anything)
		// L'argent est encaissé : PAYMENT_COMPLETED est publié même sans rattachement
		assert.Equal(t, []string{"held", domain.PaymentEventBookingSyncFailed}, d.paymentHistoryStatuses("pay-1"))
		assert.Contains(t, d.paymentHistory()[1].Notes, "Tentative 1 : booking-service injoignable")
		assert.NotNil(t, findEvent(publishedEvents(t, rdb), notification.PaymentCompleted))
	})

	t.Run("réservation annulée → remboursement intégral, pas de PAYMENT_COMPLETED", func(t *testing.T) {
		rdb := newNotifRedis(t)
		d := buildCollectTestService(rdb)
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(pendingPayment(), nil).Once()
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").
			Return(withStatus(pendingPayment(), domain.PaymentStatusHeld), nil).Once()
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").
			Return(withStatus(pendingPayment(), domain.PaymentStatusRefunded), nil)
		booking := payableBooking(time.Now().UTC())
		booking.Status = "cancelled"
		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-1").Return(booking, nil)
		d.expectOwner()
		d.fedapay.On("GetTransaction", 101).Return(&fedapay.Transaction{ID: 101, Status: "approved"}, nil)
		d.paymentWriteRepo.On("UpdatePaymentStatus", mock.Anything, "pay-1", domain.PaymentStatusHeld, "101").Return(nil)
		d.paymentWriteRepo.On("UpdateProviderStatus", mock.Anything, "pay-1", "approved").Return(nil)
		d.bookingClient.On("ConfirmPayment", mock.Anything, "bk-1", "101").
			Return("", paymentErrors.ErrorBookingNotConfirmable)
		d.refundWriteRepo.On("CreateRefund", mock.Anything, mock.MatchedBy(func(r *domain.Refund) bool {
			return r.RefundReason == domain.RefundReasonBookingUnavailable &&
				r.AmountToPassenger == 5500 && // sous-total + frais de service
				r.PayoutDestination == "90112233"
		})).Return(nil)
		d.paymentWriteRepo.On("UpdatePaymentStatus", mock.Anything, "pay-1", domain.PaymentStatusRefunded, "").Return(nil)

		result, err := d.svc.VerifyPayment(passengerCtx(), "pay-1")

		require.NoError(t, err)
		assert.Equal(t, "refunded", result.Status)
		d.refundWriteRepo.AssertExpectations(t)

		events := publishedEvents(t, rdb)
		assert.Equal(t, []string{"held", "refunded"}, d.paymentHistoryStatuses("pay-1"))
		refundHistory := d.refundHistory()
		require.Len(t, refundHistory, 1)
		assert.Equal(t, "pending", refundHistory[0].Status)
		assert.Contains(t, refundHistory[0].Notes, "bookingUnavailable")
		refundEvent := findEvent(events, notification.RefundProcessed)
		require.NotNil(t, refundEvent, "REFUND_PROCESSED doit être publié")
		assert.Equal(t, testPassengerID, refundEvent.UserID)
		assert.Equal(t, "bookingUnavailable", refundEvent.Payload["reason"])
		assert.Equal(t, "5500", refundEvent.Payload["amount_to_passenger"])
		assert.Nil(t, findEvent(events, notification.PaymentCompleted), "PAYMENT_COMPLETED ne doit pas être publié")
	})

	t.Run("réservation redevenue payable pendant la confirmation → reste held, pas de remboursement", func(t *testing.T) {
		rdb := newNotifRedis(t)
		d := buildCollectTestService(rdb)
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(pendingPayment(), nil).Once()
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").
			Return(withStatus(pendingPayment(), domain.PaymentStatusHeld), nil)
		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-1").
			Return(payableBooking(time.Now().UTC()), nil)
		d.expectOwner()
		d.fedapay.On("GetTransaction", 101).Return(&fedapay.Transaction{ID: 101, Status: "approved"}, nil)
		d.paymentWriteRepo.On("UpdatePaymentStatus", mock.Anything, "pay-1", domain.PaymentStatusHeld, "101").Return(nil)
		d.paymentWriteRepo.On("UpdateProviderStatus", mock.Anything, "pay-1", "approved").Return(nil)
		d.bookingClient.On("ConfirmPayment", mock.Anything, "bk-1", "101").
			Return("", paymentErrors.ErrorBookingNotConfirmable)

		result, err := d.svc.VerifyPayment(passengerCtx(), "pay-1")

		require.NoError(t, err)
		assert.Equal(t, "held", result.Status)
		d.refundWriteRepo.AssertNotCalled(t, "CreateRefund", mock.Anything, mock.Anything)
		assert.Empty(t, publishedEvents(t, rdb))
		assert.Equal(t, []string{"held", domain.PaymentEventBookingSyncFailed}, d.paymentHistoryStatuses("pay-1"))
	})

	t.Run("erreur - paiement d'un autre utilisateur", func(t *testing.T) {
		d := newCollectTestService()
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(pendingPayment(), nil)
		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-1").Return(payableBooking(time.Now().UTC()), nil)
		d.userClient.On("GetUserIDByFirebaseID", mock.Anything, testFirebaseUID).Return("intruder", nil)

		_, err := d.svc.VerifyPayment(passengerCtx(), "pay-1")

		assert.ErrorIs(t, err, paymentErrors.ErrorPaymentNotOwned)
		d.fedapay.AssertNotCalled(t, "GetTransaction", mock.Anything)
	})
}

// =============================================================================
// CancelPayment / CancelPaymentForBooking
// =============================================================================

func TestCancelPayment(t *testing.T) {
	t.Run("supprime la transaction FedaPay et libère les places, sans PAYMENT_FAILED", func(t *testing.T) {
		rdb := newNotifRedis(t)
		d := buildCollectTestService(rdb)
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(pendingPayment(), nil).Once()
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(withStatus(pendingPayment(), domain.PaymentStatusFailed), nil)
		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-1").Return(payableBooking(time.Now().UTC()), nil)
		d.expectOwner()
		d.fedapay.On("GetTransaction", 101).Return(&fedapay.Transaction{ID: 101, Status: "pending"}, nil)
		d.fedapay.On("CancelTransaction", 101).Return(nil)
		d.paymentWriteRepo.On("MarkPaymentFailed", mock.Anything, "pay-1", mock.Anything).Return(nil)
		d.paymentWriteRepo.On("UpdateProviderStatus", mock.Anything, "pay-1", "deleted").Return(nil)
		d.bookingClient.On("FailPayment", mock.Anything, "bk-1", mock.Anything).Return(nil)

		result, err := d.svc.CancelPayment(passengerCtx(), "pay-1")

		require.NoError(t, err)
		assert.Equal(t, "failed", result.Status)
		d.fedapay.AssertCalled(t, "CancelTransaction", 101)
		d.bookingClient.AssertCalled(t, "FailPayment", mock.Anything, "bk-1", mock.Anything)
		// Abandon volontaire : pas une erreur de paiement
		assert.Nil(t, findEvent(publishedEvents(t, rdb), notification.PaymentFailed))
		history := d.paymentHistory()
		require.Len(t, history, 1)
		assert.Equal(t, "failed", history[0].Status)
		assert.Equal(t, "deleted", history[0].ProviderStatus)
		assert.Equal(t, domain.HistoryInitiatedByPassenger, history[0].InitiatedBy)
		assert.Equal(t, domain.HistorySourceCancelPayment, history[0].Source)
	})

	t.Run("course avec une approbation : applique l'approbation sans annuler", func(t *testing.T) {
		d := newCollectTestService()
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(pendingPayment(), nil).Once()
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(withStatus(pendingPayment(), domain.PaymentStatusHeld), nil)
		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-1").Return(payableBooking(time.Now().UTC()), nil)
		d.expectOwner()
		d.fedapay.On("GetTransaction", 101).Return(&fedapay.Transaction{ID: 101, Status: "approved"}, nil)
		d.paymentWriteRepo.On("UpdatePaymentStatus", mock.Anything, "pay-1", domain.PaymentStatusHeld, "101").Return(nil)
		d.paymentWriteRepo.On("UpdateProviderStatus", mock.Anything, "pay-1", "approved").Return(nil)
		d.bookingClient.On("ConfirmPayment", mock.Anything, "bk-1", "101").Return("pendingApproval", nil)
		d.paymentWriteRepo.On("MarkBookingSynced", mock.Anything, "pay-1").Return(nil)

		result, err := d.svc.CancelPayment(passengerCtx(), "pay-1")

		require.NoError(t, err)
		assert.Equal(t, "held", result.Status)
		d.fedapay.AssertNotCalled(t, "CancelTransaction", mock.Anything)
		d.bookingClient.AssertNotCalled(t, "FailPayment", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("échec du DELETE et transaction toujours pending : ne marque pas failed", func(t *testing.T) {
		d := newCollectTestService()
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(pendingPayment(), nil)
		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-1").Return(payableBooking(time.Now().UTC()), nil)
		d.expectOwner()
		d.fedapay.On("GetTransaction", 101).Return(&fedapay.Transaction{ID: 101, Status: "pending"}, nil)
		d.fedapay.On("CancelTransaction", 101).Return(errors.New("fedapay down"))

		_, err := d.svc.CancelPayment(passengerCtx(), "pay-1")

		assert.ErrorIs(t, err, paymentErrors.ErrorFedaPayAPIError)
		d.paymentWriteRepo.AssertNotCalled(t, "MarkPaymentFailed", mock.Anything, mock.Anything, mock.Anything)
		d.bookingClient.AssertNotCalled(t, "FailPayment", mock.Anything, mock.Anything, mock.Anything)
		assert.Empty(t, d.paymentHistory())
	})
}

func TestCancelPaymentForBooking(t *testing.T) {
	t.Run("supprime la collecte sans rappeler FailPayment ni PAYMENT_FAILED", func(t *testing.T) {
		rdb := newNotifRedis(t)
		d := buildCollectTestService(rdb)
		d.paymentReadRepo.On("GetPendingByBookingID", mock.Anything, "bk-1").Return(pendingPayment(), nil)
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(withStatus(pendingPayment(), domain.PaymentStatusFailed), nil)
		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-1").Return(payableBooking(time.Now().UTC()), nil)
		d.fedapay.On("GetTransaction", 101).Return(&fedapay.Transaction{ID: 101, Status: "pending"}, nil)
		d.fedapay.On("CancelTransaction", 101).Return(nil)
		d.paymentWriteRepo.On("MarkPaymentFailed", mock.Anything, "pay-1", "Réservation annulée").Return(nil)
		d.paymentWriteRepo.On("UpdateProviderStatus", mock.Anything, "pay-1", "deleted").Return(nil)

		result, err := d.svc.CancelPaymentForBooking(context.Background(), "bk-1", "Réservation annulée")

		require.NoError(t, err)
		assert.Equal(t, "failed", result.Status)
		history := d.paymentHistory()
		require.Len(t, history, 1)
		assert.Equal(t, "failed", history[0].Status)
		assert.Equal(t, "Réservation annulée", history[0].Notes)
		assert.Equal(t, domain.HistoryInitiatedByBookingService, history[0].InitiatedBy)
		assert.Equal(t, domain.HistorySourceBookingService, history[0].Source)
		d.bookingClient.AssertNotCalled(t, "FailPayment", mock.Anything, mock.Anything, mock.Anything)
		assert.Nil(t, findEvent(publishedEvents(t, rdb), notification.PaymentFailed))
	})

	t.Run("aucune collecte en cours : rien à annuler", func(t *testing.T) {
		d := newCollectTestService()
		d.paymentReadRepo.On("GetPendingByBookingID", mock.Anything, "bk-1").Return(nil, paymentErrors.ErrorPaymentNotFound)

		result, err := d.svc.CancelPaymentForBooking(context.Background(), "bk-1", "")

		require.NoError(t, err)
		assert.Empty(t, result.Status)
		d.fedapay.AssertNotCalled(t, "CancelTransaction", mock.Anything)
	})
}

// =============================================================================
// HandlePaymentCallback
// =============================================================================

func TestHandlePaymentCallback(t *testing.T) {
	t.Run("revérifie le statut auprès de FedaPay", func(t *testing.T) {
		d := newCollectTestService()
		d.paymentReadRepo.On("GetByExternalTransactionID", mock.Anything, "101").Return(pendingPayment(), nil)
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(withStatus(pendingPayment(), domain.PaymentStatusHeld), nil)
		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-1").Return(payableBooking(time.Now().UTC()), nil)
		d.fedapay.On("GetTransaction", 101).Return(&fedapay.Transaction{ID: 101, Status: "approved"}, nil)
		d.paymentWriteRepo.On("UpdatePaymentStatus", mock.Anything, "pay-1", domain.PaymentStatusHeld, "101").Return(nil)
		d.paymentWriteRepo.On("UpdateProviderStatus", mock.Anything, "pay-1", "approved").Return(nil)
		d.bookingClient.On("ConfirmPayment", mock.Anything, "bk-1", "101").Return("pendingApproval", nil)
		d.paymentWriteRepo.On("MarkBookingSynced", mock.Anything, "pay-1").Return(nil)

		result, err := d.svc.HandlePaymentCallback(context.Background(), "101")

		require.NoError(t, err)
		assert.Equal(t, "held", result.Status)
		assert.Equal(t, "pay-1", result.PaymentID)
		assert.Equal(t, []string{"held", domain.PaymentEventBookingSynced}, d.paymentHistoryStatuses("pay-1"))
		assert.Equal(t, domain.HistoryInitiatedBySystem, d.paymentHistory()[0].InitiatedBy)
		assert.Equal(t, domain.HistorySourceFedaPayCallback, d.paymentHistory()[0].Source)
	})

	t.Run("transaction expired/deleted : PAYMENT_FAILED seulement à l'expiration", func(t *testing.T) {
		cases := []struct {
			name           string
			providerStatus string
			expiresIn      time.Duration
			wantNotified   bool
		}{
			{"deleted avant l'échéance (annulation) → pas de notification", "deleted", 5 * time.Minute, false},
			{"deleted après l'échéance → notification", "deleted", -time.Minute, true},
			{"expired → notification", "expired", 5 * time.Minute, true},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				rdb := newNotifRedis(t)
				d := buildCollectTestService(rdb)
				payment := pendingPayment()
				payment.ExpiresAt = time.Now().UTC().Add(tc.expiresIn)
				d.paymentReadRepo.On("GetByExternalTransactionID", mock.Anything, "101").Return(payment, nil)
				d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(withStatus(payment, domain.PaymentStatusFailed), nil)
				d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-1").Return(payableBooking(time.Now().UTC()), nil)
				d.fedapay.On("GetTransaction", 101).Return(&fedapay.Transaction{ID: 101, Status: tc.providerStatus}, nil)
				d.paymentWriteRepo.On("MarkPaymentFailed", mock.Anything, "pay-1", mock.Anything).Return(nil)
				d.paymentWriteRepo.On("UpdateProviderStatus", mock.Anything, "pay-1", tc.providerStatus).Return(nil)
				d.bookingClient.On("FailPayment", mock.Anything, "bk-1", mock.Anything).Return(nil)

				result, err := d.svc.HandlePaymentCallback(context.Background(), "101")

				require.NoError(t, err)
				assert.Equal(t, "failed", result.Status)
				d.bookingClient.AssertCalled(t, "FailPayment", mock.Anything, "bk-1", mock.Anything)
				assert.Equal(t, tc.wantNotified, findEvent(publishedEvents(t, rdb), notification.PaymentFailed) != nil)
			})
		}
	})

	t.Run("erreur - identifiant de transaction invalide", func(t *testing.T) {
		d := newCollectTestService()

		_, err := d.svc.HandlePaymentCallback(context.Background(), "abc")

		assert.ErrorIs(t, err, paymentErrors.ErrorInvalidInput)
	})
}

// =============================================================================
// ProcessWebhook — transaction.declined n'est plus final
// =============================================================================

func TestProcessWebhook_DeclinedKeepsPending(t *testing.T) {
	d := newCollectTestService()
	payload := []byte(`{"name":"transaction.declined","entity":{"id":101,"status":"declined"}}`)
	d.fedapay.On("VerifyWebhookSignature", "sig", payload).Return(true)
	d.paymentWriteRepo.On("SaveWebhookEvent", mock.Anything, mock.Anything).Return(nil)
	d.paymentReadRepo.On("GetByExternalTransactionID", mock.Anything, "101").Return(pendingPayment(), nil)
	d.paymentWriteRepo.On("UpdateProviderStatus", mock.Anything, "pay-1", "declined").Return(nil)
	d.paymentWriteRepo.On("MarkWebhookEventProcessed", mock.Anything, "101:transaction.declined").Return(nil)

	err := d.svc.ProcessWebhook(context.Background(), &serviceInterfaces.ProcessWebhookInput{
		Signature:  "sig",
		RawPayload: payload,
	})

	require.NoError(t, err)
	d.paymentWriteRepo.AssertNotCalled(t, "MarkPaymentFailed", mock.Anything, mock.Anything, mock.Anything)
	d.bookingClient.AssertNotCalled(t, "FailPayment", mock.Anything, mock.Anything, mock.Anything)
	d.paymentWriteRepo.AssertCalled(t, "MarkWebhookEventProcessed", mock.Anything, "101:transaction.declined")
	history := d.paymentHistory()
	require.Len(t, history, 1)
	assert.Equal(t, domain.PaymentEventAttemptFailed, history[0].Status)
	assert.Equal(t, domain.HistorySourceFedaPayWebhook, history[0].Source)
}

// =============================================================================
// ProcessWebhook — un événement dont le traitement échoue reste rejouable
// =============================================================================

func TestProcessWebhook_ProcessingFailureKeepsEventReplayable(t *testing.T) {
	payload := []byte(`{"name":"transaction.declined","entity":{"id":101,"status":"declined"}}`)

	t.Run("erreur de traitement - événement non marqué traité", func(t *testing.T) {
		d := newCollectTestService()
		d.fedapay.On("VerifyWebhookSignature", "sig", payload).Return(true)
		d.paymentWriteRepo.On("SaveWebhookEvent", mock.Anything, mock.MatchedBy(func(e *domain.WebhookEvent) bool {
			return e.FedapayEventID == "101:transaction.declined" && e.ProcessedAt == nil
		})).Return(nil)
		d.paymentReadRepo.On("GetByExternalTransactionID", mock.Anything, "101").Return(pendingPayment(), nil)
		d.paymentWriteRepo.On("UpdateProviderStatus", mock.Anything, "pay-1", "declined").
			Return(paymentErrors.ErrorInternalServer)

		err := d.svc.ProcessWebhook(context.Background(), &serviceInterfaces.ProcessWebhookInput{
			Signature:  "sig",
			RawPayload: payload,
		})

		assert.ErrorIs(t, err, paymentErrors.ErrorInternalServer)
		d.paymentWriteRepo.AssertNotCalled(t, "MarkWebhookEventProcessed", mock.Anything, mock.Anything)
		assert.Empty(t, d.paymentHistory(), "transition non appliquée : rien à historiser")
	})

	t.Run("événement déjà traité - ignoré", func(t *testing.T) {
		d := newCollectTestService()
		d.fedapay.On("VerifyWebhookSignature", "sig", payload).Return(true)
		d.paymentWriteRepo.On("SaveWebhookEvent", mock.Anything, mock.Anything).
			Return(paymentErrors.ErrorDuplicateWebhookEvent)

		err := d.svc.ProcessWebhook(context.Background(), &serviceInterfaces.ProcessWebhookInput{
			Signature:  "sig",
			RawPayload: payload,
		})

		require.NoError(t, err)
		d.paymentReadRepo.AssertNotCalled(t, "GetByExternalTransactionID", mock.Anything, mock.Anything)
		d.paymentWriteRepo.AssertNotCalled(t, "MarkWebhookEventProcessed", mock.Anything, mock.Anything)
	})

	t.Run("payout.completed - erreur de mise à jour remontée", func(t *testing.T) {
		payoutPayload := []byte(`{"name":"payout.completed","entity":{"id":555,"status":"sent"}}`)
		d := newCollectTestService()
		d.fedapay.On("VerifyWebhookSignature", "sig", payoutPayload).Return(true)
		d.paymentWriteRepo.On("SaveWebhookEvent", mock.Anything, mock.Anything).Return(nil)
		d.payoutReadRepo.On("GetByProviderReference", mock.Anything, "555").
			Return(&domain.Payout{PayoutID: "po-1"}, nil)
		d.fedapay.On("GetPayout", 555).Return(&fedapay.PayoutItem{ID: 555, Status: "sent"}, nil)
		d.payoutWriteRepo.On("ConfirmPayoutCompleted", mock.Anything, "po-1").
			Return(paymentErrors.ErrorInternalServer)

		err := d.svc.ProcessWebhook(context.Background(), &serviceInterfaces.ProcessWebhookInput{
			Signature:  "sig",
			RawPayload: payoutPayload,
		})

		assert.ErrorIs(t, err, paymentErrors.ErrorInternalServer)
		d.paymentWriteRepo.AssertNotCalled(t, "MarkWebhookEventProcessed", mock.Anything, mock.Anything)
		d.payoutHistoryRepo.AssertNotCalled(t, "CreateHistoryEntry", mock.Anything, mock.Anything)
	})
}

// =============================================================================
// Expiration (10 min) — suppression FedaPay et libération des places
// =============================================================================

func TestExpireDuePayments(t *testing.T) {
	rdb := newNotifRedis(t)
	d := buildCollectTestService(rdb)
	impl := service.AsImpl(d.svc)
	require.NotNil(t, impl)

	unpaid := pendingPayment()
	paidLate := pendingPayment()
	paidLate.PaymentID, paidLate.BookingID, paidLate.ExternalTransactionID = "pay-2", "bk-2", "202"
	deleteFails := pendingPayment()
	deleteFails.PaymentID, deleteFails.BookingID, deleteFails.ExternalTransactionID = "pay-3", "bk-3", "303"

	d.paymentReadRepo.On("GetExpiredPendingPayments", mock.Anything, mock.Anything).
		Return([]*domain.Payment{unpaid, paidLate, deleteFails}, nil)

	// Non payé → DELETE FedaPay + failed + places libérées
	d.fedapay.On("GetTransaction", 101).Return(&fedapay.Transaction{ID: 101, Status: "pending"}, nil)
	d.fedapay.On("CancelTransaction", 101).Return(nil)
	d.paymentWriteRepo.On("MarkPaymentFailed", mock.Anything, "pay-1", mock.Anything).Return(nil)
	d.paymentWriteRepo.On("UpdateProviderStatus", mock.Anything, "pay-1", "deleted").Return(nil)
	d.bookingClient.On("FailPayment", mock.Anything, "bk-1", mock.Anything).Return(nil)
	d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-1").Return(payableBooking(time.Now().UTC()), nil)

	// Payé juste avant l'échéance → held, pas de DELETE
	d.fedapay.On("GetTransaction", 202).Return(&fedapay.Transaction{ID: 202, Status: "approved"}, nil)
	d.paymentWriteRepo.On("UpdatePaymentStatus", mock.Anything, "pay-2", domain.PaymentStatusHeld, "202").Return(nil)
	d.paymentWriteRepo.On("UpdateProviderStatus", mock.Anything, "pay-2", "approved").Return(nil)
	d.bookingClient.On("ConfirmPayment", mock.Anything, "bk-2", "202").Return("pendingApproval", nil)
	d.paymentWriteRepo.On("MarkBookingSynced", mock.Anything, "pay-2").Return(nil)
	d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-2").Return(payableBooking(time.Now().UTC()), nil)

	// DELETE en échec, transaction toujours pending → retentée au prochain tick
	d.fedapay.On("GetTransaction", 303).Return(&fedapay.Transaction{ID: 303, Status: "pending"}, nil)
	d.fedapay.On("CancelTransaction", 303).Return(errors.New("fedapay down"))

	require.NoError(t, impl.ExpireDuePayments(context.Background(), 10))

	d.bookingClient.AssertCalled(t, "FailPayment", mock.Anything, "bk-1", mock.Anything)
	d.fedapay.AssertNotCalled(t, "CancelTransaction", 202)
	d.bookingClient.AssertCalled(t, "ConfirmPayment", mock.Anything, "bk-2", "202")
	d.paymentWriteRepo.AssertNotCalled(t, "MarkPaymentFailed", mock.Anything, "pay-3", mock.Anything)
	d.bookingClient.AssertNotCalled(t, "FailPayment", mock.Anything, "bk-3", mock.Anything)

	failedEvent := findEvent(publishedEvents(t, rdb), notification.PaymentFailed)
	require.NotNil(t, failedEvent, "l'expiration doit notifier le passager")
	assert.Equal(t, "pay-1", failedEvent.ReferenceID)

	assert.Equal(t, []string{"failed"}, d.paymentHistoryStatuses("pay-1"))
	assert.Equal(t, []string{"held", domain.PaymentEventBookingSynced}, d.paymentHistoryStatuses("pay-2"))
	assert.Empty(t, d.paymentHistoryStatuses("pay-3"), "DELETE en échec : aucune transition")
	for _, e := range d.paymentHistory() {
		assert.Equal(t, domain.HistorySourceExpirationWorker, e.Source)
	}
}

// =============================================================================
// Rattachement des paiements held non confirmés auprès de booking-service
// =============================================================================

// unsyncedHeldPayment construit un paiement held non rattaché à sa réservation.
func unsyncedHeldPayment(paymentID, bookingID, externalID string, attempts int) *domain.Payment {
	p := withStatus(pendingPayment(), domain.PaymentStatusHeld)
	p.PaymentID, p.BookingID, p.ExternalTransactionID = paymentID, bookingID, externalID
	p.BookingSyncAttempts = attempts
	return p
}

// expectFullRefund configure le remboursement intégral (5500) d'un paiement held avec le motif donné.
func (d *collectDeps) expectFullRefund(payment *domain.Payment, reason domain.RefundReason) {
	d.paymentReadRepo.On("GetByID", mock.Anything, payment.PaymentID).Return(payment, nil)
	d.refundWriteRepo.On("CreateRefund", mock.Anything, mock.MatchedBy(func(r *domain.Refund) bool {
		return r.PaymentID == payment.PaymentID && r.RefundReason == reason && r.AmountToPassenger == 5500
	})).Return(nil)
	d.paymentWriteRepo.On("UpdatePaymentStatus", mock.Anything, payment.PaymentID, domain.PaymentStatusRefunded, "").
		Return(nil)
}

func TestRetryUnsyncedHeldPayments(t *testing.T) {
	bookingDownErr := errors.New("booking-service unavailable")

	// retry configure le lot retourné par le repository (aucun remboursement déjà créé).
	retry := func(t *testing.T, d *collectDeps, payments ...*domain.Payment) {
		t.Helper()
		d.paymentReadRepo.On("GetUnsyncedHeldPayments", mock.Anything,
			mock.MatchedBy(func(before time.Time) bool { return before.Before(time.Now().UTC().Add(-time.Minute)) }),
			100,
		).Return(payments, nil)
		for _, p := range payments {
			d.refundReadRepo.On("GetByPaymentID", mock.Anything, p.PaymentID).
				Return(nil, paymentErrors.ErrorRefundNotFound).Maybe()
		}
		impl := service.AsImpl(d.svc)
		require.NotNil(t, impl)
		require.NoError(t, impl.RetryUnsyncedHeldPayments(context.Background()))
	}

	t.Run("réservation réactivée → paiement rattaché, compteur inchangé", func(t *testing.T) {
		d := newCollectTestService()
		d.bookingClient.On("ConfirmPayment", mock.Anything, "bk-1", "101").Return("pendingApproval", nil)
		d.paymentWriteRepo.On("MarkBookingSynced", mock.Anything, "pay-1").Return(nil)

		retry(t, d, unsyncedHeldPayment("pay-1", "bk-1", "101", 2))

		d.paymentWriteRepo.AssertCalled(t, "MarkBookingSynced", mock.Anything, "pay-1")
		d.paymentWriteRepo.AssertNotCalled(t, "IncrementBookingSyncAttempts", mock.Anything, mock.Anything)
		assert.Equal(t, []string{domain.PaymentEventBookingSynced}, d.paymentHistoryStatuses("pay-1"))
		assert.Equal(t, domain.HistorySourceBookingSyncWorker, d.paymentHistory()[0].Source)
	})

	t.Run("plus de places → remboursement seatsUnavailable, compteur inchangé", func(t *testing.T) {
		d := newCollectTestService()
		payment := unsyncedHeldPayment("pay-2", "bk-2", "202", 0)
		d.bookingClient.On("ConfirmPayment", mock.Anything, "bk-2", "202").Return("", paymentErrors.ErrorNoSeatsAvailable)
		bk2 := payableBooking(time.Now().UTC())
		bk2.BookingID, bk2.Status = "bk-2", "paymentFailed"
		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-2").Return(bk2, nil)
		d.expectFullRefund(payment, domain.RefundReasonSeatsUnavailable)

		retry(t, d, payment)

		d.refundWriteRepo.AssertNumberOfCalls(t, "CreateRefund", 1)
		d.paymentWriteRepo.AssertNotCalled(t, "IncrementBookingSyncAttempts", mock.Anything, mock.Anything)
		assert.Equal(t, []string{"refunded"}, d.paymentHistoryStatuses("pay-2"))
	})

	t.Run("booking-service indisponible sous le plafond → tentative comptée, pas de remboursement", func(t *testing.T) {
		d := newCollectTestService()
		d.bookingClient.On("ConfirmPayment", mock.Anything, "bk-3", "303").Return("", bookingDownErr)
		d.paymentWriteRepo.On("IncrementBookingSyncAttempts", mock.Anything, "pay-3").Return(1, nil)

		retry(t, d, unsyncedHeldPayment("pay-3", "bk-3", "303", 0))

		d.paymentWriteRepo.AssertCalled(t, "IncrementBookingSyncAttempts", mock.Anything, "pay-3")
		d.paymentWriteRepo.AssertNotCalled(t, "MarkBookingSynced", mock.Anything, mock.Anything)
		d.refundWriteRepo.AssertNotCalled(t, "CreateRefund", mock.Anything, mock.Anything)
		assert.Equal(t, []string{domain.PaymentEventBookingSyncFailed}, d.paymentHistoryStatuses("pay-3"))
	})

	t.Run("5e échec → remboursement intégral bookingUnavailable sur le montant encaissé", func(t *testing.T) {
		d := newCollectTestService()
		payment := unsyncedHeldPayment("pay-5", "bk-5", "505", 4)
		d.bookingClient.On("ConfirmPayment", mock.Anything, "bk-5", "505").Return("", bookingDownErr)
		d.paymentWriteRepo.On("IncrementBookingSyncAttempts", mock.Anything, "pay-5").Return(5, nil)
		// booking-service toujours indisponible : repli sur payment.Amount
		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-5").Return(nil, bookingDownErr)
		d.expectFullRefund(payment, domain.RefundReasonBookingUnavailable)

		retry(t, d, payment)

		d.refundWriteRepo.AssertNumberOfCalls(t, "CreateRefund", 1)
		d.paymentWriteRepo.AssertCalled(t, "UpdatePaymentStatus", mock.Anything, "pay-5", domain.PaymentStatusRefunded, "")
		assert.Equal(t, []string{
			domain.PaymentEventBookingSyncFailed, domain.PaymentEventBookingSyncAbandoned, "refunded",
		}, d.paymentHistoryStatuses("pay-5"))
		assert.Contains(t, d.paymentHistory()[0].Notes, "Tentative 5")
	})

	t.Run("plafond atteint, remboursement précédent en échec → remboursé sans ConfirmPayment", func(t *testing.T) {
		d := newCollectTestService()
		payment := unsyncedHeldPayment("pay-6", "bk-6", "606", 5)
		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-6").Return(nil, bookingDownErr)
		d.expectFullRefund(payment, domain.RefundReasonBookingUnavailable)

		retry(t, d, payment)

		d.bookingClient.AssertNotCalled(t, "ConfirmPayment", mock.Anything, mock.Anything, mock.Anything)
		d.paymentWriteRepo.AssertNotCalled(t, "IncrementBookingSyncAttempts", mock.Anything, mock.Anything)
		d.refundWriteRepo.AssertNumberOfCalls(t, "CreateRefund", 1)
		// Plafond déjà atteint lors d'un tick précédent : pas de nouvel « abandon »
		assert.Equal(t, []string{"refunded"}, d.paymentHistoryStatuses("pay-6"))
	})

	t.Run("remboursement déjà créé → marqué rattaché, pas de second remboursement", func(t *testing.T) {
		d := newCollectTestService()
		d.refundReadRepo.On("GetByPaymentID", mock.Anything, "pay-4").
			Return(&domain.Refund{RefundID: "ref-4", PaymentID: "pay-4"}, nil)
		d.paymentWriteRepo.On("MarkBookingSynced", mock.Anything, "pay-4").Return(nil)

		retry(t, d, unsyncedHeldPayment("pay-4", "bk-4", "404", 5))

		d.paymentWriteRepo.AssertCalled(t, "MarkBookingSynced", mock.Anything, "pay-4")
		d.bookingClient.AssertNotCalled(t, "ConfirmPayment", mock.Anything, mock.Anything, mock.Anything)
		d.refundWriteRepo.AssertNotCalled(t, "CreateRefund", mock.Anything, mock.Anything)
		assert.Equal(t, []string{domain.PaymentEventBookingSynced}, d.paymentHistoryStatuses("pay-4"))
	})
}
