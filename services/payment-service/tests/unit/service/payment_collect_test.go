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
		d.payoutHistoryRepo,
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

		result, err := d.svc.VerifyPayment(passengerCtx(), "pay-1")

		require.NoError(t, err)
		assert.Equal(t, "held", result.Status)
		assert.True(t, result.IsFinal)
		assert.Empty(t, result.PaymentURL)
		d.bookingClient.AssertCalled(t, "ConfirmPayment", mock.Anything, "bk-1", "101")
		d.bookingClient.AssertNotCalled(t, "FailPayment", mock.Anything, mock.Anything, mock.Anything)
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
	})

	t.Run("approbation pour une réservation annulée → remboursement intégral, pas de PAYMENT_COMPLETED", func(t *testing.T) {
		rdb := newNotifRedis(t)
		d := buildCollectTestService(rdb)
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(pendingPayment(), nil).Once()
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(withStatus(pendingPayment(), domain.PaymentStatusHeld), nil).Once()
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(withStatus(pendingPayment(), domain.PaymentStatusRefunded), nil)
		booking := payableBooking(time.Now().UTC())
		booking.Status = "cancelled"
		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-1").Return(booking, nil)
		d.expectOwner()
		d.fedapay.On("GetTransaction", 101).Return(&fedapay.Transaction{ID: 101, Status: "approved"}, nil)
		d.paymentWriteRepo.On("UpdatePaymentStatus", mock.Anything, "pay-1", domain.PaymentStatusHeld, "101").Return(nil)
		d.paymentWriteRepo.On("UpdateProviderStatus", mock.Anything, "pay-1", "approved").Return(nil)
		d.bookingClient.On("ConfirmPayment", mock.Anything, "bk-1", "101").Return("", paymentErrors.ErrorBookingNotConfirmable)
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
		d.paymentReadRepo.On("GetByID", mock.Anything, "pay-1").Return(withStatus(pendingPayment(), domain.PaymentStatusHeld), nil)
		d.bookingClient.On("GetBookingDetails", mock.Anything, "bk-1").Return(payableBooking(time.Now().UTC()), nil)
		d.expectOwner()
		d.fedapay.On("GetTransaction", 101).Return(&fedapay.Transaction{ID: 101, Status: "approved"}, nil)
		d.paymentWriteRepo.On("UpdatePaymentStatus", mock.Anything, "pay-1", domain.PaymentStatusHeld, "101").Return(nil)
		d.paymentWriteRepo.On("UpdateProviderStatus", mock.Anything, "pay-1", "approved").Return(nil)
		d.bookingClient.On("ConfirmPayment", mock.Anything, "bk-1", "101").Return("", paymentErrors.ErrorBookingNotConfirmable)

		result, err := d.svc.VerifyPayment(passengerCtx(), "pay-1")

		require.NoError(t, err)
		assert.Equal(t, "held", result.Status)
		d.refundWriteRepo.AssertNotCalled(t, "CreateRefund", mock.Anything, mock.Anything)
		assert.Empty(t, publishedEvents(t, rdb))
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
	t.Run("supprime la transaction FedaPay et libère les places", func(t *testing.T) {
		d := newCollectTestService()
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
	})
}

func TestCancelPaymentForBooking(t *testing.T) {
	t.Run("supprime la collecte sans rappeler FailPayment", func(t *testing.T) {
		d := newCollectTestService()
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
		d.bookingClient.AssertNotCalled(t, "FailPayment", mock.Anything, mock.Anything, mock.Anything)
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

		result, err := d.svc.HandlePaymentCallback(context.Background(), "101")

		require.NoError(t, err)
		assert.Equal(t, "held", result.Status)
		assert.Equal(t, "pay-1", result.PaymentID)
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

	err := d.svc.ProcessWebhook(context.Background(), &serviceInterfaces.ProcessWebhookInput{
		Signature:  "sig",
		RawPayload: payload,
	})

	require.NoError(t, err)
	d.paymentWriteRepo.AssertNotCalled(t, "MarkPaymentFailed", mock.Anything, mock.Anything, mock.Anything)
	d.bookingClient.AssertNotCalled(t, "FailPayment", mock.Anything, mock.Anything, mock.Anything)
}

// =============================================================================
// Expiration (10 min) — suppression FedaPay et libération des places
// =============================================================================

func TestExpireDuePayments(t *testing.T) {
	d := newCollectTestService()
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

	// Payé juste avant l'échéance → held, pas de DELETE
	d.fedapay.On("GetTransaction", 202).Return(&fedapay.Transaction{ID: 202, Status: "approved"}, nil)
	d.paymentWriteRepo.On("UpdatePaymentStatus", mock.Anything, "pay-2", domain.PaymentStatusHeld, "202").Return(nil)
	d.paymentWriteRepo.On("UpdateProviderStatus", mock.Anything, "pay-2", "approved").Return(nil)
	d.bookingClient.On("ConfirmPayment", mock.Anything, "bk-2", "202").Return("pendingApproval", nil)

	// DELETE en échec, transaction toujours pending → retentée au prochain tick
	d.fedapay.On("GetTransaction", 303).Return(&fedapay.Transaction{ID: 303, Status: "pending"}, nil)
	d.fedapay.On("CancelTransaction", 303).Return(errors.New("fedapay down"))

	require.NoError(t, impl.ExpireDuePayments(context.Background(), 10))

	d.bookingClient.AssertCalled(t, "FailPayment", mock.Anything, "bk-1", mock.Anything)
	d.fedapay.AssertNotCalled(t, "CancelTransaction", 202)
	d.bookingClient.AssertCalled(t, "ConfirmPayment", mock.Anything, "bk-2", "202")
	d.paymentWriteRepo.AssertNotCalled(t, "MarkPaymentFailed", mock.Anything, "pay-3", mock.Anything)
	d.bookingClient.AssertNotCalled(t, "FailPayment", mock.Anything, "bk-3", mock.Anything)
}
