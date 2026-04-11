package service

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/pkg/notification"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/cache"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/config"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/fedapay"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/payment-service/internal/repository/interfaces"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/payment-service/internal/service/interfaces"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
)

type paymentServiceImpl struct {
	paymentReadRepo  repoInterfaces.PaymentRepositoryRead
	paymentWriteRepo repoInterfaces.PaymentRepositoryWrite
	refundReadRepo   repoInterfaces.RefundRepositoryRead
	refundWriteRepo  repoInterfaces.RefundRepositoryWrite
	payoutReadRepo   repoInterfaces.PayoutRepositoryRead
	payoutWriteRepo  repoInterfaces.PayoutRepositoryWrite
	bookingClient    client.BookingClient
	userClient       client.UserClient
	fedapayClient    *fedapay.Client
	cache            *cache.PaymentCache
	notifRedis       *redis.Client
	cfg              *config.Config
	logger           *zap.Logger
	webhookSem       chan struct{} // sémaphore pour limiter les webhooks concurrents
}

// NewPaymentService crée le service de paiement.
func NewPaymentService(
	paymentReadRepo repoInterfaces.PaymentRepositoryRead,
	paymentWriteRepo repoInterfaces.PaymentRepositoryWrite,
	refundReadRepo repoInterfaces.RefundRepositoryRead,
	refundWriteRepo repoInterfaces.RefundRepositoryWrite,
	payoutReadRepo repoInterfaces.PayoutRepositoryRead,
	payoutWriteRepo repoInterfaces.PayoutRepositoryWrite,
	bookingClient client.BookingClient,
	userClient client.UserClient,
	fedapayClient *fedapay.Client,
	paymentCache *cache.PaymentCache,
	notifRedis *redis.Client,
	cfg *config.Config,
	logger *zap.Logger,
) serviceInterfaces.PaymentService {
	webhookMaxConcurrent := cfg.Worker.WebhookMaxConcurrent
	if webhookMaxConcurrent <= 0 {
		webhookMaxConcurrent = 10
	}

	return &paymentServiceImpl{
		paymentReadRepo:  paymentReadRepo,
		paymentWriteRepo: paymentWriteRepo,
		refundReadRepo:   refundReadRepo,
		refundWriteRepo:  refundWriteRepo,
		payoutReadRepo:   payoutReadRepo,
		payoutWriteRepo:  payoutWriteRepo,
		bookingClient:    bookingClient,
		userClient:       userClient,
		fedapayClient:    fedapayClient,
		cache:            paymentCache,
		notifRedis:       notifRedis,
		cfg:              cfg,
		logger:           logger.Named("service"),
		webhookSem:       make(chan struct{}, webhookMaxConcurrent),
	}
}

// AsImpl retourne l'implémentation concrète (pour accéder au payout worker).
func AsImpl(s serviceInterfaces.PaymentService) *paymentServiceImpl {
	impl, ok := s.(*paymentServiceImpl)
	if !ok {
		return nil
	}
	return impl
}

// generateAlphanumeric génère une chaîne aléatoire de lettres majuscules et chiffres.
func generateAlphanumeric(length int) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var sb strings.Builder
	sb.Grow(length)
	for i := 0; i < length; i++ {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		sb.WriteByte(charset[idx.Int64()])
	}
	return sb.String()
}

// =============================================================================
// CreatePayment
// =============================================================================

