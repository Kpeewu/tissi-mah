package service

import (
	"context"
	"fmt"
	"strconv"
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

// ProcessExpiredPayments traite les paiements pending expirés.
// Pour chaque paiement : vérifie le statut FedaPay, annule si toujours pending,
// puis notifie booking-service pour restaurer les places.
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

	// Récupérer les paiements pending expirés
	timeout := time.Duration(timeoutMinutes) * time.Minute
	payments, err := s.paymentReadRepo.GetExpiredPendingPayments(ctx, timeout)
	if err != nil {
		return fmt.Errorf("get expired pending payments: %w", err)
	}

	if len(payments) == 0 {
		return nil
	}

	s.logger.Info("processing expired payments", zap.Int("count", len(payments)))

	expiredCount := 0
	skippedCount := 0

	for _, payment := range payments {
		externalID, err := strconv.Atoi(payment.ExternalTransactionID)
		if err != nil {
			s.logger.Error("invalid external transaction ID",
				zap.String("paymentID", payment.PaymentID),
				zap.String("externalID", payment.ExternalTransactionID),
			)
			continue
		}

		// Vérifier le statut réel auprès de FedaPay
		transaction, err := s.fedapayClient.GetTransaction(externalID)
		if err != nil {
			s.logger.Error("get transaction from FedaPay failed",
				zap.String("paymentID", payment.PaymentID),
				zap.Error(err),
			)
			continue
		}

		// Si la transaction n'est plus pending côté FedaPay, on laisse le webhook gérer
		if transaction.Status != "pending" {
			s.logger.Debug("transaction no longer pending on FedaPay, skipping",
				zap.String("paymentID", payment.PaymentID),
				zap.String("fedapayStatus", transaction.Status),
			)
			skippedCount++
			continue
		}

		// Annuler la transaction sur FedaPay
		if err := s.fedapayClient.CancelTransaction(externalID); err != nil {
			s.logger.Error("cancel transaction on FedaPay failed",
				zap.String("paymentID", payment.PaymentID),
				zap.Error(err),
			)
			continue
		}

		// Marquer le paiement comme échoué
		reason := fmt.Sprintf("Paiement expiré après %d minutes sans réponse", timeoutMinutes)
		if err := s.paymentWriteRepo.MarkPaymentFailed(ctx, payment.PaymentID, reason); err != nil {
			s.logger.Error("mark payment failed failed",
				zap.String("paymentID", payment.PaymentID),
				zap.Error(err),
			)
			continue
		}

		// Invalider le cache
		if s.cache != nil {
			s.cache.InvalidatePaymentByBookingID(ctx, payment.BookingID)
		}

		// Notifier booking-service pour restaurer les places
		if err := s.bookingClient.FailPayment(ctx, payment.BookingID, reason); err != nil {
			s.logger.Error("fail payment to booking-service failed",
				zap.String("paymentID", payment.PaymentID),
				zap.String("bookingID", payment.BookingID),
				zap.Error(err),
			)
		}

		expiredCount++
		s.logger.Info("payment expired and cancelled",
			zap.String("paymentID", payment.PaymentID),
			zap.String("bookingID", payment.BookingID),
		)
	}

	s.logger.Info("expiration batch completed",
		zap.Int("expired", expiredCount),
		zap.Int("skipped", skippedCount),
	)

	return nil
}
