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
		if err := s.refundWriteRepo.MarkRefundFailed(ctx, refund.RefundID, reason, false); err != nil {
			s.logger.Error("mark refund failed (max retries) failed", zap.Error(err))
		}
		return nil
	}

	// 3. Récupérer les infos du paiement (numéro passager)
	payment, err := s.paymentReadRepo.GetByID(ctx, refund.PaymentID)
	if err != nil {
		s.logger.Error("get payment for refund failed", zap.Error(err), zap.String("paymentID", refund.PaymentID))
		s.refundWriteRepo.MarkRefundFailed(ctx, refund.RefundID, "payment not found: "+err.Error(), true) //nolint:errcheck
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
		s.refundWriteRepo.MarkRefundFailed(ctx, refund.RefundID, "missing passenger phone number", false) //nolint:errcheck
		return nil
	}

	// 4. Récupérer l'identité du passager (prénom/nom) via user-service
	booking, err := s.bookingClient.GetBookingDetails(ctx, refund.BookingID)
	if err != nil {
		s.logger.Error("get booking for refund failed", zap.Error(err), zap.String("bookingID", refund.BookingID))
		s.refundWriteRepo.MarkRefundFailed(ctx, refund.RefundID, "booking lookup failed: "+err.Error(), true) //nolint:errcheck
		return nil
	}

	user, err := s.userClient.GetUserByUserID(ctx, booking.PassengerID)
	if err != nil {
		s.logger.Error("get user for refund failed", zap.Error(err), zap.String("passengerID", booking.PassengerID))
		s.refundWriteRepo.MarkRefundFailed(ctx, refund.RefundID, "user lookup failed: "+err.Error(), true) //nolint:errcheck
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

	// 6. Appel FedaPay — même endpoint que le payout chauffeur, numéro passager cette fois
	fedapayPayout, err := s.fedapayClient.CreatePayout(
		refund.AmountToPassenger, "togocel", phoneNumber,
		user.FirstName, user.Name,
	)
	if err != nil {
		s.logger.Error("fedapay refund payout failed", zap.Error(err), zap.String("refundID", refund.RefundID))
		s.refundWriteRepo.MarkRefundFailed(ctx, refund.RefundID, "fedapay: "+err.Error(), true) //nolint:errcheck
		return nil
	}

	// 7. Démarrer le payout FedaPay (comme pour les chauffeurs)
	if _, err := s.fedapayClient.StartPayouts([]int{fedapayPayout.ID}); err != nil {
		s.logger.Error("fedapay start refund payout failed", zap.Error(err), zap.String("refundID", refund.RefundID))
		s.refundWriteRepo.MarkRefundFailed(ctx, refund.RefundID, "fedapay start: "+err.Error(), true) //nolint:errcheck
		return nil
	}

	// 8. Finaliser
	providerRef := strconv.Itoa(fedapayPayout.ID)
	if err := s.refundWriteRepo.MarkRefundCompleted(ctx, refund.RefundID, providerRef); err != nil {
		s.logger.Error("mark refund completed failed", zap.Error(err))
		return nil
	}

	s.logger.Info("refund completed",
		zap.String("refundID", refund.RefundID),
		zap.String("providerReference", providerRef),
		zap.Int("amountToPassenger", refund.AmountToPassenger),
	)

	// 9. Notifier le passager (non bloquant)
	if s.notifRedis != nil {
		if pubErr := notification.Publish(ctx, s.notifRedis, notification.Event{
			EventType:     notification.RefundCompleted,
			UserID:        booking.PassengerID,
			ReferenceID:   refund.RefundID,
			ReferenceType: notification.RefRefund,
			Payload: map[string]string{
				"amount":                     fmt.Sprintf("%d", refund.AmountToPassenger),
				"currency":                   "XOF",
				"payment_provider_reference": providerRef,
			},
		}); pubErr != nil {
			s.logger.Error("failed to publish REFUND_COMPLETED notification", zap.Error(pubErr))
		}
	}

	return nil
}