func (s *paymentServiceImpl) CreatePayment(ctx context.Context, input *serviceInterfaces.CreatePaymentInput) (*serviceInterfaces.CreatePaymentResult, error) {
	s.logger.Info("creating payment",
		zap.String("bookingID", input.BookingID),
		zap.Int("amount", input.Amount),
		zap.String("method", input.PaymentMethod),
	)

	if input.PaymentMethod != string(domain.PaymentMobileMoney) {
		return nil, paymentErrors.ErrorInvalidInput
	}
	if input.MobileMoneyMode != "moov_tg" && input.MobileMoneyMode != "togocel" && input.MobileMoneyMode != "momo_test" {
		return nil, paymentErrors.ErrorInvalidInput
	}

	// Vérifier qu'il n'existe pas déjà un paiement actif pour ce booking
	hasActive, err := s.paymentReadRepo.HasActivePayment(ctx, input.BookingID)
	if err != nil {
		s.logger.Error("check active payment failed", zap.Error(err))
		return nil, paymentErrors.ErrorInternalServer
	}
	if hasActive {
		return nil, paymentErrors.ErrorDuplicatePayment
	}

	// Récupérer les infos du passager via booking → user
	booking, err := s.bookingClient.GetBookingDetails(ctx, input.BookingID)
	if err != nil {
		s.logger.Error("get booking details failed", zap.Error(err))
		return nil, paymentErrors.ErrorInternalServer
	}

	userInfo, err := s.userClient.GetUserByUserID(ctx, booking.PassengerID)
	if err != nil {
		s.logger.Error("get user info failed", zap.Error(err), zap.String("passengerID", booking.PassengerID))
		return nil, paymentErrors.ErrorInternalServer
	}

	// Créer la transaction FedaPay avec les vraies infos du passager
	customer := fedapay.CustomerPayload{
		FirstName: userInfo.FirstName,
		LastName:  userInfo.Name,
		PhoneNumber: &fedapay.PhoneNumberPayload{
			Number:  input.PassengerPhoneNumber,
			Country: "tg",
		},
	}

	transaction, err := s.fedapayClient.CreateTransaction(input.Amount, fmt.Sprintf("Paiement réservation %s", booking.BookingReference), customer)
	if err != nil {
		s.logger.Error("fedapay create transaction failed", zap.Error(err))
		return nil, paymentErrors.ErrorFedaPayAPIError
	}

	// Obtenir le token d'envoi
	tokenResp, err := s.fedapayClient.GetTransactionToken(transaction.ID)
	if err != nil {
		s.logger.Error("fedapay get transaction token failed", zap.Error(err))
		return nil, paymentErrors.ErrorFedaPayAPIError
	}

	// Sauvegarder en DB AVANT l'envoi USSD pour éviter la race condition :
	// FedaPay envoie le webhook quasi-instantanément après SendTransaction,
	// le paiement doit déjà exister en DB pour que ProcessWebhook le trouve.
	paymentID := uuid.New().String()
	paymentRef := fmt.Sprintf("PAY-%s-%s", time.Now().UTC().Format("20060102"), generateAlphanumeric(6))

	payment := &domain.Payment{
		PaymentID:             paymentID,
		BookingID:             input.BookingID,
		TripID:                input.TripID,
		Amount:                input.Amount,
		PaymentMethod:         domain.PaymentMethod(input.PaymentMethod),
		PaymentProvider:       "fedapay",
		Status:                domain.PaymentStatusPending,
		ExternalTransactionID: strconv.Itoa(transaction.ID),
		PaymentReference:      paymentRef,
	}

	if err := s.paymentWriteRepo.CreatePayment(ctx, payment); err != nil {
		s.logger.Error("save payment failed", zap.Error(err))
		return nil, paymentErrors.ErrorInternalServer
	}

	// Envoyer en USSD avec le token
	_, err = s.fedapayClient.SendTransaction(tokenResp.Token, input.MobileMoneyMode, input.PassengerPhoneNumber)
	if err != nil {
		s.logger.Error("fedapay send transaction failed", zap.Error(err))
		return nil, paymentErrors.ErrorFedaPayAPIError
	}

	return &serviceInterfaces.CreatePaymentResult{
		PaymentID:        paymentID,
		PaymentReference: paymentRef,
		Status:           string(domain.PaymentStatusPending),
	}, nil
}

// =============================================================================
// ProcessWebhook
// =============================================================================

