package service_test

import (
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/config"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/service"
	"github.com/stretchr/testify/assert"
)

func defaultRefundConfig() config.RefundConfig {
	return config.RefundConfig{
		CancellationFullRefundHours:    24,
		CancellationGracePeriodMinutes: 30,
		NoShowDriverDelayMinutes:       15,
		NoShowPassengerDelayMinutes:    15,
	}
}

// =============================================================================
// DetermineRefundRule
// =============================================================================

func TestDetermineRefundRule(t *testing.T) {
	cfg := defaultRefundConfig()
	departure := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)

	t.Run("annulation chauffeur → driverCancellation", func(t *testing.T) {
		rule := service.DetermineRefundRule(
			domain.RefundReasonCancelledByDriver,
			departure, nil,
			time.Now().UTC(), cfg,
		)
		assert.Equal(t, domain.RefundRuleDriverCancellation, rule)
	})

	t.Run("no-show chauffeur → noShowDriver", func(t *testing.T) {
		rule := service.DetermineRefundRule(
			domain.RefundReasonNoShowDriver,
			departure, nil,
			time.Now().UTC(), cfg,
		)
		assert.Equal(t, domain.RefundRuleNoShowDriver, rule)
	})

	t.Run("no-show passager → noShowPassenger", func(t *testing.T) {
		rule := service.DetermineRefundRule(
			domain.RefundReasonNoShowPassenger,
			departure, nil,
			time.Now().UTC(), cfg,
		)
		assert.Equal(t, domain.RefundRuleNoShowPassenger, rule)
	})

	t.Run("rejet booking → driverCancellation", func(t *testing.T) {
		rule := service.DetermineRefundRule(
			domain.RefundReasonBookingRejected,
			departure, nil,
			time.Now().UTC(), cfg,
		)
		assert.Equal(t, domain.RefundRuleDriverCancellation, rule)
	})

	t.Run("trip annule → driverCancellation", func(t *testing.T) {
		rule := service.DetermineRefundRule(
			domain.RefundReasonTripCancelled,
			departure, nil,
			time.Now().UTC(), cfg,
		)
		assert.Equal(t, domain.RefundRuleDriverCancellation, rule)
	})

	t.Run("passager annule plus de 24h avant depart → cancelled24hBefore", func(t *testing.T) {
		cancelledAt := departure.Add(-25 * time.Hour) // 25h avant
		rule := service.DetermineRefundRule(
			domain.RefundReasonCancelledByPassenger,
			departure, nil,
			cancelledAt, cfg,
		)
		assert.Equal(t, domain.RefundRuleCancelled24hBefore, rule)
	})

	t.Run("passager annule moins de 30min apres approbation → cancelled30minAfterApproval", func(t *testing.T) {
		// Depart dans 1h → annulation est dans les 24h, donc la condition 24h ne s'applique pas
		nearDeparture := time.Now().UTC().Add(1 * time.Hour)
		approvedAt := time.Now().UTC()
		cancelledAt := approvedAt.Add(10 * time.Minute) // 10min apres
		rule := service.DetermineRefundRule(
			domain.RefundReasonCancelledByPassenger,
			nearDeparture, &approvedAt,
			cancelledAt, cfg,
		)
		assert.Equal(t, domain.RefundRuleCancelled30minAfterApproval, rule)
	})

	t.Run("passager annule plus de 30min apres approbation → cancelledOver30minAfterApproval", func(t *testing.T) {
		// Depart dans 1h → annulation est dans les 24h
		nearDeparture := time.Now().UTC().Add(1 * time.Hour)
		approvedAt := time.Now().UTC().Add(-2 * time.Hour)
		cancelledAt := approvedAt.Add(45 * time.Minute) // 45min apres
		rule := service.DetermineRefundRule(
			domain.RefundReasonCancelledByPassenger,
			nearDeparture, &approvedAt,
			cancelledAt, cfg,
		)
		assert.Equal(t, domain.RefundRuleCancelledOver30minAfterApproval, rule)
	})

	t.Run("passager annule sans approbation et moins de 24h avant → over30min", func(t *testing.T) {
		cancelledAt := departure.Add(-12 * time.Hour) // 12h avant, pas d'approbation
		rule := service.DetermineRefundRule(
			domain.RefundReasonCancelledByPassenger,
			departure, nil,
			cancelledAt, cfg,
		)
		assert.Equal(t, domain.RefundRuleCancelledOver30minAfterApproval, rule)
	})
}

