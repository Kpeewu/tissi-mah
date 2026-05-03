package service

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/Kpeewu/tissi-mah/pkg/notification"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
)

const maxPayoutRetries = 3

// StartPayoutWorker lance le cron worker de payout en background.
// La PaymentWindow restreint l'exécution des batches à une plage horaire
// quotidienne (cf. payment_window.go). Hors fenêtre, le ticker continue
// mais ProcessPayoutBatch n'est pas appelé. TriggerManualPayout (RPC admin)
// reste actif 24/7 puisqu'il n'utilise pas ce worker.
func StartPayoutWorker(ctx context.Context, impl *paymentServiceImpl, intervalSeconds int, window PaymentWindow, logger *zap.Logger) {
	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
	defer ticker.Stop()

	tz := "n/a"
	if window.Location != nil {
		tz = window.Location.String()
	}
	logger.Info("payout worker started",
		zap.Int("intervalSeconds", intervalSeconds),
		zap.Bool("windowEnabled", window.Enabled),
		zap.String("windowTz", tz),
		zap.Int("windowStart", window.StartHour),
		zap.Int("windowEnd", window.EndHour),
	)

	for {
		select {
		case <-ctx.Done():
			logger.Info("payout worker stopped")
			return
		case <-ticker.C:
			if !window.IsOpen(time.Now()) {
				logger.Debug("payout window closed, skipping tick")
				continue
			}
			if err := impl.ProcessPayoutBatch(ctx); err != nil {
				logger.Error("payout batch processing failed", zap.Error(err))
			}
		}
	}
}

// ProcessPayoutBatch traite un batch de payouts.
// Le lock global protège uniquement la phase de discovery, puis les trips sont traités en parallèle.
func (s *paymentServiceImpl) ProcessPayoutBatch(ctx context.Context) error {
	if s.cache == nil || s.payoutReadRepo == nil || s.payoutWriteRepo == nil {
		return nil
	}

	// 1. Acquérir le lock global pour la discovery uniquement
	acquired, err := s.cache.AcquirePayoutLock(ctx)
	if err != nil {
		return fmt.Errorf("acquire payout lock: %w", err)
	}
	if !acquired {
		s.logger.Debug("payout lock already held, skipping")
		return nil
	}

	// Trouver les trips éligibles
	tripIDs, err := s.payoutReadRepo.GetTripsReadyForPayout(ctx)

	// Relâcher le lock global immédiatement après la discovery
	s.cache.ReleasePayoutLock(ctx)

	if err != nil {
		return fmt.Errorf("get trips ready for payout: %w", err)
	}

	if len(tripIDs) == 0 {
		s.logger.Debug("no trips ready for payout")
		return nil
	}

	s.logger.Info("processing payout batch", zap.Int("tripsCount", len(tripIDs)))

	// 2. Créer le batch record
	batchID := uuid.New().String()
	now := time.Now().UTC()
	batch := &domain.PayoutBatch{
		BatchID:      batchID,
		Status:       "processing",
		TotalPayouts: len(tripIDs),
		StartedAt:    &now,
	}
	if err := s.payoutWriteRepo.CreateBatch(ctx, batch); err != nil {
		return err
	}

	// 3. Traiter les trips en parallèle avec sémaphore
	maxConcurrent := s.cfg.Worker.PayoutMaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 5
	}

	g, gCtx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, maxConcurrent)

	var mu sync.Mutex
	successCount := 0
	failCount := 0
	totalAmount := 0

	for _, tripID := range tripIDs {
		tripID := tripID
		g.Go(func() error {
			// Sémaphore pour limiter la concurrence
			sem <- struct{}{}
			defer func() { <-sem }()

			// Lock per-trip pour éviter les doublons
			tripAcquired, err := s.cache.AcquireTripPayoutLock(gCtx, tripID)
			if err != nil {
				s.logger.Warn("acquire trip payout lock failed", zap.Error(err), zap.String("tripID", tripID))
				mu.Lock()
				failCount++
				mu.Unlock()
				return nil
			}
			if !tripAcquired {
				s.logger.Debug("trip payout lock already held, skipping", zap.String("tripID", tripID))
				return nil
			}
			defer s.cache.ReleaseTripPayoutLock(gCtx, tripID)

			_, amount, err := s.processPayoutForTrip(gCtx, tripID)
			mu.Lock()
			if err != nil {
				s.logger.Error("payout for trip failed", zap.Error(err), zap.String("tripID", tripID))
				failCount++
			} else {
				successCount++
				totalAmount += amount
			}
			mu.Unlock()

			return nil
		})
	}
	g.Wait()

	// 4. MAJ batch
	completedAt := time.Now().UTC()
	batch.Status = "completed"
	batch.SuccessfulCount = successCount
	batch.FailedCount = failCount
	batch.TotalAmount = totalAmount
	batch.CompletedAt = &completedAt
	s.payoutWriteRepo.UpdateBatch(ctx, batch) //nolint:errcheck

	s.logger.Info("payout batch completed",
		zap.String("batchID", batchID),
		zap.Int("success", successCount),
		zap.Int("failed", failCount),
		zap.Int("totalAmount", totalAmount),
	)

	return nil
}

