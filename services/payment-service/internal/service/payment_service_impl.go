package service

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
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
	paymentReadRepo    repoInterfaces.PaymentRepositoryRead
	paymentWriteRepo   repoInterfaces.PaymentRepositoryWrite
	refundReadRepo     repoInterfaces.RefundRepositoryRead
	refundWriteRepo    repoInterfaces.RefundRepositoryWrite
	payoutReadRepo     repoInterfaces.PayoutRepositoryRead
	payoutWriteRepo    repoInterfaces.PayoutRepositoryWrite
	payoutHistoryRepo  repoInterfaces.PayoutHistoryRepository
	paymentHistoryRepo repoInterfaces.PaymentHistoryRepository
	refundHistoryRepo  repoInterfaces.RefundHistoryRepository
	bookingClient      client.BookingClient
	userClient         client.UserClient
	supportClient      client.SupportClient
	fedapayClient      fedapay.FedaPayClient
	cache              *cache.PaymentCache
	notifRedis         *redis.Client
	cfg                *config.Config
	logger             *zap.Logger
	webhookSem         chan struct{} // sémaphore pour limiter les webhooks concurrents
}

// NewPaymentService crée le service de paiement.
func NewPaymentService(
	paymentReadRepo repoInterfaces.PaymentRepositoryRead,
	paymentWriteRepo repoInterfaces.PaymentRepositoryWrite,
	refundReadRepo repoInterfaces.RefundRepositoryRead,
	refundWriteRepo repoInterfaces.RefundRepositoryWrite,
	payoutReadRepo repoInterfaces.PayoutRepositoryRead,
	payoutWriteRepo repoInterfaces.PayoutRepositoryWrite,
	payoutHistoryRepo repoInterfaces.PayoutHistoryRepository,
	paymentHistoryRepo repoInterfaces.PaymentHistoryRepository,
	refundHistoryRepo repoInterfaces.RefundHistoryRepository,
	bookingClient client.BookingClient,
	userClient client.UserClient,
	supportClient client.SupportClient,
	fedapayClient fedapay.FedaPayClient,
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
		paymentReadRepo:    paymentReadRepo,
		paymentWriteRepo:   paymentWriteRepo,
		refundReadRepo:     refundReadRepo,
		refundWriteRepo:    refundWriteRepo,
		payoutReadRepo:     payoutReadRepo,
		payoutWriteRepo:    payoutWriteRepo,
		payoutHistoryRepo:  payoutHistoryRepo,
		paymentHistoryRepo: paymentHistoryRepo,
		refundHistoryRepo:  refundHistoryRepo,
		bookingClient:      bookingClient,
		userClient:         userClient,
		supportClient:      supportClient,
		fedapayClient:      fedapayClient,
		cache:              paymentCache,
		notifRedis:         notifRedis,
		cfg:                cfg,
		logger:             logger.Named("service"),
		webhookSem:         make(chan struct{}, webhookMaxConcurrent),
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
	ctx = withHistoryOrigin(ctx, domain.HistoryInitiatedBySystem, domain.HistorySourceFedaPayWebhook)

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

	// Idempotence — enregistrer l'événement comme reçu (processed_at posé après traitement réussi)
	eventID := uuid.New().String()
	payloadJSON, _ := json.Marshal(payload) //nolint:errcheck

	webhookEvent := &domain.WebhookEvent{
		EventID:        eventID,
		FedapayEventID: strconv.Itoa(payload.Entity.ID) + ":" + payload.Name,
		EventType:      payload.Name,
		Payload:        string(payloadJSON),
	}

	if err := s.paymentWriteRepo.SaveWebhookEvent(ctx, webhookEvent); err != nil {
		if err == paymentErrors.ErrorDuplicateWebhookEvent {
			s.logger.Info("duplicate webhook event, skipping", zap.String("eventType", payload.Name))
			return nil
		}
		return err
	}

	// En cas d'échec, l'événement reste non traité : le retry FedaPay le rejouera
	if err := s.handleWebhookEvent(ctx, payload); err != nil {
		return err
	}

	if err := s.paymentWriteRepo.MarkWebhookEventProcessed(ctx, webhookEvent.FedapayEventID); err != nil {
		// Traitement déjà appliqué : un éventuel retry rejouera une logique idempotente
		s.logger.Warn("mark webhook event processed failed",
			zap.String("fedapayEventID", webhookEvent.FedapayEventID),
			zap.Error(err),
		)
	}

	return nil
}

