package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/fedapay"
)

const (
	// payoutReconcileDelay laisse au webhook FedaPay le temps d'arriver avant de vérifier un virement.
	payoutReconcileDelay = 10 * time.Minute
	// payoutReconcileBatchSize borne le nombre de refunds et de payouts vérifiés par tick.
	payoutReconcileBatchSize = 50
)

// StartPayoutReconciler lance le worker qui vérifie auprès de FedaPay les virements démarrés mais
// non confirmés (webhook perdu). Il n'est pas soumis à la PaymentWindow : GetPayout est une lecture
// sans effet financier, et un virement démarré dans la fenêtre peut aboutir après sa fermeture.
func StartPayoutReconciler(ctx context.Context, impl *paymentServiceImpl, intervalSeconds int, logger *zap.Logger) {
	if intervalSeconds <= 0 {
		intervalSeconds = 300
	}
	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
	defer ticker.Stop()

	logger.Info("payout reconciler started", zap.Int("intervalSeconds", intervalSeconds))

	for {
		select {
		case <-ctx.Done():
			logger.Info("payout reconciler stopped")
			return
		case <-ticker.C:
			if err := impl.ReconcileProviderPayouts(ctx); err != nil {
				logger.Error("payout reconciliation failed", zap.Error(err))
			}
		}
	}
}

// ReconcileProviderPayouts relit auprès de FedaPay les refunds et payouts processing dont le virement
// a été démarré depuis plus de payoutReconcileDelay, et applique leur statut. Une erreur sur un
// virement est journalisée sans interrompre le lot.
func (s *paymentServiceImpl) ReconcileProviderPayouts(ctx context.Context) error {
	if s.cache == nil || s.fedapayClient == nil {
		return nil
	}

	acquired, err := s.cache.AcquireReconcileLock(ctx)
	if err != nil {
		return fmt.Errorf("acquire reconcile lock: %w", err)
	}
	if !acquired {
		s.logger.Debug("reconcile lock already held, skipping")
		return nil
	}
	defer s.cache.ReleaseReconcileLock(ctx)

	ctx = withHistoryOrigin(ctx, domain.HistoryInitiatedBySystem, domain.HistorySourcePayoutReconciler)
	updatedBefore := time.Now().UTC().Add(-payoutReconcileDelay)

	if s.refundReadRepo != nil {
		// Réconcilier les virements passagers en cours
		refunds, err := s.refundReadRepo.GetProcessingRefundsToReconcile(ctx, updatedBefore, payoutReconcileBatchSize)
		if err != nil {
			return fmt.Errorf("get processing refunds: %w", err)
		}
		for _, refund := range refunds {
			s.reconcileRefund(ctx, refund)
		}

		// Réconcilier les virements chauffeur issus de remboursements en cours
		driverRefunds, err := s.refundReadRepo.GetProcessingDriverRefundPayoutsToReconcile(ctx, updatedBefore, payoutReconcileBatchSize)
		if err != nil {
			return fmt.Errorf("get processing driver refund payouts: %w", err)
		}
		for _, refund := range driverRefunds {
			s.reconcileDriverRefundPayout(ctx, refund)
		}
	}

	payouts, err := s.payoutReadRepo.GetProcessingPayoutsToReconcile(ctx, updatedBefore, payoutReconcileBatchSize)
	if err != nil {
		return fmt.Errorf("get processing payouts: %w", err)
	}
	for _, payout := range payouts {
		s.reconcileDriverPayout(ctx, payout)
	}
	return nil
}

func (s *paymentServiceImpl) reconcileRefund(ctx context.Context, refund *domain.Refund) {
	s.reconcileProviderPayout(ctx, "refundID", refund.RefundID, refund.PaymentProviderReference,
		func(item *fedapay.PayoutItem) error { return s.applyRefundPayoutStatus(ctx, refund, item) })
}

func (s *paymentServiceImpl) reconcileDriverRefundPayout(ctx context.Context, refund *domain.Refund) {
	s.reconcileProviderPayout(ctx, "refundID(driver)", refund.RefundID, refund.DriverPayoutProviderRef,
		func(item *fedapay.PayoutItem) error { return s.applyDriverRefundPayoutStatus(ctx, refund, item) })
}

func (s *paymentServiceImpl) reconcileDriverPayout(ctx context.Context, payout *domain.Payout) {
	s.reconcileProviderPayout(ctx, "payoutID", payout.PayoutID, payout.PaymentProviderReference,
		func(item *fedapay.PayoutItem) error { return s.applyDriverPayoutStatus(ctx, payout, item) })
}

// reconcileProviderPayout relit le virement FedaPay providerRef et applique son statut ; les erreurs
// sont journalisées (idKey/id identifient le refund ou le payout dans les logs).
func (s *paymentServiceImpl) reconcileProviderPayout(
	ctx context.Context, idKey, id, providerRef string, apply func(*fedapay.PayoutItem) error,
) {
	fedapayID, err := strconv.Atoi(providerRef)
	if err != nil {
		s.logger.Error("reconcile: invalid fedapay reference", zap.String(idKey, id), zap.String("providerRef", providerRef))
		return
	}
	item, err := s.fedapayClient.GetPayout(fedapayID)
	if err != nil {
		s.logger.Error("reconcile: get fedapay payout failed", zap.String(idKey, id), zap.Error(err))
		return
	}
	if err := apply(item); err != nil {
		s.logger.Error("reconcile: apply fedapay payout status failed", zap.String(idKey, id), zap.Error(err))
	}
}