func (s *paymentServiceImpl) ProcessWebhook(ctx context.Context, input *serviceInterfaces.ProcessWebhookInput) error {
	// Limiter les webhooks concurrents pour ne pas saturer le pool DB
	select {
	case s.webhookSem <- struct{}{}:
		defer func() { <-s.webhookSem }()
	case <-ctx.Done():
		return ctx.Err()
	}

	s.logger.Debug("processing webhook")

	// Vérifier la signature HMAC
	if !s.fedapayClient.VerifyWebhookSignature(input.Signature, input.RawPayload) {
		s.logger.Warn("webhook signature verification failed",
			zap.String("receivedSignature", input.Signature),
		)
		return paymentErrors.ErrorWebhookVerificationFailed
	}

	// Parser le payload
	payload, err := fedapay.ParseWebhookEvent(input.RawPayload)
	if err != nil {
		s.logger.Error("parse webhook payload failed", zap.Error(err))
		return paymentErrors.ErrorInvalidInput
	}

	// Idempotence — sauvegarder l'événement
	now := time.Now().UTC()
	eventID := uuid.New().String()
	payloadJSON, _ := json.Marshal(payload) //nolint:errcheck

	webhookEvent := &domain.WebhookEvent{
		EventID:        eventID,
		FedapayEventID: strconv.Itoa(payload.Entity.ID) + ":" + payload.Name,
		EventType:      payload.Name,
		Payload:        string(payloadJSON),
		ProcessedAt:    &now,
	}

	if err := s.paymentWriteRepo.SaveWebhookEvent(ctx, webhookEvent); err != nil {
		if err == paymentErrors.ErrorDuplicateWebhookEvent {
			s.logger.Info("duplicate webhook event, skipping", zap.String("eventType", payload.Name))
			return nil
		}
		return err
	}

	// Trouver le paiement par l'external transaction ID
	externalID := strconv.Itoa(payload.Entity.ID)
	payment, err := s.paymentReadRepo.GetByExternalTransactionID(ctx, externalID)
	if err != nil {
		s.logger.Error("payment not found for webhook", zap.String("externalID", externalID))
		return err
	}

	// Traiter selon le type d'événement
	switch payload.Name {
	case "transaction.created":
		// Transaction créée côté FedaPay — rien à faire, le paiement est déjà en DB
		s.logger.Info("transaction created", zap.String("paymentID", payment.PaymentID))

	case "transaction.approved":
		if payment.Status != domain.PaymentStatusPending {
			s.logger.Info("payment already processed", zap.String("paymentID", payment.PaymentID), zap.String("status", string(payment.Status)))
			return nil
		}

		// MAJ status → held (verrouillage optimiste : AND status = 'pending')
		if err := s.paymentWriteRepo.UpdatePaymentStatus(ctx, payment.PaymentID, domain.PaymentStatusHeld, externalID); err != nil {
			if err == paymentErrors.ErrorPaymentAlreadyProcessed {
				s.logger.Info("payment already processed by another webhook, skipping", zap.String("paymentID", payment.PaymentID))
				return nil
			}
			return err
		}

		// Invalider le cache
		if s.cache != nil {
			s.cache.InvalidatePaymentByBookingID(ctx, payment.BookingID)
		}

		// Notifier booking-service
		if err := s.bookingClient.ConfirmPayment(ctx, payment.BookingID, externalID); err != nil {
			s.logger.Error("confirm payment to booking-service failed", zap.Error(err))
			// Le paiement est held — on ne rollback pas
		}

		// Notifier le passager (non bloquant)
		if s.notifRedis != nil {
			if booking, err := s.bookingClient.GetBookingDetails(ctx, payment.BookingID); err == nil {
				if pubErr := notification.Publish(ctx, s.notifRedis, notification.Event{
					EventType:     notification.PaymentCompleted,
					UserID:        booking.PassengerID,
					ReferenceID:   payment.PaymentID,
					ReferenceType: notification.RefPayment,
					Payload: map[string]string{
						"amount": fmt.Sprintf("%d", payment.Amount),
					},
				}); pubErr != nil {
					s.logger.Error("failed to publish PAYMENT_COMPLETED notification", zap.Error(pubErr))
				}
			}
		}

		s.logger.Info("payment held", zap.String("paymentID", payment.PaymentID))

	case "transaction.declined", "transaction.canceled", "transaction.expired", "transaction.deleted":
		if payment.Status != domain.PaymentStatusPending {
			s.logger.Info("payment already processed, skipping", zap.String("paymentID", payment.PaymentID), zap.String("status", string(payment.Status)))
			return nil
		}

		reason := fmt.Sprintf("FedaPay: %s", payload.Name)

		// Marquer le paiement comme échoué (verrouillage optimiste : AND status = 'pending')
		if err := s.paymentWriteRepo.MarkPaymentFailed(ctx, payment.PaymentID, reason); err != nil {
			if err == paymentErrors.ErrorPaymentAlreadyProcessed {
				s.logger.Info("payment already processed by another webhook, skipping", zap.String("paymentID", payment.PaymentID))
				return nil
			}
			return err
		}

		if s.cache != nil {
			s.cache.InvalidatePaymentByBookingID(ctx, payment.BookingID)
		}

		// Notifier booking-service pour restaurer les places et changer le statut
		if err := s.bookingClient.FailPayment(ctx, payment.BookingID, reason); err != nil {
			s.logger.Error("fail payment to booking-service failed", zap.Error(err))
		}

		// Notifier le passager (non bloquant)
		if s.notifRedis != nil {
			if booking, err := s.bookingClient.GetBookingDetails(ctx, payment.BookingID); err == nil {
				if pubErr := notification.Publish(ctx, s.notifRedis, notification.Event{
					EventType:     notification.PaymentFailed,
					UserID:        booking.PassengerID,
					ReferenceID:   payment.PaymentID,
					ReferenceType: notification.RefPayment,
				}); pubErr != nil {
					s.logger.Error("failed to publish PAYMENT_FAILED notification", zap.Error(pubErr))
				}
			}
		}

		s.logger.Info("payment failed", zap.String("paymentID", payment.PaymentID), zap.String("reason", reason))

	default:
		s.logger.Debug("unhandled webhook event", zap.String("eventType", payload.Name))
	}

	return nil
}