// handleWebhookEvent applique un événement webhook FedaPay (payout ou collecte).
func (s *paymentServiceImpl) handleWebhookEvent(ctx context.Context, payload *fedapay.WebhookPayload) error {
	// Virements sortants (remboursements et payouts chauffeurs) : les noms des événements payout.* ne sont
	// pas documentés par FedaPay, le statut est relu via GetPayout (cf. payout_confirmation.go)
	if strings.HasPrefix(payload.Name, "payout.") {
		return s.confirmProviderPayout(ctx, strconv.Itoa(payload.Entity.ID))
	}

	// Événements de collecte : traduits en statut FedaPay puis appliqués via la logique commune
	providerStatus := providerStatusFromWebhook(payload)
	if providerStatus == "" {
		s.logger.Debug("unhandled webhook event", zap.String("eventType", payload.Name))
		return nil
	}

	externalID := strconv.Itoa(payload.Entity.ID)
	payment, err := s.paymentReadRepo.GetByExternalTransactionID(ctx, externalID)
	if err != nil {
		s.logger.Error("payment not found for webhook", zap.String("externalID", externalID))
		return err
	}

	if _, err := s.applyProviderStatus(ctx, payment, providerStatus); err != nil {
		s.logger.Error("apply webhook status failed",
			zap.String("paymentID", payment.PaymentID),
			zap.String("eventType", payload.Name),
			zap.Error(err),
		)
		return err
	}

	return nil
}

// providerStatusFromWebhook traduit un événement webhook de collecte en statut FedaPay.
// Retourne "" pour les événements sans effet (transaction.created, payouts inconnus…).
func providerStatusFromWebhook(payload *fedapay.WebhookPayload) string {
	switch payload.Name {
	case "transaction.approved":
		return fedapay.TransactionStatusApproved
	case "transaction.transferred":
		return fedapay.TransactionStatusTransferred
	case "transaction.declined":
		return fedapay.TransactionStatusDeclined
	case "transaction.canceled":
		return fedapay.TransactionStatusCanceled
	case "transaction.expired":
		return fedapay.TransactionStatusExpired
	case "transaction.deleted":
		return fedapay.TransactionStatusDeleted
	default:
		return ""
	}
}

// =============================================================================
// GetPaymentStatus / GetPaymentByBooking
// =============================================================================

func (s *paymentServiceImpl) GetPaymentStatus(ctx context.Context, paymentID string) (*serviceInterfaces.PaymentStatusResult, error) {
	payment, err := s.paymentReadRepo.GetByID(ctx, paymentID)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeBookingRead(ctx, payment.BookingID); err != nil {
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
		PaymentURL:       pendingPaymentURL(payment),
		ProviderStatus:   payment.ProviderStatus,
		ExpiresAt:        formatOptionalTime(&payment.ExpiresAt),
	}, nil
}

func (s *paymentServiceImpl) GetPaymentByBooking(ctx context.Context, bookingID string) (*serviceInterfaces.PaymentByBookingResult, error) {
	if bookingID == "" {
		return nil, paymentErrors.ErrorInvalidInput
	}
	// Contrôle avant toute lecture (cache compris)
	if err := s.authorizeBookingRead(ctx, bookingID); err != nil {
		return nil, err
	}
	return s.GetPaymentByBookingInternal(ctx, bookingID)
}

// GetPaymentByBookingInternal retourne le paiement d'une réservation sans contrôle d'appartenance.
// Réservé aux appels inter-services (sweeper paymentPending de booking-service).
func (s *paymentServiceImpl) GetPaymentByBookingInternal(ctx context.Context, bookingID string) (*serviceInterfaces.PaymentByBookingResult, error) {
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
				CompletedAt:      formatOptionalTime(cached.CompletedAt),
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
		CompletedAt:      formatOptionalTime(payment.CompletedAt),
		PaymentURL:       pendingPaymentURL(payment),
		ProviderStatus:   payment.ProviderStatus,
		ExpiresAt:        formatOptionalTime(&payment.ExpiresAt),
	}, nil
}

// =============================================================================
// ReleasePayment
// =============================================================================

