package service

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/Kpeewu/tissi-mah/pkg/notification"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
)

// StartRefundWorker lance le worker qui traite les refunds en attente en fond.
// Chaque tick appelle ProcessPendingRefunds qui rembourse les passagers via FedaPay.
// La PaymentWindow restreint l'exécution des batches à une plage horaire
// quotidienne (cf. payment_window.go). Hors fenêtre, le ticker continue
// mais ProcessPendingRefunds n'est pas appelé.
func StartRefundWorker(ctx context.Context, impl *paymentServiceImpl, intervalSeconds int, window PaymentWindow, logger *zap.Logger) {
	if intervalSeconds <= 0 {
		intervalSeconds = 60
	}
	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
	defer ticker.Stop()

	tz := "n/a"
	if window.Location != nil {
		tz = window.Location.String()
	}
	logger.Info("refund worker started",
		zap.Int("intervalSeconds", intervalSeconds),
		zap.Bool("windowEnabled", window.Enabled),
		zap.String("windowTz", tz),
		zap.Int("windowStart", window.StartHour),
		zap.Int("windowEnd", window.EndHour),
	)

	for {
		select {
		case <-ctx.Done():
			logger.Info("refund worker stopped")
			return
		case <-ticker.C:
			if !window.IsOpen(time.Now()) {
				logger.Debug("refund window closed, skipping tick")
				continue
			}
			if err := impl.ProcessPendingRefunds(ctx); err != nil {
				logger.Error("refund batch processing failed", zap.Error(err))
			}
			if err := impl.ProcessPendingDriverRefundPayouts(ctx); err != nil {
				logger.Error("driver refund payout batch processing failed", zap.Error(err))
			}
		}
	}
}

// ProcessPendingRefunds traite un batch de refunds 'pending' avec amount_to_passenger > 0.
// Chaque refund est protégé par un lock Redis per-refund pour éviter les doubles transferts
// si plusieurs instances du worker tournent en parallèle.
func (s *paymentServiceImpl) ProcessPendingRefunds(ctx context.Context) error {
	if s.refundReadRepo == nil || s.refundWriteRepo == nil || s.fedapayClient == nil {
		return nil
	}
	ctx = withHistoryOrigin(ctx, domain.HistoryInitiatedBySystem, domain.HistorySourceRefundWorker)

	batchSize := s.cfg.RefundWorker.BatchSize
	if batchSize <= 0 {
		batchSize = 20
	}

	refunds, err := s.refundReadRepo.GetPendingRefundsForPayout(ctx, batchSize)
	if err != nil {
		return fmt.Errorf("get pending refunds: %w", err)
	}
	if len(refunds) == 0 {
		s.logger.Debug("no pending refunds to process")
		return nil
	}

	s.logger.Info("processing refund batch", zap.Int("count", len(refunds)))

	maxConcurrent := s.cfg.Worker.RefundMaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 5
	}

	g, gCtx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, maxConcurrent)

	var mu sync.Mutex
	successCount := 0
	failCount := 0

	for _, refund := range refunds {
		refund := refund
		g.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := s.processRefund(gCtx, refund); err != nil {
				mu.Lock()
				failCount++
				mu.Unlock()
				s.logger.Error("process refund failed", zap.Error(err), zap.String("refundID", refund.RefundID))
				return nil
			}
			mu.Lock()
			successCount++
			mu.Unlock()
			return nil
		})
	}
	g.Wait()

	s.logger.Info("refund batch completed",
		zap.Int("success", successCount),
		zap.Int("failed", failCount),
		zap.Int("total", len(refunds)),
	)

	return nil
}

