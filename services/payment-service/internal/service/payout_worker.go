package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
)

const maxPayoutRetries = 3

// StartPayoutWorker lance le cron worker de payout en background.
func StartPayoutWorker(ctx context.Context, impl *paymentServiceImpl, intervalSeconds int, logger *zap.Logger) {
	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
	defer ticker.Stop()

	logger.Info("payout worker started", zap.Int("intervalSeconds", intervalSeconds))

	for {
		select {
		case <-ctx.Done():
			logger.Info("payout worker stopped")
			return
		case <-ticker.C:
			if err := impl.ProcessPayoutBatch(ctx); err != nil {
				logger.Error("payout batch processing failed", zap.Error(err))
			}
		}
	}
}

// ProcessPayoutBatch traite un batch de payouts.
func (s *paymentServiceImpl) ProcessPayoutBatch(ctx context.Context) error {
	if s.cache == nil || s.payoutReadRepo == nil || s.payoutWriteRepo == nil {
		return nil
	}

	// Acquérir le lock distribué
	acquired, err := s.cache.AcquirePayoutLock(ctx)
	if err != nil {
		return fmt.Errorf("acquire payout lock: %w", err)
	}
	if !acquired {
		s.logger.Debug("payout lock already held, skipping")
		return nil
	}
	defer s.cache.ReleasePayoutLock(ctx)

	// Trouver les trips éligibles
	tripIDs, err := s.payoutReadRepo.GetTripsReadyForPayout(ctx)
	if err != nil {
		return fmt.Errorf("get trips ready for payout: %w", err)
	}

	if len(tripIDs) == 0 {
		s.logger.Debug("no trips ready for payout")
		return nil
	}

	s.logger.Info("processing payout batch", zap.Int("tripsCount", len(tripIDs)))

	// Créer le batch record
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

	successCount := 0
	failCount := 0
	totalAmount := 0

	for _, tripID := range tripIDs {
		if err := s.processPayoutForTrip(ctx, tripID, &totalAmount); err != nil {
			s.logger.Error("payout for trip failed", zap.Error(err), zap.String("tripID", tripID))
			failCount++
		} else {
			successCount++
		}
	}

	// MAJ batch
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

// processPayoutForTrip traite le payout pour un trip donné.
func (s *paymentServiceImpl) processPayoutForTrip(ctx context.Context, tripID string, totalAmount *int) error {
	// Récupérer les paiements released du trip
	payments, err := s.payoutReadRepo.GetReleasedPaymentsForTrip(ctx, tripID)
	if err != nil {
		return err
	}

	if len(payments) == 0 {
		return nil
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
		return fmt.Errorf("get booking details: %w", err)
	}

	// Récupérer le withdraw_number du chauffeur
	user, err := s.userClient.GetUserByUserID(ctx, booking.DriverID)
	if err != nil {
		return fmt.Errorf("get driver info: %w", err)
	}

	if user.WithdrawNumber == "" {
		s.logger.Warn("driver has no withdraw number, skipping payout",
			zap.String("driverID", booking.DriverID),
			zap.String("tripID", tripID),
		)
		return fmt.Errorf("driver %s has no withdraw number", booking.DriverID)
	}

	// Créer le payout FedaPay
	fedapayPayout, err := s.fedapayClient.CreatePayout(
		netAmount, "moov_tg", user.WithdrawNumber,
		user.FirstName, user.Name,
	)
	if err != nil {
		return fmt.Errorf("fedapay create payout: %w", err)
	}

	// Sauvegarder le payout en DB
	payoutID := uuid.New().String()
	payoutRef := fmt.Sprintf("PO-%s", payoutID[:8])
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
		return err
	}

	// Démarrer le payout FedaPay
	_, err = s.fedapayClient.StartPayouts([]int{fedapayPayout.ID})
	if err != nil {
		s.logger.Error("fedapay start payout failed", zap.Error(err), zap.String("payoutID", payoutID))
		s.payoutWriteRepo.MarkPayoutFailed(ctx, payoutID, "fedapay start failed: "+err.Error()) //nolint:errcheck
		return err
	}

	// MAJ status du payout → processing
	s.payoutWriteRepo.UpdatePayoutStatus(ctx, payoutID, domain.PayoutStatusProcessing, strconv.Itoa(fedapayPayout.ID)) //nolint:errcheck

	// MAJ les paiements du trip → paidOut
	if err := s.payoutWriteRepo.MarkPaymentsAsPaidOut(ctx, tripID); err != nil {
		s.logger.Error("mark payments as paid out failed", zap.Error(err))
	}

	*totalAmount += netAmount

	s.logger.Info("payout processed for trip",
		zap.String("tripID", tripID),
		zap.String("payoutID", payoutID),
		zap.Int("netAmount", netAmount),
	)

	return nil
}
