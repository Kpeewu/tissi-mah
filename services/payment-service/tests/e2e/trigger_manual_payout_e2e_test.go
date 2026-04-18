package e2e

import (
	"testing"

	"github.com/Kpeewu/tissi-mah/services/payment-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	paymentpb "github.com/Kpeewu/tissi-mah/services/payment-service/proto/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestE2E_TriggerManualPayout couvre les chemins d'erreur et de précondition.
// Le chemin nominal complet (création de payout + appel FedaPay) n'est pas couvert
// ici car `FedaPayClient=nil` dans le setup e2e — même limite que ProcessPayoutBatch.
func TestE2E_TriggerManualPayout(t *testing.T) {
	t.Run("metadata manquant - Unauthenticated", func(t *testing.T) {
		ctx := ctxWithUID("e2e-user") // mauvais header (x-firebase-uid au lieu de x-support-uid)
		cleanupPaymentTables(t, ctx)

		_, err := grpcClient.TriggerManualPayout(ctx, &paymentpb.TriggerManualPayoutRequest{TripId: "trip-e2e-1"})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.Unauthenticated, st.Code())
	})

	t.Run("trip pas prêt - FailedPrecondition", func(t *testing.T) {
		ctx := ctxWithSupportUID("support-1")
		cleanupPaymentTables(t, ctx)

		_, err := grpcClient.TriggerManualPayout(ctx, &paymentpb.TriggerManualPayoutRequest{TripId: "trip-not-ready"})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.FailedPrecondition, st.Code())
	})

	t.Run("support inactif - PermissionDenied", func(t *testing.T) {
		ctx := ctxWithSupportUID("support-inactive")
		cleanupPaymentTables(t, ctx)

		payment := fixtures.NewTestPayment(
			fixtures.WithPaymentTripID("trip-inactive"),
			fixtures.WithPaymentStatus(domain.PaymentStatusReleased),
		)
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, payment))

		mockSupportClient.On("GetSupportUserByID", mock.Anything, "support-inactive").
			Return(&client.SupportUserInfo{
				UserID: "support-inactive", FirstName: "Jean", LastName: "Dupont",
				Role: "admin", IsActive: false,
			}, nil).Once()

		_, err := grpcClient.TriggerManualPayout(ctx, &paymentpb.TriggerManualPayoutRequest{TripId: "trip-inactive"})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.PermissionDenied, st.Code())
		mockSupportClient.AssertExpectations(t)
	})

	t.Run("rôle non habilité - PermissionDenied", func(t *testing.T) {
		ctx := ctxWithSupportUID("support-wrong-role")
		cleanupPaymentTables(t, ctx)

		payment := fixtures.NewTestPayment(
			fixtures.WithPaymentTripID("trip-wrong-role"),
			fixtures.WithPaymentStatus(domain.PaymentStatusReleased),
		)
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, payment))

		mockSupportClient.On("GetSupportUserByID", mock.Anything, "support-wrong-role").
			Return(&client.SupportUserInfo{
				UserID: "support-wrong-role", FirstName: "Awa", LastName: "Diallo",
				Role: "driver", IsActive: true,
			}, nil).Once()

		_, err := grpcClient.TriggerManualPayout(ctx, &paymentpb.TriggerManualPayoutRequest{TripId: "trip-wrong-role"})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.PermissionDenied, st.Code())
		mockSupportClient.AssertExpectations(t)
	})

	t.Run("payout scheduled existant - FailedPrecondition (trip non prêt)", func(t *testing.T) {
		ctx := ctxWithSupportUID("support-2")
		cleanupPaymentTables(t, ctx)

		payment := fixtures.NewTestPayment(
			fixtures.WithPaymentTripID("trip-already-payout"),
			fixtures.WithPaymentStatus(domain.PaymentStatusReleased),
		)
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, payment))

		payout := fixtures.NewTestPayout(
			fixtures.WithPayoutTripID("trip-already-payout"),
			fixtures.WithPayoutStatus(domain.PayoutStatusScheduled),
		)
		require.NoError(t, fixtures.InsertPayout(ctx, testPool, payout))

		_, err := grpcClient.TriggerManualPayout(ctx, &paymentpb.TriggerManualPayoutRequest{TripId: "trip-already-payout"})

		require.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.FailedPrecondition, st.Code())
	})
}