// processRefund gère un seul refund : acquiert un lock Redis, appelle FedaPay, met à jour le refund.
// Retourne une erreur uniquement si le flux doit être interrompu ; les erreurs FedaPay et les erreurs
// de données sont absorbées (le refund est marqué failed dans tous les cas).
func (s *paymentServiceImpl) processRefund(ctx context.Context, refund *domain.Refund) error {
	// 1. Lock per-refund (protège contre les doubles transferts si multiples workers)
	if s.cache != nil {
		acquired, err := s.cache.AcquireRefundLock(ctx, refund.RefundID)
		if err != nil {
			s.logger.Warn("acquire refund lock failed", zap.Error(err), zap.String("refundID", refund.RefundID))
			return nil
		}
		if !acquired {
			s.logger.Debug("refund lock already held, skipping", zap.String("refundID", refund.RefundID))
			return nil
		}
		defer s.cache.ReleaseRefundLock(ctx, refund.RefundID)
	}

	// 2. Vérifier le plafond de retry avant toute opération coûteuse
	maxRetries := int16(s.cfg.RefundWorker.MaxRetries)
	if maxRetries <= 0 {
		maxRetries = 3
	}
	if refund.RetryCount >= maxRetries {
		s.logger.Error("refund max retries exceeded, manual intervention required",
			zap.String("refundID", refund.RefundID),
			zap.Int16("retryCount", refund.RetryCount),
		)
		// Marquer définitivement failed (sans incrémenter) pour sortir de la file pending.
		reason := fmt.Sprintf("max retries (%d) exceeded", maxRetries)
		if err := s.markRefundFailed(ctx, refund, reason, false); err != nil {
			s.logger.Error("mark refund failed (max retries) failed", zap.Error(err))
			return nil
		}
		s.publishRefundFailed(ctx, refund)
		return nil
	}

	// 3. Récupérer les infos du paiement (numéro passager)
	payment, err := s.paymentReadRepo.GetByID(ctx, refund.PaymentID)
	if err != nil {
		s.logger.Error("get payment for refund failed", zap.Error(err), zap.String("paymentID", refund.PaymentID))
		s.markRefundFailed(ctx, refund, "payment not found: "+err.Error(), true) //nolint:errcheck
		return nil
	}

	phoneNumber := payment.PassengerPhoneNumber
	if phoneNumber == "" && refund.PayoutDestination != "" {
		// Fallback pour les refunds créés avec l'ancien code (pré-migration).
		phoneNumber = refund.PayoutDestination
	}
	if phoneNumber == "" {
		s.logger.Error("refund has no passenger phone number, unrecoverable",
			zap.String("refundID", refund.RefundID),
			zap.String("paymentID", refund.PaymentID),
		)
		err := s.markRefundFailed(ctx, refund, "missing passenger phone number", false)
		if err != nil {
			s.logger.Error("mark refund failed (missing phone) failed", zap.Error(err))
			return nil
		}
		s.publishRefundFailed(ctx, refund)
		return nil
	}

	// 4. Récupérer l'identité du passager (prénom/nom) via user-service
	booking, err := s.bookingClient.GetBookingDetails(ctx, refund.BookingID)
	if err != nil {
		s.logger.Error("get booking for refund failed", zap.Error(err), zap.String("bookingID", refund.BookingID))
		s.markRefundFailed(ctx, refund, "booking lookup failed: "+err.Error(), true) //nolint:errcheck
		return nil
	}

	user, err := s.userClient.GetUserByUserID(ctx, booking.PassengerID)
	if err != nil {
		s.logger.Error("get user for refund failed", zap.Error(err), zap.String("passengerID", booking.PassengerID))
		s.markRefundFailed(ctx, refund, "user lookup failed: "+err.Error(), true) //nolint:errcheck
		return nil
	}

	// 5. Passer en processing (verrouillage optimiste)
	if err := s.refundWriteRepo.MarkRefundProcessing(ctx, refund.RefundID); err != nil {
		if err == paymentErrors.ErrorRefundAlreadyProcessed {
			s.logger.Debug("refund already being processed, skipping", zap.String("refundID", refund.RefundID))
			return nil
		}
		s.logger.Error("mark refund processing failed", zap.Error(err))
		return nil
	}
	s.writeRefundHistory(ctx, refund.RefundID, string(domain.RefundStatusProcessing), refund.RetryCount,
		fmt.Sprintf("Virement FedaPay de %d XOF en cours", refund.AmountToPassenger))

	// 6. Appel FedaPay — même endpoint que le payout chauffeur, numéro passager cette fois
	fedapayPayout, err := s.fedapayClient.CreatePayout(
		refund.AmountToPassenger, "togocel", phoneNumber,
		user.FirstName, user.Name,
	)
	if err != nil {
		s.logger.Error("fedapay refund payout failed", zap.Error(err), zap.String("refundID", refund.RefundID))
		s.markRefundFailed(ctx, refund, "fedapay: "+err.Error(), true) //nolint:errcheck
		return nil
	}

	// 7. Enregistrer la référence FedaPay avant le démarrage : aucun virement ne part sans être traçable
	// (un payout créé mais jamais démarré n'envoie rien).
	providerRef := strconv.Itoa(fedapayPayout.ID)
	if err := s.refundWriteRepo.SetRefundProviderReference(ctx, refund.RefundID, providerRef); err != nil {
		s.logger.Error("set refund provider reference failed", zap.Error(err), zap.String("refundID", refund.RefundID))
		s.markRefundFailed(ctx, refund, "store fedapay reference: "+err.Error(), true) //nolint:errcheck
		return nil
	}

	// 8. Démarrer le payout FedaPay (comme pour les chauffeurs)
	if _, err := s.fedapayClient.StartPayouts([]int{fedapayPayout.ID}); err != nil {
		s.logger.Error("fedapay start refund payout failed", zap.Error(err), zap.String("refundID", refund.RefundID))
		s.markRefundFailed(ctx, refund, "fedapay start: "+err.Error(), true) //nolint:errcheck
		return nil
	}

	// 9. Le refund reste processing jusqu'à la confirmation FedaPay (webhook payout.* ou réconciliation),
	// qui le passe en completed et notifie le passager (cf. payout_confirmation.go).
	s.writeRefundHistory(ctx, refund.RefundID, domain.RefundEventPayoutStarted, refund.RetryCount,
		"Virement FedaPay "+providerRef+" démarré, en attente de confirmation")

	s.logger.Info("refund payout started",
		zap.String("refundID", refund.RefundID),
		zap.String("providerReference", providerRef),
		zap.Int("amountToPassenger", refund.AmountToPassenger),
	)

	return nil
}