// processPayoutForTrip traite le payout pour un trip donné et retourne le payoutID et le montant net versé.
// Écrit les entrées d'historique (scheduled, processing ou failed) avec initiated_by="system".
func (s *paymentServiceImpl) processPayoutForTrip(ctx context.Context, tripID string) (string, int, error) {
	// Récupérer les paiements released du trip
	payments, err := s.payoutReadRepo.GetReleasedPaymentsForTrip(ctx, tripID)
	if err != nil {
		return "", 0, err
	}

	if len(payments) == 0 {
		return "", 0, nil
	}

	// Calculer le montant total à verser au chauffeur
	grossAmount := 0
	for _, p := range payments {
		grossAmount += p.Amount
	}

	// Appliquer la commission plateforme
	platformFee := grossAmount * s.cfg.Payout.PlatformFeePercent / 100
	netAmount := grossAmount - platformFee

	// Récupérer le booking pour trouver le driverID
	booking, err := s.bookingClient.GetBookingDetails(ctx, payments[0].BookingID)
	if err != nil {
		return "", 0, fmt.Errorf("get booking details: %w", err)
	}

	// Récupérer le withdraw_number du chauffeur
	user, err := s.userClient.GetUserByUserID(ctx, booking.DriverID)
	if err != nil {
		return "", 0, fmt.Errorf("get driver info: %w", err)
	}

	if user.WithdrawNumber == "" {
		s.logger.Warn("driver has no withdraw number, skipping payout",
			zap.String("driverID", booking.DriverID),
			zap.String("tripID", tripID),
		)
		return "", 0, fmt.Errorf("driver %s has no withdraw number", booking.DriverID)
	}

	// Créer le payout FedaPay
	fedapayPayout, err := s.fedapayClient.CreatePayout(
		netAmount, "togocel", user.WithdrawNumber,
		user.FirstName, user.Name,
	)
	if err != nil {
		return "", 0, fmt.Errorf("fedapay create payout: %w", err)
	}

	// Sauvegarder le payout en DB
	payoutID := uuid.New().String()
	payoutRef := fmt.Sprintf("PYO-%s-%s", time.Now().UTC().Format("20060102"), generateAlphanumeric(6))
	scheduledAt := time.Now().UTC()

	payout := &domain.Payout{
		PayoutID:                 payoutID,
		PayoutReference:          payoutRef,
		DriverID:                 booking.DriverID,
		TripID:                   tripID,
		GrossAmount:              grossAmount,
		PlatformFee:              platformFee,
		NetAmount:                netAmount,
		PayoutMethod:             "mobileMoney",
		PayoutDestination:        user.WithdrawNumber,
		DestinationName:          user.FirstName + " " + user.Name,
		Status:                   domain.PayoutStatusScheduled,
		ScheduledAt:              &scheduledAt,
		PaymentProvider:          "fedapay",
		PaymentProviderReference: strconv.Itoa(fedapayPayout.ID),
	}

	if err := s.payoutWriteRepo.CreatePayout(ctx, payout); err != nil {
		return "", 0, err
	}

	// Historique : payout planifié
	s.writeHistory(ctx, payoutID, "scheduled", "system", "", "", "", "Payout planifié")

	// Démarrer le payout FedaPay
	_, err = s.fedapayClient.StartPayouts([]int{fedapayPayout.ID})
	if err != nil {
		s.logger.Error("fedapay start payout failed", zap.Error(err), zap.String("payoutID", payoutID))
		s.payoutWriteRepo.MarkPayoutFailed(ctx, payoutID, "fedapay start failed: "+err.Error()) //nolint:errcheck
		s.writeHistory(ctx, payoutID, "failed", "system", "", "", "", "FedaPay start failed: "+err.Error())
		return "", 0, err
	}

	// MAJ status du payout → processing
	s.payoutWriteRepo.UpdatePayoutStatus(ctx, payoutID, domain.PayoutStatusProcessing, strconv.Itoa(fedapayPayout.ID)) //nolint:errcheck

	// Historique : payout en cours de traitement
	s.writeHistory(ctx, payoutID, "processing", "system", "", "", "", "FedaPay payout démarré")

	// MAJ les paiements du trip → paidOut
	if err := s.payoutWriteRepo.MarkPaymentsAsPaidOut(ctx, tripID); err != nil {
		s.logger.Error("mark payments as paid out failed", zap.Error(err))
	}

	// Notifier le conducteur (non bloquant)
	if s.notifRedis != nil {
		if pubErr := notification.Publish(ctx, s.notifRedis, notification.Event{
			EventType:     notification.DriverPaymentLaunched,
			UserID:        booking.DriverID,
			ReferenceID:   payoutID,
			ReferenceType: notification.RefPayment,
			Payload: map[string]string{
				"amount": fmt.Sprintf("%d", netAmount),
			},
		}); pubErr != nil {
			s.logger.Error("failed to publish DRIVER_PAYMENT_LAUNCHED notification", zap.Error(pubErr))
		}
	}

	s.logger.Info("payout processed for trip",
		zap.String("tripID", tripID),
		zap.String("payoutID", payoutID),
		zap.Int("netAmount", netAmount),
	)

	return payoutID, netAmount, nil
}

// writeHistory écrit une entrée dans payout_status_history de manière non bloquante.
func (s *paymentServiceImpl) writeHistory(ctx context.Context, payoutID, status, initiatedBy, supportUserID, supportFirstName, supportLastName, notes string) {
	if s.payoutHistoryRepo == nil {
		return
	}
	entry := &domain.PayoutStatusHistory{
		HistoryID:        uuid.New().String(),
		PayoutID:         payoutID,
		Status:           status,
		InitiatedBy:      initiatedBy,
		SupportUserID:    supportUserID,
		SupportFirstName: supportFirstName,
		SupportLastName:  supportLastName,
		OccurredAt:       time.Now().UTC(),
		Notes:            notes,
	}
	if err := s.payoutHistoryRepo.CreateHistoryEntry(ctx, entry); err != nil {
		s.logger.Error("write payout history failed",
			zap.Error(err),
			zap.String("payoutID", payoutID),
			zap.String("status", status),
		)
	}
}
