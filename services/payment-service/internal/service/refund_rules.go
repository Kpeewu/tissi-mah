package service

import (
	"time"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/config"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
)

// RefundCalculation contient les montants calculés pour un remboursement.
type RefundCalculation struct {
	RefundPercentage  int
	RefundAmount      int
	ServiceFeeRefunded bool
	AmountToPassenger int
	AmountToDriver    int
	AmountToPlatform  int
}

// DetermineRefundRule détermine la règle de remboursement à appliquer selon le contexte.
func DetermineRefundRule(
	reason domain.RefundReason,
	departureDatetime time.Time,
	approvedAt *time.Time,
	cancelledAt time.Time,
	cfg config.RefundConfig,
) domain.RefundRule {
	switch reason {
	case domain.RefundReasonCancelledByDriver:
		return domain.RefundRuleDriverCancellation

	case domain.RefundReasonNoShowDriver:
		return domain.RefundRuleNoShowDriver

	case domain.RefundReasonNoShowPassenger:
		return domain.RefundRuleNoShowPassenger

	case domain.RefundReasonCancelledByPassenger:
		// Plus de 24h avant le départ → remboursement intégral
		fullRefundDeadline := departureDatetime.Add(-time.Duration(cfg.CancellationFullRefundHours) * time.Hour)
		if cancelledAt.Before(fullRefundDeadline) {
			return domain.RefundRuleCancelled24hBefore
		}

		// Moins de 30min après l'acceptation du conducteur → remboursement intégral
		if approvedAt != nil {
			gracePeriodEnd := approvedAt.Add(time.Duration(cfg.CancellationGracePeriodMinutes) * time.Minute)
			if cancelledAt.Before(gracePeriodEnd) {
				return domain.RefundRuleCancelled30minAfterApproval
			}
		}

		// Sinon → remboursement partiel (50%)
		return domain.RefundRuleCancelledOver30minAfterApproval

	case domain.RefundReasonBookingRejected:
		// Rejet par le chauffeur → même traitement qu'annulation chauffeur
		return domain.RefundRuleDriverCancellation

	case domain.RefundReasonTripCancelled:
		// Trip annulé → même traitement qu'annulation chauffeur
		return domain.RefundRuleDriverCancellation

	default:
		// Par défaut, remboursement intégral
		return domain.RefundRuleDriverCancellation
	}
}

// CalculateRefund calcule les montants de remboursement selon la règle.
func CalculateRefund(originalAmount int, serviceFee int, rule domain.RefundRule) *RefundCalculation {
	switch rule {
	case domain.RefundRuleDriverCancellation, domain.RefundRuleNoShowDriver:
		// Remboursement intégral (montant + frais de service)
		total := originalAmount + serviceFee
		return &RefundCalculation{
			RefundPercentage:  100,
			RefundAmount:      total,
			ServiceFeeRefunded: true,
			AmountToPassenger: total,
			AmountToDriver:    0,
			AmountToPlatform:  0,
		}

	case domain.RefundRuleCancelled24hBefore, domain.RefundRuleCancelled30minAfterApproval:
		// 100% du montant au passager, frais de service non remboursés
		return &RefundCalculation{
			RefundPercentage:  100,
			RefundAmount:      originalAmount,
			ServiceFeeRefunded: false,
			AmountToPassenger: originalAmount,
			AmountToDriver:    0,
			AmountToPlatform:  serviceFee,
		}

	case domain.RefundRuleCancelledOver30minAfterApproval:
		// 50% du montant (hors frais) au passager, 50% au chauffeur
		halfAmount := originalAmount / 2
		return &RefundCalculation{
			RefundPercentage:  50,
			RefundAmount:      halfAmount,
			ServiceFeeRefunded: false,
			AmountToPassenger: halfAmount,
			AmountToDriver:    originalAmount - halfAmount,
			AmountToPlatform:  serviceFee,
		}

	case domain.RefundRuleNoShowPassenger:
		// Pas de remboursement
		return &RefundCalculation{
			RefundPercentage:  0,
			RefundAmount:      0,
			ServiceFeeRefunded: false,
			AmountToPassenger: 0,
			AmountToDriver:    originalAmount,
			AmountToPlatform:  serviceFee,
		}

	default:
		return &RefundCalculation{
			RefundPercentage:  0,
			RefundAmount:      0,
			ServiceFeeRefunded: false,
			AmountToPassenger: 0,
			AmountToDriver:    0,
			AmountToPlatform:  originalAmount + serviceFee,
		}
	}
}
