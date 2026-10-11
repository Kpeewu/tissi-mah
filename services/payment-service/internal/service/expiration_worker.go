package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
)

const (
	// unsyncedHeldGracePeriod laisse à applyApproval le temps de terminer le rattachement
	// avant qu'un paiement held non rattaché ne soit repris par le worker.
	unsyncedHeldGracePeriod = 2 * time.Minute
	// unsyncedHeldBatchSize borne le nombre de paiements held repris par tick.
	unsyncedHeldBatchSize = 100
	// maxBookingSyncAttempts : reprises de rattachement échouées au-delà desquelles le passager
	// est remboursé intégralement.
	maxBookingSyncAttempts = 5
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

	expireErr := s.ExpireDuePayments(ctx, timeoutMinutes)
	if err := s.RetryUnsyncedHeldPayments(ctx); err != nil {
		s.logger.Error("unsynced held payments retry failed", zap.Error(err))
	}
	return expireErr
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
		if err := s.cancelPendingPayment(ctx, payment, reason, originExpiration); err != nil {
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

// RetryUnsyncedHeldPayments retente le rattachement des paiements held dont la confirmation n'a pas
// atteint booking-service (indisponibilité lors de l'approbation, notamment une approbation tardive
// sur une réservation paymentFailed que le sweeper booking-service ne traite pas). Chaque paiement
// est confirmé sur sa réservation ou remboursé ; sinon il est retenté au tick suivant, au plus
// maxBookingSyncAttempts fois, après quoi le rattachement est abandonné et le passager remboursé.
// Sans verrou distribué : l'appelant doit garantir l'exclusivité (cf. ProcessExpiredPayments).
func (s *paymentServiceImpl) RetryUnsyncedHeldPayments(ctx context.Context) error {
	completedBefore := time.Now().UTC().Add(-unsyncedHeldGracePeriod)
	payments, err := s.paymentReadRepo.GetUnsyncedHeldPayments(ctx, completedBefore, unsyncedHeldBatchSize)
	if err != nil {
		return fmt.Errorf("get unsynced held payments: %w", err)
	}

	if len(payments) == 0 {
		return nil
	}

	s.logger.Info("retrying unsynced held payments", zap.Int("count", len(payments)))

	confirmedCount := 0
	refundedCount := 0
	pendingCount := 0

	for _, payment := range payments {
		refunded, err := s.hasRefund(ctx, payment.PaymentID)
		if err != nil {
			pendingCount++
			s.logger.Error("unsynced held payment: refund lookup failed, will retry next tick",
				zap.String("paymentID", payment.PaymentID), zap.Error(err))
			continue
		}
		if refunded {
			// Remboursement déjà créé (passage en refunded manqué) : ne pas rembourser une seconde fois
			if err := s.paymentWriteRepo.MarkBookingSynced(ctx, payment.PaymentID); err != nil {
				s.logger.Warn("mark booking synced failed", zap.String("paymentID", payment.PaymentID), zap.Error(err))
			}
			refundedCount++
			continue
		}

		status, synced := s.retryUnsyncedHeldPayment(ctx, payment)
		switch {
		case synced:
			confirmedCount++
		case status == domain.PaymentStatusRefunded:
			refundedCount++
		default:
			pendingCount++
		}
	}

	s.logger.Info("unsynced held payments batch completed",
		zap.Int("confirmed", confirmedCount),
		zap.Int("refunded", refundedCount),
		zap.Int("pending", pendingCount),
	)

	return nil
}

// retryUnsyncedHeldPayment retente le rattachement d'un paiement held et retourne son statut résultant
// et synced=true si la réservation est confirmée. Chaque échec incrémente booking_sync_attempts ;
// au plafond, le passager est remboursé intégralement (retenté à chaque tick si le remboursement échoue).
func (s *paymentServiceImpl) retryUnsyncedHeldPayment(
	ctx context.Context, payment *domain.Payment,
) (domain.PaymentStatus, bool) {
	attempts := payment.BookingSyncAttempts
	if attempts < maxBookingSyncAttempts {
		status, outcome := s.attachHeldPayment(ctx, payment)
		if outcome == attachConfirmed {
			return status, true
		}
		if status == domain.PaymentStatusRefunded {
			return status, false
		}

		newAttempts, err := s.paymentWriteRepo.IncrementBookingSyncAttempts(ctx, payment.PaymentID)
		if err != nil {
			s.logger.Warn("increment booking sync attempts failed", zap.String("paymentID", payment.PaymentID), zap.Error(err))
			return status, false
		}
		attempts = newAttempts
		if attempts < maxBookingSyncAttempts {
			return status, false
		}
	}

	s.logger.Error("max booking sync attempts reached, refunding passenger",
		zap.String("paymentID", payment.PaymentID),
		zap.String("bookingID", payment.BookingID),
		zap.Int("attempts", attempts),
	)
	return s.refundUnconfirmablePayment(ctx, payment.PaymentID, domain.RefundReasonBookingUnavailable), false
}

// hasRefund indique si un remboursement existe déjà pour le paiement.
func (s *paymentServiceImpl) hasRefund(ctx context.Context, paymentID string) (bool, error) {
	if s.refundReadRepo == nil {
		return false, nil
	}
	if _, err := s.refundReadRepo.GetByPaymentID(ctx, paymentID); err != nil {
		if errors.Is(err, paymentErrors.ErrorRefundNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
