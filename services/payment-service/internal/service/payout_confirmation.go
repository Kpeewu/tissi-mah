package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/pkg/notification"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/fedapay"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
)

// =============================================================================
// Confirmation des virements FedaPay (remboursements passagers et payouts chauffeurs)
// =============================================================================
//
// Un virement démarré (StartPayouts) n'est pas encore livré : le refund ou le payout reste processing
// jusqu'à ce que FedaPay donne un statut final. Les webhooks payout.* (noms non documentés par FedaPay)
// ne servent que de signal : le statut est toujours relu via GetPayout. Le worker de réconciliation
// rattrape les webhooks perdus (cf. payout_reconciler.go).

// confirmProviderPayout relit le payout FedaPay providerRef et applique son statut au refund ou au
// payout chauffeur correspondant. Une référence inconnue est ignorée ; une erreur DB ou FedaPay est
// retournée pour que l'appelant puisse réessayer (webhook rejoué).
func (s *paymentServiceImpl) confirmProviderPayout(ctx context.Context, providerRef string) error {
	payoutID, err := strconv.Atoi(providerRef)
	if err != nil || payoutID <= 0 {
		s.logger.Warn("invalid fedapay payout reference", zap.String("providerRef", providerRef))
		return nil
	}

	if s.refundReadRepo != nil {
		// Chercher un virement de remboursement passager
		refund, err := s.refundReadRepo.GetByProviderReference(ctx, providerRef)
		switch {
		case err == nil:
			item, err := s.fedapayClient.GetPayout(payoutID)
			if err != nil {
				s.logger.Error("get fedapay refund payout failed", zap.String("refundID", refund.RefundID), zap.Error(err))
				return paymentErrors.ErrorFedaPayAPIError
			}
			return s.applyRefundPayoutStatus(ctx, refund, item)
		case !errors.Is(err, paymentErrors.ErrorRefundNotFound):
			return err
		}

		// Chercher un virement de payout chauffeur issu d'un remboursement
		driverRefund, err := s.refundReadRepo.GetByDriverPayoutProviderReference(ctx, providerRef)
		switch {
		case err == nil:
			item, err := s.fedapayClient.GetPayout(payoutID)
			if err != nil {
				s.logger.Error("get fedapay driver refund payout failed", zap.String("refundID", driverRefund.RefundID), zap.Error(err))
				return paymentErrors.ErrorFedaPayAPIError
			}
			return s.applyDriverRefundPayoutStatus(ctx, driverRefund, item)
		case !errors.Is(err, paymentErrors.ErrorRefundNotFound):
			return err
		}
	}

	payout, err := s.payoutReadRepo.GetByProviderReference(ctx, providerRef)
	if err != nil {
		if errors.Is(err, paymentErrors.ErrorPayoutNotFound) {
			s.logger.Warn("fedapay payout matches no refund nor payout", zap.String("providerRef", providerRef))
			return nil
		}
		return err
	}
	item, err := s.fedapayClient.GetPayout(payoutID)
	if err != nil {
		s.logger.Error("get fedapay driver payout failed", zap.String("payoutID", payout.PayoutID), zap.Error(err))
		return paymentErrors.ErrorFedaPayAPIError
	}
	return s.applyDriverPayoutStatus(ctx, payout, item)
}