// =============================================================================
// CalculateRefund
// =============================================================================

func TestCalculateRefund(t *testing.T) {
	originalAmount := 5000
	serviceFee := 500

	t.Run("driverCancellation → 100% remboursement total (montant + frais)", func(t *testing.T) {
		calc := service.CalculateRefund(originalAmount, serviceFee, domain.RefundRuleDriverCancellation)

		assert.Equal(t, 100, calc.RefundPercentage)
		assert.Equal(t, 5500, calc.RefundAmount) // 5000 + 500
		assert.True(t, calc.ServiceFeeRefunded)
		assert.Equal(t, 5500, calc.AmountToPassenger)
		assert.Equal(t, 0, calc.AmountToDriver)
		assert.Equal(t, 0, calc.AmountToPlatform)
	})

	t.Run("noShowDriver → 100% remboursement total (montant + frais)", func(t *testing.T) {
		calc := service.CalculateRefund(originalAmount, serviceFee, domain.RefundRuleNoShowDriver)

		assert.Equal(t, 100, calc.RefundPercentage)
		assert.Equal(t, 5500, calc.RefundAmount)
		assert.True(t, calc.ServiceFeeRefunded)
		assert.Equal(t, 5500, calc.AmountToPassenger)
		assert.Equal(t, 0, calc.AmountToDriver)
		assert.Equal(t, 0, calc.AmountToPlatform)
	})

	t.Run("cancelled24hBefore → 100% montant au passager, frais conserves", func(t *testing.T) {
		calc := service.CalculateRefund(originalAmount, serviceFee, domain.RefundRuleCancelled24hBefore)

		assert.Equal(t, 100, calc.RefundPercentage)
		assert.Equal(t, 5000, calc.RefundAmount) // seulement le montant
		assert.False(t, calc.ServiceFeeRefunded)
		assert.Equal(t, 5000, calc.AmountToPassenger)
		assert.Equal(t, 0, calc.AmountToDriver)
		assert.Equal(t, 500, calc.AmountToPlatform)
	})

	t.Run("cancelled30minAfterApproval → 100% montant au passager, frais conserves", func(t *testing.T) {
		calc := service.CalculateRefund(originalAmount, serviceFee, domain.RefundRuleCancelled30minAfterApproval)

		assert.Equal(t, 100, calc.RefundPercentage)
		assert.Equal(t, 5000, calc.RefundAmount)
		assert.False(t, calc.ServiceFeeRefunded)
		assert.Equal(t, 5000, calc.AmountToPassenger)
		assert.Equal(t, 0, calc.AmountToDriver)
		assert.Equal(t, 500, calc.AmountToPlatform)
	})

	t.Run("cancelledOver30minAfterApproval → 50% au passager, 50% au chauffeur", func(t *testing.T) {
		calc := service.CalculateRefund(originalAmount, serviceFee, domain.RefundRuleCancelledOver30minAfterApproval)

		assert.Equal(t, 50, calc.RefundPercentage)
		assert.Equal(t, 2500, calc.RefundAmount) // 5000 / 2
		assert.False(t, calc.ServiceFeeRefunded)
		assert.Equal(t, 2500, calc.AmountToPassenger)
		assert.Equal(t, 2500, calc.AmountToDriver)
		assert.Equal(t, 500, calc.AmountToPlatform)
	})

	t.Run("noShowPassenger → 0% remboursement, tout au chauffeur + plateforme", func(t *testing.T) {
		calc := service.CalculateRefund(originalAmount, serviceFee, domain.RefundRuleNoShowPassenger)

		assert.Equal(t, 0, calc.RefundPercentage)
		assert.Equal(t, 0, calc.RefundAmount)
		assert.False(t, calc.ServiceFeeRefunded)
		assert.Equal(t, 0, calc.AmountToPassenger)
		assert.Equal(t, 5000, calc.AmountToDriver)
		assert.Equal(t, 500, calc.AmountToPlatform)
	})

	t.Run("montant impair → arrondi correct avec 50%", func(t *testing.T) {
		calc := service.CalculateRefund(5001, 500, domain.RefundRuleCancelledOver30minAfterApproval)

		assert.Equal(t, 50, calc.RefundPercentage)
		assert.Equal(t, 2500, calc.RefundAmount) // 5001/2 = 2500 (int division)
		assert.Equal(t, 2500, calc.AmountToPassenger)
		assert.Equal(t, 2501, calc.AmountToDriver) // 5001 - 2500
	})
}
