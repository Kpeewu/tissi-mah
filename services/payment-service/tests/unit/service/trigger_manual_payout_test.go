package service_test

import (
	"context"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/client"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestTriggerManualPayout couvre les cas d'erreur (input, précondition, autorisation)
// ainsi que le chemin où aucun paiement released n'existe (processPayoutForTrip no-op).
// Le chemin nominal complet (création payout + appel FedaPay) n'est pas testable unitairement
// car le client FedaPay n'est pas mockable sans réécriture — couvert indirectement en e2e.
func TestTriggerManualPayout(t *testing.T) {
	ctx := context.Background()

	t.Run("tripID vide - retourne ErrorInvalidInput", func(t *testing.T) {
		d := newTestService()

		_, err := d.svc.TriggerManualPayout(ctx, "", "support-1")

		require.ErrorIs(t, err, paymentErrors.ErrorInvalidInput)
		d.assertExpectations(t)
	})

	t.Run("supportUserID vide - retourne ErrorInvalidInput", func(t *testing.T) {
		d := newTestService()

		_, err := d.svc.TriggerManualPayout(ctx, "trip-1", "")

		require.ErrorIs(t, err, paymentErrors.ErrorInvalidInput)
		d.assertExpectations(t)
	})

	t.Run("trip pas prêt - retourne ErrorTripNotReadyForPayout", func(t *testing.T) {
		d := newTestService()
		d.payoutReadRepo.On("IsTripReadyForPayout", mock.Anything, "trip-1").Return(false, nil)

		_, err := d.svc.TriggerManualPayout(ctx, "trip-1", "support-1")

		require.ErrorIs(t, err, paymentErrors.ErrorTripNotReadyForPayout)
		d.assertExpectations(t)
	})

	t.Run("erreur DB sur IsTripReadyForPayout - propage l'erreur", func(t *testing.T) {
		d := newTestService()
		d.payoutReadRepo.On("IsTripReadyForPayout", mock.Anything, "trip-1").
			Return(false, paymentErrors.ErrorDataRetrievalFailed)

		_, err := d.svc.TriggerManualPayout(ctx, "trip-1", "support-1")

		require.ErrorIs(t, err, paymentErrors.ErrorDataRetrievalFailed)
		d.assertExpectations(t)
	})

	t.Run("supportClient retourne une erreur - propage", func(t *testing.T) {
		d := newTestService()
		d.payoutReadRepo.On("IsTripReadyForPayout", mock.Anything, "trip-1").Return(true, nil)
		d.supportClient.On("GetSupportUserByID", mock.Anything, "support-1").
			Return(nil, paymentErrors.ErrorSupportServiceUnavailable)

		_, err := d.svc.TriggerManualPayout(ctx, "trip-1", "support-1")

		require.ErrorIs(t, err, paymentErrors.ErrorSupportServiceUnavailable)
		d.assertExpectations(t)
	})

	t.Run("user support désactivé - retourne ErrorUnauthorized", func(t *testing.T) {
		d := newTestService()
		d.payoutReadRepo.On("IsTripReadyForPayout", mock.Anything, "trip-1").Return(true, nil)
		d.supportClient.On("GetSupportUserByID", mock.Anything, "support-1").
			Return(&client.SupportUserInfo{
				UserID:    "support-1",
				FirstName: "Jean",
				LastName:  "Dupont",
				Role:      "admin",
				IsActive:  false,
			}, nil)

		_, err := d.svc.TriggerManualPayout(ctx, "trip-1", "support-1")

		require.ErrorIs(t, err, paymentErrors.ErrorUnauthorized)
		d.assertExpectations(t)
	})

	t.Run("rôle non habilité - retourne ErrorUnauthorized", func(t *testing.T) {
		d := newTestService()
		d.payoutReadRepo.On("IsTripReadyForPayout", mock.Anything, "trip-1").Return(true, nil)
		d.supportClient.On("GetSupportUserByID", mock.Anything, "support-1").
			Return(&client.SupportUserInfo{
				UserID:    "support-1",
				FirstName: "Jean",
				LastName:  "Dupont",
				Role:      "driver",
				IsActive:  true,
			}, nil)

		_, err := d.svc.TriggerManualPayout(ctx, "trip-1", "support-1")

		require.ErrorIs(t, err, paymentErrors.ErrorUnauthorized)
		d.assertExpectations(t)
	})

	t.Run("role admin actif accepté - pas de released payments, no-op", func(t *testing.T) {
		d := newTestService()
		d.payoutReadRepo.On("IsTripReadyForPayout", mock.Anything, "trip-1").Return(true, nil)
		d.supportClient.On("GetSupportUserByID", mock.Anything, "support-1").
			Return(&client.SupportUserInfo{
				UserID:    "support-1",
				FirstName: "Awa",
				LastName:  "Diallo",
				Role:      "admin",
				IsActive:  true,
			}, nil)
		// processPayoutForTrip : GetReleasedPaymentsForTrip retourne vide → no-op (pas d'appel FedaPay)
		d.payoutReadRepo.On("GetReleasedPaymentsForTrip", mock.Anything, "trip-1").
			Return(nil, nil)

		netAmount, err := d.svc.TriggerManualPayout(ctx, "trip-1", "support-1")

		require.NoError(t, err)
		require.Equal(t, 0, netAmount)
		// Ni CreatePayout ni CreateHistoryEntry appelés — pas d'historique launched_by_support
		d.assertExpectations(t)
	})

	t.Run("role support actif accepté - pas de released payments, no-op", func(t *testing.T) {
		d := newTestService()
		d.payoutReadRepo.On("IsTripReadyForPayout", mock.Anything, "trip-1").Return(true, nil)
		d.supportClient.On("GetSupportUserByID", mock.Anything, "support-1").
			Return(&client.SupportUserInfo{
				UserID:    "support-1",
				FirstName: "Awa",
				LastName:  "Diallo",
				Role:      "support",
				IsActive:  true,
			}, nil)
		d.payoutReadRepo.On("GetReleasedPaymentsForTrip", mock.Anything, "trip-1").
			Return(nil, nil)

		netAmount, err := d.svc.TriggerManualPayout(ctx, "trip-1", "support-1")

		require.NoError(t, err)
		require.Equal(t, 0, netAmount)
		d.assertExpectations(t)
	})
}