// =============================================================================
// GetPaymentStatus / GetPaymentByBooking
// =============================================================================

func (s *paymentServiceImpl) GetPaymentStatus(ctx context.Context, paymentID string) (*serviceInterfaces.PaymentStatusResult, error) {
	payment, err := s.paymentReadRepo.GetByID(ctx, paymentID)
	if err != nil {
		return nil, err
	}

	return &serviceInterfaces.PaymentStatusResult{
		PaymentID:        payment.PaymentID,
		BookingID:        payment.BookingID,
		Amount:           payment.Amount,
		PaymentMethod:    string(payment.PaymentMethod),
		Status:           string(payment.Status),
		PaymentReference: payment.PaymentReference,
		CreatedAt:        payment.CreatedAt.Format(time.RFC3339),
		CompletedAt:      formatOptionalTime(payment.CompletedAt),
	}, nil
}

func (s *paymentServiceImpl) GetPaymentByBooking(ctx context.Context, bookingID string) (*serviceInterfaces.PaymentByBookingResult, error) {
	// Essayer le cache
	if s.cache != nil {
		cached, _ := s.cache.GetPaymentByBookingID(ctx, bookingID)
		if cached != nil {
			return &serviceInterfaces.PaymentByBookingResult{
				PaymentID:        cached.PaymentID,
				BookingID:        cached.BookingID,
				Amount:           cached.Amount,
				PaymentMethod:    string(cached.PaymentMethod),
				Status:           string(cached.Status),
				PaymentReference: cached.PaymentReference,
				CreatedAt:        cached.CreatedAt.Format(time.RFC3339),
			}, nil
		}
	}

	payment, err := s.paymentReadRepo.GetByBookingID(ctx, bookingID)
	if err != nil {
		return nil, err
	}

	// Mettre en cache
	if s.cache != nil {
		s.cache.SetPaymentByBookingID(ctx, bookingID, payment) //nolint:errcheck
	}

	return &serviceInterfaces.PaymentByBookingResult{
		PaymentID:        payment.PaymentID,
		BookingID:        payment.BookingID,
		Amount:           payment.Amount,
		PaymentMethod:    string(payment.PaymentMethod),
		Status:           string(payment.Status),
		PaymentReference: payment.PaymentReference,
		CreatedAt:        payment.CreatedAt.Format(time.RFC3339),
	}, nil
}

// =============================================================================
// ReleasePayment
// =============================================================================