// ProcessPendingDriverRefundPayouts traite un batch de payouts chauffeur issus de remboursements
// (driver_payout_status = 'pending', amount_to_driver > 0). Chaque entrée est protégée par un
// lock Redis per-refund distinct du lock passager.
func (s *paymentServiceImpl) ProcessPendingDriverRefundPayouts(ctx context.Context) error {
	if s.refundReadRepo == nil || s.refundWriteRepo == nil || s.fedapayClient == nil {
		return nil
	}
	ctx = withHistoryOrigin(ctx, domain.HistoryInitiatedBySystem, domain.HistorySourceRefundWorker)

	batchSize := s.cfg.RefundWorker.BatchSize
	if batchSize <= 0 {
		batchSize = 20
	}

	refunds, err := s.refundReadRepo.GetPendingDriverRefundPayouts(ctx, batchSize)
	if err != nil {
		return fmt.Errorf("get pending driver refund payouts: %w", err)
	}
	if len(refunds) == 0 {
		s.logger.Debug("no pending driver refund payouts to process")
		return nil
	}

	s.logger.Info("processing driver refund payout batch", zap.Int("count", len(refunds)))

	maxConcurrent := s.cfg.Worker.RefundMaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 5
	}

	g, gCtx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, maxConcurrent)

	var mu sync.Mutex
	successCount := 0
	failCount := 0

	for _, refund := range refunds {
		refund := refund
		g.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := s.processDriverRefundPayout(gCtx, refund); err != nil {
				mu.Lock()
				failCount++
				mu.Unlock()
				s.logger.Error("process driver refund payout failed", zap.Error(err), zap.String("refundID", refund.RefundID))
				return nil
			}
			mu.Lock()
			successCount++
			mu.Unlock()
			return nil
		})
	}
	g.Wait()

	s.logger.Info("driver refund payout batch completed",
		zap.Int("success", successCount),
		zap.Int("failed", failCount),
		zap.Int("total", len(refunds)),
	)

	return nil
}