// applyRefundPayoutStatus applique le statut FedaPay du virement d'un refund processing :
// sent → completed (REFUND_COMPLETED), failed → pending pour une nouvelle tentative
// (le plafond REFUND_MAX_RETRIES du worker produit ensuite failed + REFUND_FAILED).
func (s *paymentServiceImpl) applyRefundPayoutStatus(ctx context.Context, refund *domain.Refund, item *fedapay.PayoutItem) error {
	providerRef := strconv.Itoa(item.ID)

	switch item.Status {
	case fedapay.PayoutStatusSent:
		if err := s.refundWriteRepo.MarkRefundCompleted(ctx, refund.RefundID, providerRef); err != nil {
			if errors.Is(err, paymentErrors.ErrorRefundAlreadyProcessed) {
				return nil
			}
			return err
		}
		s.writeRefundHistory(ctx, refund.RefundID, string(domain.RefundStatusCompleted), refund.RetryCount,
			"Virement FedaPay "+providerRef+" confirmé")
		s.logger.Info("refund completed", zap.String("refundID", refund.RefundID), zap.String("providerReference", providerRef))
		s.publishRefundCompleted(ctx, refund, providerRef)

	case fedapay.PayoutStatusFailed:
		reason := "fedapay payout failed: " + item.FailReason
		if err := s.refundWriteRepo.MarkRefundPayoutFailed(ctx, refund.RefundID, reason); err != nil {
			if errors.Is(err, paymentErrors.ErrorRefundAlreadyProcessed) {
				return nil
			}
			return err
		}
		s.writeRefundHistory(ctx, refund.RefundID, domain.RefundEventRetryScheduled, refund.RetryCount+1, reason)
		s.logger.Warn("refund payout failed at fedapay, will retry",
			zap.String("refundID", refund.RefundID), zap.String("reason", item.FailReason))

	default:
		s.logger.Debug("refund payout not final yet",
			zap.String("refundID", refund.RefundID), zap.String("fedapayStatus", item.Status))
	}
	return nil
}

// applyDriverPayoutStatus applique le statut FedaPay d'un payout chauffeur processing :
// sent → completed, failed → payout failed et paiements du trajet remis en released
// (repris par le batch suivant, au plus maxAutomaticPayoutAttempts fois).
func (s *paymentServiceImpl) applyDriverPayoutStatus(ctx context.Context, payout *domain.Payout, item *fedapay.PayoutItem) error {
	switch item.Status {
	case fedapay.PayoutStatusSent:
		if err := s.payoutWriteRepo.ConfirmPayoutCompleted(ctx, payout.PayoutID); err != nil {
			if errors.Is(err, paymentErrors.ErrorPayoutAlreadyProcessed) {
				return nil
			}
			return err
		}
		s.writePayoutHistory(ctx, payout.PayoutID, string(domain.PayoutStatusCompleted), "system", "", "", "", "Confirmé par FedaPay")
		s.logger.Info("driver payout completed", zap.String("payoutID", payout.PayoutID))

	case fedapay.PayoutStatusFailed:
		reason := "fedapay payout failed: " + item.FailReason
		paymentIDs, err := s.payoutWriteRepo.FailProcessingPayout(ctx, payout.PayoutID, payout.TripID, reason)
		if err != nil {
			if errors.Is(err, paymentErrors.ErrorPayoutAlreadyProcessed) {
				return nil
			}
			return err
		}
		s.writePayoutHistory(ctx, payout.PayoutID, string(domain.PayoutStatusFailed), "system", "", "", "", reason)
		for _, paymentID := range paymentIDs {
			s.writePaymentHistory(ctx, paymentID, string(domain.PaymentStatusReleased), "",
				fmt.Sprintf("Payout %s échoué chez FedaPay, nouvelle tentative au prochain batch", payout.PayoutReference))
		}
		s.logger.Warn("driver payout failed at fedapay, payments released for retry",
			zap.String("payoutID", payout.PayoutID),
			zap.String("tripID", payout.TripID),
			zap.String("reason", item.FailReason),
		)

	default:
		s.logger.Debug("driver payout not final yet",
			zap.String("payoutID", payout.PayoutID), zap.String("fedapayStatus", item.Status))
	}
	return nil
}