func (s *paymentServiceImpl) ReleasePayment(ctx context.Context, bookingID string) error {
	payment, err := s.paymentReadRepo.GetByBookingID(ctx, bookingID)
	if err != nil {
		return err
	}

	if payment.Status != domain.PaymentStatusHeld {
		s.logger.Warn("cannot release payment: not held",
			zap.String("paymentID", payment.PaymentID),
			zap.String("status", string(payment.Status)),
		)
		return paymentErrors.ErrorInvalidPaymentStatus
	}

	if err := s.paymentWriteRepo.UpdatePaymentStatus(ctx, payment.PaymentID, domain.PaymentStatusReleased, ""); err != nil {
		return err
	}

	if s.cache != nil {
		s.cache.InvalidatePaymentByBookingID(ctx, bookingID)
	}

	s.logger.Info("payment released", zap.String("paymentID", payment.PaymentID))
	return nil
}

// =============================================================================
// RequestRefund
// =============================================================================

func (s *paymentServiceImpl) RequestRefund(ctx context.Context, input *serviceInterfaces.RequestRefundInput) (*serviceInterfaces.RequestRefundResult, error) {
	s.logger.Info("requesting refund", zap.String("bookingID", input.BookingID), zap.String("reason", input.RefundReason))

	// Trouver le paiement
	payment, err := s.paymentReadRepo.GetByBookingID(ctx, input.BookingID)
	if err != nil {
		return nil, err
	}

	if payment.Status != domain.PaymentStatusHeld && payment.Status != domain.PaymentStatusReleased {
		return nil, paymentErrors.ErrorInvalidPaymentStatus
	}

	// Parser les dates
	departureDatetime, err := time.Parse(time.RFC3339, input.DepartureDatetime)
	if err != nil {
		return nil, paymentErrors.ErrorInvalidInput
	}

	cancelledAt, err := time.Parse(time.RFC3339, input.CancelledAt)
	if err != nil {
		return nil, paymentErrors.ErrorInvalidInput
	}

	var approvedAt *time.Time
	if input.ApprovedAt != "" {
		t, err := time.Parse(time.RFC3339, input.ApprovedAt)
		if err != nil {
			return nil, paymentErrors.ErrorInvalidInput
		}
		approvedAt = &t
	}

	// Déterminer la règle de remboursement
	reason := domain.RefundReason(input.RefundReason)
	rule := DetermineRefundRule(reason, departureDatetime, approvedAt, cancelledAt, s.cfg.Refund)

	// Calculer les montants
	calc := CalculateRefund(input.OriginalAmount, input.ServiceFee, rule)

	// Créer le refund
	refundID := uuid.New().String()
	refundRef := fmt.Sprintf("RMB-%s-%s", time.Now().UTC().Format("20060102"), generateAlphanumeric(6))
	now := time.Now().UTC()

	refund := &domain.Refund{
		RefundID:           refundID,
		RefundReference:    refundRef,
		PaymentID:          payment.PaymentID,
		BookingID:          input.BookingID,
		RefundReason:       reason,
		RefundRuleApplied:  rule,
		OriginalAmount:     input.OriginalAmount,
		RefundPercentage:   int16(calc.RefundPercentage),
		RefundAmount:       calc.RefundAmount,
		ServiceFeeRefunded: calc.ServiceFeeRefunded,
		AmountToPassenger:  calc.AmountToPassenger,
		AmountToDriver:     calc.AmountToDriver,
		AmountToPlatform:   calc.AmountToPlatform,
		Status:             domain.RefundStatusCompleted,
		RefundMethod:       string(payment.PaymentMethod),
		ProcessedAt:        &now,
	}

	if s.refundWriteRepo != nil {
		if err := s.refundWriteRepo.CreateRefund(ctx, refund); err != nil {
			return nil, err
		}
	}

	// MAJ statut du paiement → refunded
	if err := s.paymentWriteRepo.UpdatePaymentStatus(ctx, payment.PaymentID, domain.PaymentStatusRefunded, ""); err != nil {
		s.logger.Error("update payment status to refunded failed", zap.Error(err))
	}

	if s.cache != nil {
		s.cache.InvalidatePaymentByBookingID(ctx, input.BookingID)
	}

	s.logger.Info("refund created",
		zap.String("refundID", refundID),
		zap.String("rule", string(rule)),
		zap.Int("refundAmount", calc.RefundAmount),
	)

	// Notifier le passager (non bloquant)
	if s.notifRedis != nil {
		if booking, err := s.bookingClient.GetBookingDetails(ctx, input.BookingID); err == nil {
			if pubErr := notification.Publish(ctx, s.notifRedis, notification.Event{
				EventType:     notification.RefundProcessed,
				UserID:        booking.PassengerID,
				ReferenceID:   refundID,
				ReferenceType: notification.RefRefund,
				Payload: map[string]string{
					"amount": fmt.Sprintf("%d", calc.RefundAmount),
				},
			}); pubErr != nil {
				s.logger.Error("failed to publish REFUND_PROCESSED notification", zap.Error(pubErr))
			}
		}
	}

	return &serviceInterfaces.RequestRefundResult{
		RefundID:        refundID,
		RefundReference: refundRef,
		Status:          string(domain.RefundStatusCompleted),
		RefundAmount:    calc.RefundAmount,
	}, nil
}