// processDriverRefundPayout gère un seul payout chauffeur issu d'un remboursement.
func (s *paymentServiceImpl) processDriverRefundPayout(ctx context.Context, refund *domain.Refund) error {
	// 1. Lock per-refund (clé distincte du lock passager)
	if s.cache != nil {
		acquired, err := s.cache.AcquireDriverRefundPayoutLock(ctx, refund.RefundID)
		if err != nil {
			s.logger.Warn("acquire driver refund payout lock failed", zap.Error(err), zap.String("refundID", refund.RefundID))
			return nil
		}
		if !acquired {
			s.logger.Debug("driver refund payout lock already held, skipping", zap.String("refundID", refund.RefundID))
			return nil
		}
		defer s.cache.ReleaseDriverRefundPayoutLock(ctx, refund.RefundID)
	}

	// 2. Vérifier le plafond de retry
	maxRetries := int16(s.cfg.RefundWorker.MaxRetries)
	if maxRetries <= 0 {
		maxRetries = 3
	}
	if refund.DriverPayoutRetryCount >= maxRetries {
		s.logger.Error("driver refund payout max retries exceeded, manual intervention required",
			zap.String("refundID", refund.RefundID),
			zap.Int16("retryCount", refund.DriverPayoutRetryCount),
		)
		reason := fmt.Sprintf("max retries (%d) exceeded", maxRetries)
		if err := s.refundWriteRepo.MarkDriverPayoutFailed(ctx, refund.RefundID, reason, false); err != nil {
			s.logger.Error("mark driver payout failed (max retries) failed", zap.Error(err))
		}
		s.writeRefundHistory(ctx, refund.RefundID, string(domain.RefundStatusFailed)+"_driver", refund.DriverPayoutRetryCount, reason)
		return nil
	}

	// 3. Récupérer le booking pour trouver le driverID
	booking, err := s.bookingClient.GetBookingDetails(ctx, refund.BookingID)
	if err != nil {
		s.logger.Error("get booking for driver refund payout failed", zap.Error(err), zap.String("bookingID", refund.BookingID))
		s.refundWriteRepo.MarkDriverPayoutFailed(ctx, refund.RefundID, "booking lookup failed: "+err.Error(), true) //nolint:errcheck
		return nil
	}

	// 4. Récupérer le withdraw_number du chauffeur
	user, err := s.userClient.GetUserByUserID(ctx, booking.DriverID)
	if err != nil {
		s.logger.Error("get driver for driver refund payout failed", zap.Error(err), zap.String("driverID", booking.DriverID))
		s.refundWriteRepo.MarkDriverPayoutFailed(ctx, refund.RefundID, "user lookup failed: "+err.Error(), true) //nolint:errcheck
		return nil
	}

	if user.WithdrawNumber == "" {
		s.logger.Error("driver has no withdraw number, unrecoverable",
			zap.String("refundID", refund.RefundID),
			zap.String("driverID", booking.DriverID),
		)
		if err := s.refundWriteRepo.MarkDriverPayoutFailed(ctx, refund.RefundID, "missing driver withdraw number", false); err != nil {
			s.logger.Error("mark driver payout failed (missing withdraw number) failed", zap.Error(err))
		}
		return nil
	}

	// 5. Passer en processing (verrouillage optimiste)
	if err := s.refundWriteRepo.MarkDriverPayoutProcessing(ctx, refund.RefundID); err != nil {
		if err == paymentErrors.ErrorRefundAlreadyProcessed {
			s.logger.Debug("driver refund payout already being processed, skipping", zap.String("refundID", refund.RefundID))
			return nil
		}
		s.logger.Error("mark driver payout processing failed", zap.Error(err))
		return nil
	}
	s.writeRefundHistory(ctx, refund.RefundID, domain.RefundEventDriverPayoutStarted, refund.DriverPayoutRetryCount,
		fmt.Sprintf("Virement FedaPay de %d XOF chauffeur en cours", refund.AmountToDriver))

	// 6. Appel FedaPay
	fedapayPayout, err := s.fedapayClient.CreatePayout(
		refund.AmountToDriver, "togocel", user.WithdrawNumber,
		user.FirstName, user.Name,
	)
	if err != nil {
		s.logger.Error("fedapay driver refund payout failed", zap.Error(err), zap.String("refundID", refund.RefundID))
		s.refundWriteRepo.MarkDriverPayoutFailed(ctx, refund.RefundID, "fedapay: "+err.Error(), true) //nolint:errcheck
		return nil
	}

	// 7. Enregistrer la référence FedaPay avant le démarrage
	providerRef := strconv.Itoa(fedapayPayout.ID)
	if err := s.refundWriteRepo.SetDriverPayoutProviderReference(ctx, refund.RefundID, providerRef); err != nil {
		s.logger.Error("set driver payout provider reference failed", zap.Error(err), zap.String("refundID", refund.RefundID))
		s.refundWriteRepo.MarkDriverPayoutFailed(ctx, refund.RefundID, "store fedapay reference: "+err.Error(), true) //nolint:errcheck
		return nil
	}

	// 8. Démarrer le payout FedaPay
	if _, err := s.fedapayClient.StartPayouts([]int{fedapayPayout.ID}); err != nil {
		s.logger.Error("fedapay start driver refund payout failed", zap.Error(err), zap.String("refundID", refund.RefundID))
		s.refundWriteRepo.MarkDriverPayoutFailed(ctx, refund.RefundID, "fedapay start: "+err.Error(), true) //nolint:errcheck
		return nil
	}

	// 9. Reste processing jusqu'à la confirmation FedaPay (webhook ou réconciliation).
	s.writeRefundHistory(ctx, refund.RefundID, domain.RefundEventDriverPayoutStarted, refund.DriverPayoutRetryCount,
		"Virement FedaPay "+providerRef+" démarré, en attente de confirmation")

	s.logger.Info("driver refund payout started",
		zap.String("refundID", refund.RefundID),
		zap.String("driverID", booking.DriverID),
		zap.String("providerReference", providerRef),
		zap.Int("amountToDriver", refund.AmountToDriver),
	)

	return nil
}