// applyDriverRefundPayoutStatus applique le statut FedaPay d'un payout chauffeur issu d'un remboursement :
// sent → completed (notification chauffeur), failed → pending pour une nouvelle tentative
// (le plafond de retry du worker produit ensuite failed définitif).
func (s *paymentServiceImpl) applyDriverRefundPayoutStatus(ctx context.Context, refund *domain.Refund, item *fedapay.PayoutItem) error {
	providerRef := strconv.Itoa(item.ID)

	switch item.Status {
	case fedapay.PayoutStatusSent:
		if err := s.refundWriteRepo.MarkDriverPayoutCompleted(ctx, refund.RefundID); err != nil {
			if errors.Is(err, paymentErrors.ErrorRefundAlreadyProcessed) {
				return nil
			}
			return err
		}
		s.writeRefundHistory(ctx, refund.RefundID, domain.RefundEventDriverPayoutStarted+"_completed", refund.DriverPayoutRetryCount,
			"Virement chauffeur FedaPay "+providerRef+" confirmé")
		s.logger.Info("driver refund payout completed", zap.String("refundID", refund.RefundID), zap.String("providerReference", providerRef))
		s.publishDriverRefundPayoutCompleted(ctx, refund, providerRef)

	case fedapay.PayoutStatusFailed:
		reason := "fedapay payout failed: " + item.FailReason
		if err := s.refundWriteRepo.MarkDriverPayoutFailed(ctx, refund.RefundID, reason, true); err != nil {
			if errors.Is(err, paymentErrors.ErrorRefundAlreadyProcessed) {
				return nil
			}
			return err
		}
		s.writeRefundHistory(ctx, refund.RefundID, domain.RefundEventDriverPayoutRetry, refund.DriverPayoutRetryCount+1, reason)
		s.logger.Warn("driver refund payout failed at fedapay, will retry",
			zap.String("refundID", refund.RefundID), zap.String("reason", item.FailReason))

	default:
		s.logger.Debug("driver refund payout not final yet",
			zap.String("refundID", refund.RefundID), zap.String("fedapayStatus", item.Status))
	}
	return nil
}

// publishDriverRefundPayoutCompleted notifie le chauffeur (non bloquant) que sa part du remboursement lui a été versée.
func (s *paymentServiceImpl) publishDriverRefundPayoutCompleted(ctx context.Context, refund *domain.Refund, providerRef string) {
	if s.notifRedis == nil {
		return
	}
	booking, err := s.bookingClient.GetBookingDetails(ctx, refund.BookingID)
	if err != nil {
		s.logger.Warn("get booking for driver REFUND_PAYOUT_COMPLETED notification failed",
			zap.String("refundID", refund.RefundID), zap.Error(err))
		return
	}
	if pubErr := notification.Publish(ctx, s.notifRedis, notification.Event{
		EventType:     notification.DriverPaymentLaunched,
		UserID:        booking.DriverID,
		ReferenceID:   refund.RefundID,
		ReferenceType: notification.RefRefund,
		Payload: map[string]string{
			"amount":                     fmt.Sprintf("%d", refund.AmountToDriver),
			"currency":                   "XOF",
			"payment_provider_reference": providerRef,
			"reason":                     string(refund.RefundReason),
		},
	}); pubErr != nil {
		s.logger.Error("failed to publish driver refund payout completed notification", zap.Error(pubErr))
	}
}

// publishRefundCompleted notifie le passager (non bloquant) que son remboursement lui a été versé.
func (s *paymentServiceImpl) publishRefundCompleted(ctx context.Context, refund *domain.Refund, providerRef string) {
	if s.notifRedis == nil {
		return
	}
	booking, err := s.bookingClient.GetBookingDetails(ctx, refund.BookingID)
	if err != nil {
		s.logger.Warn("get booking for REFUND_COMPLETED notification failed",
			zap.String("refundID", refund.RefundID), zap.Error(err))
		return
	}
	if pubErr := notification.Publish(ctx, s.notifRedis, notification.Event{
		EventType:     notification.RefundCompleted,
		UserID:        booking.PassengerID,
		ReferenceID:   refund.RefundID,
		ReferenceType: notification.RefRefund,
		Payload: map[string]string{
			"amount":                     fmt.Sprintf("%d", refund.AmountToPassenger),
			"currency":                   "XOF",
			"payment_provider_reference": providerRef,
			"reason":                     string(refund.RefundReason),
		},
	}); pubErr != nil {
		s.logger.Error("failed to publish REFUND_COMPLETED notification", zap.Error(pubErr))
	}
}
