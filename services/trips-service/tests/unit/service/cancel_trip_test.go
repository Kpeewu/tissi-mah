package service_test

import (
	"context"
	"testing"

	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/trips-service/internal/service/interfaces"
	tripErrors "github.com/Kpeewu/tissi-mah/services/trips-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCancelTrip(t *testing.T) {
	t.Run("succès", func(t *testing.T) {
		_, writeRepo, userClient, _, svc := newTestService()
		ctx := context.Background()
		userClient.On("GetUserIDByAuthID", ctx, "driver-1").Return("driver-1", nil)
		writeRepo.On("CancelTrip", ctx, "trip-1", "driver-1", "raison").Return(nil)

		err := svc.CancelTrip(ctx, &serviceInterfaces.CancelTripInput{
			DriverID: "driver-1", TripID: "trip-1", CancellationReason: "raison",
		})
		require.NoError(t, err)
		writeRepo.AssertExpectations(t)
	})

	t.Run("erreur - TripID vide → InvalidInput", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		err := svc.CancelTrip(context.Background(), &serviceInterfaces.CancelTripInput{
			DriverID: "d", TripID: "", CancellationReason: "r",
		})
		assert.ErrorIs(t, err, tripErrors.ErrorInvalidInput)
	})

	t.Run("erreur - reason vide → InvalidInput", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		err := svc.CancelTrip(context.Background(), &serviceInterfaces.CancelTripInput{
			DriverID: "d", TripID: "t", CancellationReason: "",
		})
		assert.ErrorIs(t, err, tripErrors.ErrorInvalidInput)
	})

	t.Run("erreur - driverID vide → Unauthorized (resolve)", func(t *testing.T) {
		_, _, userClient, _, svc := newTestService()
		ctx := context.Background()
		userClient.On("GetUserIDByAuthID", ctx, "").Return("", nil).Maybe()
		err := svc.CancelTrip(ctx, &serviceInterfaces.CancelTripInput{
			DriverID: "", TripID: "t", CancellationReason: "r",
		})
		assert.ErrorIs(t, err, tripErrors.ErrorUnauthorized)
	})

	t.Run("erreur - repo CancelTrip échoue → propage", func(t *testing.T) {
		_, writeRepo, userClient, _, svc := newTestService()
		ctx := context.Background()
		userClient.On("GetUserIDByAuthID", ctx, "driver-1").Return("driver-1", nil)
		writeRepo.On("CancelTrip", ctx, "trip-404", "driver-1", "r").Return(tripErrors.ErrorTripNotFound)

		err := svc.CancelTrip(ctx, &serviceInterfaces.CancelTripInput{
			DriverID: "driver-1", TripID: "trip-404", CancellationReason: "r",
		})
		assert.ErrorIs(t, err, tripErrors.ErrorTripNotFound)
	})
}

func TestIncrementLegBookedSeats(t *testing.T) {
	t.Run("succès", func(t *testing.T) {
		_, writeRepo, _, _, svc := newTestService()
		ctx := context.Background()
		writeRepo.On("IncrementLegBookedSeats", ctx, "trip-1", 1, 3, 2).Return(nil)

		err := svc.IncrementLegBookedSeats(ctx, &serviceInterfaces.IncrementLegBookedSeatsInput{
			TripID: "trip-1", FromOrder: 1, ToOrder: 3, Delta: 2,
		})
		require.NoError(t, err)
	})

	t.Run("erreur - delta 0 → InvalidInput", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		err := svc.IncrementLegBookedSeats(context.Background(), &serviceInterfaces.IncrementLegBookedSeatsInput{
			TripID: "t", FromOrder: 1, ToOrder: 2, Delta: 0,
		})
		assert.ErrorIs(t, err, tripErrors.ErrorInvalidInput)
	})

	t.Run("erreur - ToOrder <= FromOrder → InvalidInput", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		err := svc.IncrementLegBookedSeats(context.Background(), &serviceInterfaces.IncrementLegBookedSeatsInput{
			TripID: "t", FromOrder: 3, ToOrder: 3, Delta: 1,
		})
		assert.ErrorIs(t, err, tripErrors.ErrorInvalidInput)
	})

	t.Run("erreur - TripID vide → InvalidInput", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		err := svc.IncrementLegBookedSeats(context.Background(), &serviceInterfaces.IncrementLegBookedSeatsInput{
			TripID: "", FromOrder: 1, ToOrder: 2, Delta: 1,
		})
		assert.ErrorIs(t, err, tripErrors.ErrorInvalidInput)
	})
}

func TestSyncLegBookedSeats(t *testing.T) {
	t.Run("succès", func(t *testing.T) {
		_, writeRepo, _, _, svc := newTestService()
		ctx := context.Background()
		writeRepo.On("SyncLegBookedSeats", ctx, "trip-1", mock.Anything).Return(nil)

		err := svc.SyncLegBookedSeats(ctx, &serviceInterfaces.SyncLegBookedSeatsInput{
			TripID: "trip-1",
			Legs: []serviceInterfaces.LegBookedSeats{
				{SequencerOrder: 1, BookedSeats: 2},
				{SequencerOrder: 2, BookedSeats: 3},
			},
		})
		require.NoError(t, err)
	})

	t.Run("erreur - legs vide → InvalidInput", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		err := svc.SyncLegBookedSeats(context.Background(), &serviceInterfaces.SyncLegBookedSeatsInput{
			TripID: "trip-1", Legs: nil,
		})
		assert.ErrorIs(t, err, tripErrors.ErrorInvalidInput)
	})
}