func (s *paymentServiceImpl) ReleasePayment(ctx context.Context, bookingID string) error {
	ctx = withHistoryOrigin(ctx, domain.HistoryInitiatedByBookingService, domain.HistorySourceBookingService)

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
	s.writePaymentHistory(ctx, payment.PaymentID, string(domain.PaymentStatusReleased), "", "Trajet terminé, fonds libérés pour le versement chauffeur")

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
	ctx = withHistoryOrigin(ctx, domain.HistoryInitiatedByBookingService, domain.HistorySourceBookingService)

	// Un motif hors enum refund_reason ferait échouer l'INSERT avec une erreur SQL opaque
	if !domain.RefundReason(input.RefundReason).IsValid() {
		s.logger.Warn("unknown refund reason",
			zap.String("bookingID", input.BookingID),
			zap.String("reason", input.RefundReason),
		)
		return nil, paymentErrors.ErrorInvalidInput
	}

	// Trouver le paiement
	payment, err := s.paymentReadRepo.GetByBookingID(ctx, input.BookingID)
	if err != nil {
		return nil, err
	}

	return s.createRefundForPayment(ctx, payment, input)
}

// createRefundForPayment crée le remboursement d'un paiement held/released et le passe en refunded.
// Idempotent : un paiement n'a qu'un remboursement (index unique), une nouvelle demande retourne
// celui qui existe déjà (rejeu par booking-service après une réponse perdue).
func (s *paymentServiceImpl) createRefundForPayment(ctx context.Context, payment *domain.Payment, input *serviceInterfaces.RequestRefundInput) (*serviceInterfaces.RequestRefundResult, error) {
	if existing, err := s.existingRefundResult(ctx, payment); err != nil || existing != nil {
		return existing, err
	}

	if payment.Status != domain.PaymentStatusHeld && payment.Status != domain.PaymentStatusReleased {
		return nil, paymentErrors.ErrorInvalidPaymentStatus
	}

	// Parser les dates. Le départ ne sert qu'à la règle des 24h d'une annulation passager :
	// il peut manquer pour les autres motifs (ex. arrêt supprimé, introuvable côté trips-service).
	reason := domain.RefundReason(input.RefundReason)
	var departureDatetime time.Time
	if input.DepartureDatetime != "" || reason == domain.RefundReasonCancelledByPassenger {
		parsed, err := time.Parse(time.RFC3339, input.DepartureDatetime)
		if err != nil {
			return nil, paymentErrors.ErrorInvalidInput
		}
		departureDatetime = parsed
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
	rule := DetermineRefundRule(reason, departureDatetime, approvedAt, cancelledAt, s.cfg.Refund)

	// Calculer les montants
	calc := CalculateRefund(input.OriginalAmount, input.ServiceFee, rule)

	// Créer le refund
	// Si AmountToPassenger == 0 (ex. noShowPassenger), aucun transfert FedaPay n'est requis
	//   → on marque directement 'completed'.
	// Sinon, on crée le refund en 'pending' et le refund_worker.go s'occupera du transfert FedaPay.
	refundID := uuid.New().String()
	refundRef := fmt.Sprintf("RMB-%s-%s", time.Now().UTC().Format("20060102"), generateAlphanumeric(6))
	now := time.Now().UTC()

	var refundStatus domain.RefundStatus
	var completedAt *time.Time
	if calc.AmountToPassenger == 0 {
		refundStatus = domain.RefundStatusCompleted
		completedAt = &now
	} else {
		refundStatus = domain.RefundStatusPending
	}

	// Le chauffeur reçoit sa part si la règle lui en attribue une (annulation tardive ou no-show passager).
	var driverPayoutStatus *string
	if calc.AmountToDriver > 0 {
		s := "pending"
		driverPayoutStatus = &s
	}

	refund := &domain.Refund{
		RefundID:            refundID,
		RefundReference:     refundRef,
		PaymentID:           payment.PaymentID,
		BookingID:           input.BookingID,
		RefundReason:        reason,
		RefundRuleApplied:   rule,
		OriginalAmount:      input.OriginalAmount,
		RefundPercentage:    int16(calc.RefundPercentage),
		RefundAmount:        calc.RefundAmount,
		ServiceFeeRefunded:  calc.ServiceFeeRefunded,
		AmountToPassenger:   calc.AmountToPassenger,
		AmountToDriver:      calc.AmountToDriver,
		AmountToPlatform:    calc.AmountToPlatform,
		Status:              refundStatus,
		RefundMethod:        string(payment.PaymentMethod),
		ProcessedAt:         &now,
		CompletedAt:         completedAt,
		PayoutDestination:   payment.PassengerPhoneNumber,
		PaymentProvider:     "fedapay",
		DriverPayoutStatus:  driverPayoutStatus,
	}

	if s.refundWriteRepo != nil {
		if err := s.refundWriteRepo.CreateRefund(ctx, refund); err != nil {
			if errors.Is(err, paymentErrors.ErrorRefundAlreadyProcessed) {
				// Demande concurrente pour le même paiement : retourner le remboursement gagnant
				if existing, findErr := s.existingRefundResult(ctx, payment); findErr != nil || existing != nil {
					return existing, findErr
				}
			}
			return nil, err
		}
		s.writeRefundHistory(ctx, refundID, string(refundStatus), 0, refundCreatedNotes(refund))
	}

	// MAJ statut du paiement → refunded
	if err := s.paymentWriteRepo.UpdatePaymentStatus(ctx, payment.PaymentID, domain.PaymentStatusRefunded, ""); err != nil {
		s.logger.Error("update payment status to refunded failed", zap.Error(err))
	} else {
		s.writePaymentHistory(ctx, payment.PaymentID, string(domain.PaymentStatusRefunded), "", paymentRefundedNotes(refund))
	}

	if s.cache != nil {
		s.cache.InvalidatePaymentByBookingID(ctx, input.BookingID)
	}

	s.logger.Info("refund created",
		zap.String("refundID", refundID),
		zap.String("rule", string(rule)),
		zap.String("status", string(refundStatus)),
		zap.Int("refundAmount", calc.RefundAmount),
		zap.Int("amountToPassenger", calc.AmountToPassenger),
	)

	// Notifier le passager (non bloquant) que le remboursement est en cours de traitement.
	// Le REFUND_COMPLETED sera publié par le refund_worker après succès FedaPay.
	if s.notifRedis != nil {
		if booking, err := s.bookingClient.GetBookingDetails(ctx, input.BookingID); err == nil {
			if pubErr := notification.Publish(ctx, s.notifRedis, notification.Event{
				EventType:     notification.RefundProcessed,
				UserID:        booking.PassengerID,
				ReferenceID:   refundID,
				ReferenceType: notification.RefRefund,
				Payload: map[string]string{
					"amount":              fmt.Sprintf("%d", calc.RefundAmount),
					"amount_to_passenger": fmt.Sprintf("%d", calc.AmountToPassenger),
					"reason":              string(reason),
				},
			}); pubErr != nil {
				s.logger.Error("failed to publish REFUND_PROCESSED notification", zap.Error(pubErr))
			}
		}
	}

	return &serviceInterfaces.RequestRefundResult{
		RefundID:        refundID,
		RefundReference: refundRef,
		Status:          string(refundStatus),
		RefundAmount:    calc.RefundAmount,
	}, nil
}

// existingRefundResult retourne le remboursement déjà créé pour le paiement, ou nil s'il n'y en a pas.
// Si le passage du paiement en refunded avait échoué, il est retenté.
func (s *paymentServiceImpl) existingRefundResult(
	ctx context.Context, payment *domain.Payment,
) (*serviceInterfaces.RequestRefundResult, error) {
	if s.refundReadRepo == nil {
		return nil, nil
	}
	refund, err := s.refundReadRepo.GetByPaymentID(ctx, payment.PaymentID)
	if err != nil {
		if errors.Is(err, paymentErrors.ErrorRefundNotFound) {
			return nil, nil
		}
		return nil, err
	}

	if payment.Status == domain.PaymentStatusHeld || payment.Status == domain.PaymentStatusReleased {
		err := s.paymentWriteRepo.UpdatePaymentStatus(ctx, payment.PaymentID, domain.PaymentStatusRefunded, "")
		if err != nil {
			s.logger.Error("update payment status to refunded failed", zap.Error(err))
		} else {
			s.writePaymentHistory(ctx, payment.PaymentID, string(domain.PaymentStatusRefunded), "", paymentRefundedNotes(refund))
		}
	}

	s.logger.Info("refund already exists for payment, returning it",
		zap.String("paymentID", payment.PaymentID),
		zap.String("refundID", refund.RefundID),
	)
	return &serviceInterfaces.RequestRefundResult{
		RefundID:        refund.RefundID,
		RefundReference: refund.RefundReference,
		Status:          string(refund.Status),
		RefundAmount:    refund.RefundAmount,
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
	if err := s.authorizeBookingRead(ctx, refund.BookingID); err != nil {
		return nil, err
	}

	return refundToStatusResult(refund), nil
}

// GetRefundByBooking retourne le remboursement le plus récent d'une réservation.
// Symétrique de GetPaymentByBooking : le client ne connaît que le BookingId, le
// RefundId étant attribué côté backend (annulation via booking-service ou worker).
// Retourne ErrorRefundNotFound tant qu'aucun remboursement n'existe pour la réservation.
func (s *paymentServiceImpl) GetRefundByBooking(ctx context.Context, bookingID string) (*serviceInterfaces.RefundStatusResult, error) {
	if s.refundReadRepo == nil {
		return nil, paymentErrors.ErrorInternalServer
	}
	if bookingID == "" {
		return nil, paymentErrors.ErrorInvalidInput
	}
	if err := s.authorizeBookingRead(ctx, bookingID); err != nil {
		return nil, err
	}

	refund, err := s.refundReadRepo.GetByBookingID(ctx, bookingID)
	if err != nil {
		return nil, err
	}

	return refundToStatusResult(refund), nil
}

// authorizeBookingRead vérifie que l'appelant est le passager de la réservation
// (lectures client : GetPaymentStatus, GetPaymentByBooking, GetRefundStatus, GetRefundByBooking).
func (s *paymentServiceImpl) authorizeBookingRead(ctx context.Context, bookingID string) error {
	booking, err := s.getBooking(ctx, bookingID)
	if err != nil {
		return err
	}
	return s.checkBookingOwnership(ctx, booking)
}

// refundToStatusResult projette un Refund domaine vers le résultat exposé aux clients
// (partagé par GetRefundStatus et GetRefundByBooking).
func refundToStatusResult(refund *domain.Refund) *serviceInterfaces.RefundStatusResult {
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
	}
}

// =============================================================================
// GetPayoutStatus / GetDriverPayouts (implémentés en Phase 5)
// =============================================================================

// GetPayoutStatus retourne le statut d'un payout. L'appelant doit être le chauffeur propriétaire du payout.
func (s *paymentServiceImpl) GetPayoutStatus(ctx context.Context, payoutID string) (*serviceInterfaces.PayoutStatusResult, error) {
	if s.payoutReadRepo == nil {
		return nil, paymentErrors.ErrorInternalServer
	}

	payout, err := s.payoutReadRepo.GetByID(ctx, payoutID)
	if err != nil {
		return nil, err
	}

	driverID, err := callerUserID(ctx)
	if err != nil {
		return nil, err
	}
	if driverID != payout.DriverID {
		s.logger.Warn("payout access denied: caller is not the payout driver",
			zap.String("payoutID", payoutID),
		)
		return nil, paymentErrors.ErrorPaymentNotOwned
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

// GetDriverPayouts retourne les payouts du chauffeur appelant (UserID résolu par l'intercepteur).
func (s *paymentServiceImpl) GetDriverPayouts(
	ctx context.Context, pageIndex int,
) ([]*serviceInterfaces.PayoutPreviewResult, error) {
	if s.payoutReadRepo == nil {
		return nil, paymentErrors.ErrorInternalServer
	}
	driverID, err := callerUserID(ctx)
	if err != nil {
		return nil, err
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
// TriggerManualPayout — exécuté par un agent support via api-gateway
// =============================================================================

func (s *paymentServiceImpl) TriggerManualPayout(ctx context.Context, tripID string, supportUserID string) (int, error) {
	if tripID == "" || supportUserID == "" {
		return 0, paymentErrors.ErrorInvalidInput
	}

	// Lock per-trip pour éviter les doublons avec le worker
	if s.cache != nil {
		acquired, err := s.cache.AcquireTripPayoutLock(ctx, tripID)
		if err != nil {
			return 0, paymentErrors.ErrorInternalServer
		}
		if !acquired {
			return 0, paymentErrors.ErrorPayoutAlreadyExists
		}
		defer s.cache.ReleaseTripPayoutLock(ctx, tripID)
	}

	// Précondition : trip éligible (paiements released, aucun pending/held, pas de payout actif)
	ready, err := s.payoutReadRepo.IsTripReadyForPayout(ctx, tripID)
	if err != nil {
		return 0, err
	}
	if !ready {
		return 0, paymentErrors.ErrorTripNotReadyForPayout
	}

	// Autorisation : vérifier l'identité et l'habilitation de l'agent support (source de vérité DB)
	if s.supportClient == nil {
		return 0, paymentErrors.ErrorSupportServiceUnavailable
	}
	supportUser, err := s.supportClient.GetSupportUserByID(ctx, supportUserID)
	if err != nil {
		return 0, err
	}
	if !supportUser.IsActive {
		s.logger.Warn("inactive support user attempted manual payout", zap.String("supportUserID", supportUserID))
		return 0, paymentErrors.ErrorUnauthorized
	}
	if supportUser.Role != "admin" && supportUser.Role != "support" {
		s.logger.Warn("unauthorized role attempted manual payout",
			zap.String("supportUserID", supportUserID),
			zap.String("role", supportUser.Role),
		)
		return 0, paymentErrors.ErrorUnauthorized
	}

	// Exécuter le payout (écrit déjà "scheduled"/"processing"/"failed" dans l'historique)
	ctx = withHistoryOrigin(ctx, domain.HistoryInitiatedBySupport, domain.HistorySourceSupportManualPayout)
	payoutID, netAmount, err := s.processPayoutForTrip(ctx, tripID)
	if err != nil {
		return 0, err
	}

	// Aucun payout créé (aucun paiement released) — rien à auditer
	if payoutID == "" {
		return 0, nil
	}

	// Entrée d'historique avec les informations de l'agent support (source de vérité DB)
	s.writePayoutHistory(ctx, payoutID, "launched_by_support", "support",
		supportUser.UserID, supportUser.FirstName, supportUser.LastName,
		fmt.Sprintf("Paiement initié manuellement par %s %s", supportUser.FirstName, supportUser.LastName),
	)

	s.logger.Info("manual payout triggered by support",
		zap.String("tripID", tripID),
		zap.String("payoutID", payoutID),
		zap.String("supportUserID", supportUserID),
		zap.Int("netAmount", netAmount),
	)

	return netAmount, nil
}

// =============================================================================
// Helpers
// =============================================================================

// refundCreatedNotes décrit un remboursement à sa création (règle et ventilation des montants).
func refundCreatedNotes(refund *domain.Refund) string {
	notes := fmt.Sprintf("Motif %s, règle %s (%d%%) : %d XOF passager, %d XOF chauffeur, %d XOF plateforme",
		refund.RefundReason, refund.RefundRuleApplied, refund.RefundPercentage,
		refund.AmountToPassenger, refund.AmountToDriver, refund.AmountToPlatform,
	)
	if refund.AmountToPassenger == 0 {
		notes += " — aucun virement passager requis"
	}
	return notes
}

// paymentRefundedNotes décrit le passage d'un paiement en refunded.
func paymentRefundedNotes(refund *domain.Refund) string {
	return fmt.Sprintf("Remboursement %s (%s, %d XOF au passager)",
		refund.RefundReference, refund.RefundReason, refund.AmountToPassenger)
}

func formatOptionalTime(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// CheckDeletionEligibility vérifie si le chauffeur a un payout en attente.
func (s *paymentServiceImpl) CheckDeletionEligibility(ctx context.Context, userID string) (bool, string, error) {
	if userID == "" {
		return false, "", paymentErrors.ErrorInvalidInput
	}
	hasActive, err := s.payoutReadRepo.HasActivePayout(ctx, userID)
	if err != nil {
		return false, "", paymentErrors.ErrorInternalServer
	}
	if hasActive {
		return false, "chauffeur avec un versement en attente de traitement", nil
	}
	return true, "", nil
}

// AnonymizeUserData anonymise les données liées à l'utilisateur dans payment-service.
func (s *paymentServiceImpl) AnonymizeUserData(ctx context.Context, userID string, bookingIDs []string) error {
	if userID == "" {
		return paymentErrors.ErrorInvalidInput
	}
	if err := s.payoutWriteRepo.AnonymizeDriverRefs(ctx, userID); err != nil {
		return err
	}
	if len(bookingIDs) > 0 {
		if err := s.paymentWriteRepo.AnonymizePassengerPhoneNumbers(ctx, bookingIDs); err != nil {
			return err
		}
	}
	return nil
}