// =============================================================================
// GetRefundStatus
// =============================================================================

func (s *paymentServiceImpl) GetRefundStatus(ctx context.Context, refundID string) (*serviceInterfaces.RefundStatusResult, error) {
	if s.refundReadRepo == nil {
		return nil, paymentErrors.ErrorInternalServer
	}

	refund, err := s.refundReadRepo.GetByID(ctx, refundID)
	if err != nil {
		return nil, err
	}

	return &serviceInterfaces.RefundStatusResult{
		RefundID:          refund.RefundID,
		BookingID:         refund.BookingID,
		RefundReason:      string(refund.RefundReason),
		RefundRule:        string(refund.RefundRuleApplied),
		OriginalAmount:    refund.OriginalAmount,
		RefundPercentage:  int(refund.RefundPercentage),
		RefundAmount:      refund.RefundAmount,
		AmountToPassenger: refund.AmountToPassenger,
		AmountToDriver:    refund.AmountToDriver,
		AmountToPlatform:  refund.AmountToPlatform,
		Status:            string(refund.Status),
		ProcessedAt:       formatOptionalTime(refund.ProcessedAt),
		CompletedAt:       formatOptionalTime(refund.CompletedAt),
	}, nil
}

// =============================================================================
// GetPayoutStatus / GetDriverPayouts (implémentés en Phase 5)
// =============================================================================

func (s *paymentServiceImpl) GetPayoutStatus(ctx context.Context, payoutID string) (*serviceInterfaces.PayoutStatusResult, error) {
	if s.payoutReadRepo == nil {
		return nil, paymentErrors.ErrorInternalServer
	}

	payout, err := s.payoutReadRepo.GetByID(ctx, payoutID)
	if err != nil {
		return nil, err
	}

	var failureReason string
	if payout.FailureReason != nil {
		failureReason = *payout.FailureReason
	}

	return &serviceInterfaces.PayoutStatusResult{
		PayoutID:      payout.PayoutID,
		DriverID:      payout.DriverID,
		TripID:        payout.TripID,
		GrossAmount:   payout.GrossAmount,
		PlatformFee:   payout.PlatformFee,
		NetAmount:     payout.NetAmount,
		Status:        string(payout.Status),
		ScheduledAt:   formatOptionalTime(payout.ScheduledAt),
		CompletedAt:   formatOptionalTime(payout.CompletedAt),
		FailureReason: failureReason,
	}, nil
}

func (s *paymentServiceImpl) GetDriverPayouts(ctx context.Context, driverID string, pageIndex int) ([]*serviceInterfaces.PayoutPreviewResult, error) {
	if s.payoutReadRepo == nil {
		return nil, paymentErrors.ErrorInternalServer
	}

	payouts, err := s.payoutReadRepo.GetDriverPayouts(ctx, driverID, pageIndex)
	if err != nil {
		return nil, err
	}

	results := make([]*serviceInterfaces.PayoutPreviewResult, 0, len(payouts))
	for _, p := range payouts {
		results = append(results, &serviceInterfaces.PayoutPreviewResult{
			PayoutID:        p.PayoutID,
			PayoutReference: p.PayoutReference,
			TripID:          p.TripID,
			NetAmount:       p.NetAmount,
			Status:          string(p.Status),
			CompletedAt:     formatOptionalTime(p.CompletedAt),
			CreatedAt:       p.CreatedAt.Format(time.RFC3339),
		})
	}

	return results, nil
}

// =============================================================================
// Helpers
// =============================================================================

func formatOptionalTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