// markRefundFailed enregistre l'échec d'un refund puis l'historise : retry_scheduled si l'échec est
// récupérable (le refund repasse pending, retry_count incrémenté), failed sinon.
func (s *paymentServiceImpl) markRefundFailed(ctx context.Context, refund *domain.Refund, reason string, incrementRetry bool) error {
	if err := s.refundWriteRepo.MarkRefundFailed(ctx, refund.RefundID, reason, incrementRetry); err != nil {
		return err
	}
	if incrementRetry {
		s.writeRefundHistory(ctx, refund.RefundID, domain.RefundEventRetryScheduled, refund.RetryCount+1, reason)
	} else {
		s.writeRefundHistory(ctx, refund.RefundID, string(domain.RefundStatusFailed), refund.RetryCount, reason)
	}
	return nil
}

// publishRefundFailed notifie le passager (non bloquant) que son remboursement a échoué définitivement
// et qu'il sera traité manuellement par le support.
func (s *paymentServiceImpl) publishRefundFailed(ctx context.Context, refund *domain.Refund) {
	if s.notifRedis == nil {
		return
	}
	booking, err := s.bookingClient.GetBookingDetails(ctx, refund.BookingID)
	if err != nil {
		s.logger.Warn("get booking for REFUND_FAILED notification failed",
			zap.String("refundID", refund.RefundID), zap.Error(err))
		return
	}
	if pubErr := notification.Publish(ctx, s.notifRedis, notification.Event{
		EventType:     notification.RefundFailed,
		UserID:        booking.PassengerID,
		ReferenceID:   refund.RefundID,
		ReferenceType: notification.RefRefund,
		Payload: map[string]string{
			"amount":   fmt.Sprintf("%d", refund.AmountToPassenger),
			"currency": "XOF",
			"reason":   string(refund.RefundReason),
		},
	}); pubErr != nil {
		s.logger.Error("failed to publish REFUND_FAILED notification", zap.Error(pubErr))
	}
}
