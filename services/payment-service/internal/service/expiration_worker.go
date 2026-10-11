package service

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// StartExpirationWorker lance le worker de nettoyage des paiements expirés.
func StartExpirationWorker(ctx context.Context, impl *paymentServiceImpl, intervalSeconds int, timeoutMinutes int, logger *zap.Logger) {
	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
	defer ticker.Stop()

	logger.Info("expiration worker started",
		zap.Int("intervalSeconds", intervalSeconds),
		zap.Int("timeoutMinutes", timeoutMinutes),
	)

	for {
		select {
		case <-ctx.Done():
			logger.Info("expiration worker stopped")
			return
		case <-ticker.C:
			if err := impl.ProcessExpiredPayments(ctx, timeoutMinutes); err != nil {
				logger.Error("expiration batch processing failed", zap.Error(err))
			}
		}
	}
}

// ProcessExpiredPayments traite les paiements pending dont la fenêtre de paiement est dépassée
// (expires_at = création de la réservation + PAYMENT_TIMEOUT_MINUTES).
// Une transaction FedaPay pending vivant 24h, c'est ce worker qui borne l'attente : pour chaque
// paiement, il vérifie le statut FedaPay, applique l'approbation si l'utilisateur a payé, sinon
// supprime la transaction (DELETE) et notifie booking-service pour libérer les places.
func (s *paymentServiceImpl) ProcessExpiredPayments(ctx context.Context, timeoutMinutes int) error {
	if s.cache == nil {
		return nil
	}

	// Acquérir le lock distribué
	acquired, err := s.cache.AcquireExpirationLock(ctx)
	if err != nil {
		return fmt.Errorf("acquire expiration lock: %w", err)
	}
	if !acquired {
		s.logger.Debug("expiration lock already held, skipping")
		return nil
	}
	defer s.cache.ReleaseExpirationLock(ctx)

	return s.ExpireDuePayments(ctx, timeoutMinutes)
}

// ExpireDuePayments traite un lot de paiements pending expirés (sans verrou distribué :
// l'appelant doit garantir l'exclusivité, cf. ProcessExpiredPayments).
func (s *paymentServiceImpl) ExpireDuePayments(ctx context.Context, timeoutMinutes int) error {
	payments, err := s.paymentReadRepo.GetExpiredPendingPayments(ctx, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("get expired pending payments: %w", err)
	}

	if len(payments) == 0 {
		return nil
	}

	s.logger.Info("processing expired payments", zap.Int("count", len(payments)))

	processedCount := 0
	errorCount := 0
	reason := fmt.Sprintf("Paiement non finalisé dans les %d minutes", timeoutMinutes)

	for _, payment := range payments {
		if err := s.cancelPendingPayment(ctx, payment, reason, true); err != nil {
			errorCount++
			s.logger.Error("expire payment failed, will retry next tick",
				zap.String("paymentID", payment.PaymentID),
				zap.String("bookingID", payment.BookingID),
				zap.Error(err),
			)
			continue
		}
		processedCount++
	}

	s.logger.Info("expiration batch completed",
		zap.Int("processed", processedCount),
		zap.Int("errors", errorCount),
	)

	return nil
}
