package handler_test

import (
	"context"
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/middleware"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/trips-service/internal/service/interfaces"
	tripErrors "github.com/Kpeewu/tissi-mah/services/trips-service/pkg/errors"
	trippb "github.com/Kpeewu/tissi-mah/services/trips-service/proto/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
)

// emptyCtx retourne un contexte sans Firebase UID — déclenche ErrorUnauthorized.
func emptyCtx() context.Context { return context.Background() }

func TestHealth_Handler(t *testing.T) {
	handler, _ := newHandler()
	resp, err := handler.Health(context.Background(), &trippb.HealthRequest{})
	require.NoError(t, err)
	assert.Equal(t, "SERVING", resp.Status)
	assert.NotZero(t, resp.Timestamp)
}

func TestCreateRecurringTrip_Handler(t *testing.T) {
	t.Run("succès", func(t *testing.T) {
		handler, svc := newHandler()
		ctx := ctxWithUID()
		svc.On("CreateRecurringTrip", ctx, mock.AnythingOfType("*interfaces.CreateRecurringTripInput")).
			Return("pattern-1", nil)

		resp, err := handler.CreateRecurringTrip(ctx, &trippb.CreateRecurringTripRequest{
			VehicleId:      "v1",
			DepartureTime:  time.Now().Format(time.RFC3339),
			RecurrenceType: "weekly",
			StartDate:      "2026-01-01",
			EndDate:        "2026-02-01",
			TotalSeats:     4,
			DaysOfWeek:     &trippb.DaysOfWeekInput{Days: []int32{1, 3}},
		})
		require.NoError(t, err)
		assert.True(t, resp.Success)
	})

	t.Run("ctx sans UID → Unauthenticated-style (PermissionDenied)", func(t *testing.T) {
		handler, _ := newHandler()
		_, err := handler.CreateRecurringTrip(emptyCtx(), &trippb.CreateRecurringTripRequest{})
		assertGRPCCode(t, err, codes.PermissionDenied)
	})

	t.Run("service erreur → mapping gRPC", func(t *testing.T) {
		handler, svc := newHandler()
		ctx := ctxWithUID()
		svc.On("CreateRecurringTrip", ctx, mock.Anything).Return("", tripErrors.ErrorInvalidInput)
		_, err := handler.CreateRecurringTrip(ctx, &trippb.CreateRecurringTripRequest{})
		assertGRPCCode(t, err, codes.InvalidArgument)
	})
}

func TestGetTripsPreviews_Handler(t *testing.T) {
	t.Run("succès - mappe résultats proto", func(t *testing.T) {
		handler, svc := newHandler()
		ctx := ctxWithUID()
		depart := time.Date(2026, 1, 2, 8, 30, 0, 0, time.UTC)
		svc.On("GetTripsPreviews", ctx, mock.Anything).Return([]*serviceInterfaces.TripPreviewResult{
			{TripID: "t1", DriverID: "d1", DriverName: "J", VehicleID: "v1",
				VehicleBrand: "T", VehiclePlate: "AA", DepartureDatetime: depart,
				TotalSeats: 4, AvailableSeats: 3,
				DepartureLocationName: "A", ArrivalLocationName: "B"},
		}, nil)

		resp, err := handler.GetTripsPreviews(ctx, &trippb.GetTripsPreviewsRequest{Index: 0})
		require.NoError(t, err)
		require.Len(t, resp.TripsPreviews, 1)
		assert.Equal(t, "t1", resp.TripsPreviews[0].TripId)
		assert.Equal(t, "2026-01-02", resp.TripsPreviews[0].DepartureDate)
		assert.Equal(t, "08:30", resp.TripsPreviews[0].DepartureTime)
	})

	t.Run("ctx sans UID", func(t *testing.T) {
		handler, _ := newHandler()
		_, err := handler.GetTripsPreviews(emptyCtx(), &trippb.GetTripsPreviewsRequest{})
		assertGRPCCode(t, err, codes.PermissionDenied)
	})
}

func TestGetCompletedTripsPreviews_Handler(t *testing.T) {
	handler, svc := newHandler()
	ctx := ctxWithUID()
	svc.On("GetCompletedTripsPreviews", ctx, mock.Anything).Return([]*serviceInterfaces.CompletedTripPreviewResult{
		{TripID: "tc1", DepartureDatetime: time.Now().UTC()},
	}, nil)
	resp, err := handler.GetCompletedTripsPreviews(ctx, &trippb.GetCompletedTripsPreviewsRequest{Index: 0})
	require.NoError(t, err)
	assert.Len(t, resp.TripsPreviews, 1)
}

func TestGetScheduledTripsPreviews_Handler(t *testing.T) {
	t.Run("erreur - départ vide → InvalidArgument", func(t *testing.T) {
		handler, _ := newHandler()
		_, err := handler.GetScheduledTripsPreviews(context.Background(), &trippb.GetScheduledTripsPreviewsRequest{
			DepartureLocationName: "", ArrivalLocationName: "B",
		})
		assertGRPCCode(t, err, codes.InvalidArgument)
	})

	t.Run("succès - mappe pagination", func(t *testing.T) {
		handler, svc := newHandler()
		svc.On("GetScheduledTripsPreviews", mock.Anything, mock.AnythingOfType("*interfaces.GetScheduledTripsPreviewsInput")).
			Return(&serviceInterfaces.ScheduledTripsPreviewsResult{
				Previews:   []*serviceInterfaces.TripPreviewResult{{TripID: "t1", DepartureDatetime: time.Now().UTC()}},
				NextIndex:  -1,
				TotalCount: 1,
			}, nil)
		resp, err := handler.GetScheduledTripsPreviews(context.Background(), &trippb.GetScheduledTripsPreviewsRequest{
			DepartureLocationName: "A", ArrivalLocationName: "B",
		})
		require.NoError(t, err)
		assert.Equal(t, int32(1), resp.TotalCount)
		assert.Equal(t, int32(-1), resp.NextIndex)
	})
}

func TestChangeTripDateAndTime_Handler(t *testing.T) {
	handler, svc := newHandler()
	ctx := ctxWithUID()
	svc.On("ChangeTripDateAndTime", ctx, mock.Anything).Return(nil)
	resp, err := handler.ChangeTripDateAndTime(ctx, &trippb.ChangeTripDateAndTimeRequest{TripId: "t", DepartureDatetime: "x"})
	require.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestChangeTripVehicle_Handler(t *testing.T) {
	t.Run("succès", func(t *testing.T) {
		handler, svc := newHandler()
		ctx := ctxWithUID()
		svc.On("ChangeTripVehicle", ctx, mock.Anything).Return(nil)
		resp, err := handler.ChangeTripVehicle(ctx, &trippb.ChangeTripVehicleRequest{TripId: "t", VehicleId: "v"})
		require.NoError(t, err)
		assert.True(t, resp.Success)
	})

	t.Run("erreur service", func(t *testing.T) {
		handler, svc := newHandler()
		ctx := ctxWithUID()
		svc.On("ChangeTripVehicle", ctx, mock.Anything).Return(tripErrors.ErrorTripNotScheduled)
		_, err := handler.ChangeTripVehicle(ctx, &trippb.ChangeTripVehicleRequest{TripId: "t", VehicleId: "v"})
		assertGRPCCode(t, err, codes.FailedPrecondition)
	})
}

func TestChangeTripAllowances_Handler(t *testing.T) {
	handler, svc := newHandler()
	ctx := ctxWithUID()
	svc.On("ChangeTripAllowances", ctx, mock.Anything).Return(nil)
	resp, err := handler.ChangeTripAllowances(ctx, &trippb.ChangeTripAllowancesRequest{TripId: "t"})
	require.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestChangeAutoApprove_Handler(t *testing.T) {
	handler, svc := newHandler()
	ctx := ctxWithUID()
	svc.On("ChangeAutoApprove", ctx, mock.Anything).Return(nil)
	resp, err := handler.ChangeAutoApprove(ctx, &trippb.ChangeAutoApproveRequest{TripId: "t", AutoApprove: true})
	require.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestGetDriverTripDetails_Handler(t *testing.T) {
	t.Run("succès", func(t *testing.T) {
		handler, svc := newHandler()
		ctx := ctxWithUID()
		now := time.Now().UTC()
		svc.On("GetDriverTripDetails", ctx, mock.Anything).Return(&serviceInterfaces.DriverTripDetailResult{
			TripID: "t1", DriverID: "driver-1",
			DepartureDatetime: now, EstimatedArrivalDatetime: now.Add(time.Hour),
			Waypoints: []serviceInterfaces.DriverWaypointDetailResult{{WaypointID: "w1"}},
		}, nil)
		resp, err := handler.GetDriverTripDetails(ctx, &trippb.GetDriverTripDetailsRequest{TripId: "t1"})
		require.NoError(t, err)
		assert.Equal(t, "t1", resp.TripId)
		assert.Len(t, resp.Waypoints, 1)
	})

	t.Run("ctx sans UID → PermissionDenied", func(t *testing.T) {
		handler, _ := newHandler()
		_, err := handler.GetDriverTripDetails(context.Background(), &trippb.GetDriverTripDetailsRequest{TripId: "t"})
		assertGRPCCode(t, err, codes.PermissionDenied)
	})

	t.Run("service erreur NotFound", func(t *testing.T) {
		handler, svc := newHandler()
		ctx := context.WithValue(context.Background(), middleware.FirebaseIDKey, "driver-1")
		svc.On("GetDriverTripDetails", ctx, mock.Anything).
			Return((*serviceInterfaces.DriverTripDetailResult)(nil), tripErrors.ErrorTripNotFound)
		_, err := handler.GetDriverTripDetails(ctx, &trippb.GetDriverTripDetailsRequest{TripId: "bad"})
		assertGRPCCode(t, err, codes.NotFound)
	})
}

func TestGetPassengerTripDetails_Handler(t *testing.T) {
	t.Run("succès - ctx public", func(t *testing.T) {
		handler, svc := newHandler()
		now := time.Now().UTC()
		svc.On("GetPassengerTripDetails", mock.Anything, mock.Anything).Return(&serviceInterfaces.PassengerTripDetailResult{
			TripID: "t1", DepartureDatetime: now, EstimatedArrivalDatetime: now,
		}, nil)
		resp, err := handler.GetPassengerTripDetails(context.Background(), &trippb.GetPassengerTripDetailsRequest{TripId: "t1"})
		require.NoError(t, err)
		assert.Equal(t, "t1", resp.TripId)
	})

	t.Run("erreur service - TripID vide", func(t *testing.T) {
		handler, svc := newHandler()
		svc.On("GetPassengerTripDetails", mock.Anything, mock.Anything).
			Return((*serviceInterfaces.PassengerTripDetailResult)(nil), tripErrors.ErrorInvalidInput)
		_, err := handler.GetPassengerTripDetails(context.Background(), &trippb.GetPassengerTripDetailsRequest{TripId: ""})
		assertGRPCCode(t, err, codes.InvalidArgument)
	})
}

func TestCancelTrip_Handler(t *testing.T) {
	t.Run("succès", func(t *testing.T) {
		handler, svc := newHandler()
		ctx := ctxWithUID()
		svc.On("CancelTrip", ctx, mock.Anything).Return(nil)
		resp, err := handler.CancelTrip(ctx, &trippb.CancelTripRequest{TripId: "t", CancellationReason: "r"})
		require.NoError(t, err)
		assert.True(t, resp.Success)
	})

	t.Run("erreur - déjà annulé → FailedPrecondition", func(t *testing.T) {
		handler, svc := newHandler()
		ctx := ctxWithUID()
		svc.On("CancelTrip", ctx, mock.Anything).Return(tripErrors.ErrorTripNotScheduled)
		_, err := handler.CancelTrip(ctx, &trippb.CancelTripRequest{TripId: "t", CancellationReason: "r"})
		assertGRPCCode(t, err, codes.FailedPrecondition)
	})

	t.Run("ctx sans UID", func(t *testing.T) {
		handler, _ := newHandler()
		_, err := handler.CancelTrip(context.Background(), &trippb.CancelTripRequest{TripId: "t"})
		assertGRPCCode(t, err, codes.PermissionDenied)
	})
}

func TestIncrementLegBookedSeats_Handler(t *testing.T) {
	t.Run("succès", func(t *testing.T) {
		handler, svc := newHandler()
		svc.On("IncrementLegBookedSeats", mock.Anything, mock.Anything).Return(nil)
		resp, err := handler.IncrementLegBookedSeats(context.Background(), &trippb.IncrementLegBookedSeatsRequest{
			TripId: "t", FromOrder: 1, ToOrder: 2, Delta: 1,
		})
		require.NoError(t, err)
		assert.True(t, resp.Success)
	})

	t.Run("TripID vide → InvalidArgument", func(t *testing.T) {
		handler, _ := newHandler()
		_, err := handler.IncrementLegBookedSeats(context.Background(), &trippb.IncrementLegBookedSeatsRequest{TripId: ""})
		assertGRPCCode(t, err, codes.InvalidArgument)
	})

	t.Run("service erreur", func(t *testing.T) {
		handler, svc := newHandler()
		svc.On("IncrementLegBookedSeats", mock.Anything, mock.Anything).Return(tripErrors.ErrorInvalidInput)
		_, err := handler.IncrementLegBookedSeats(context.Background(), &trippb.IncrementLegBookedSeatsRequest{
			TripId: "t", FromOrder: 1, ToOrder: 1, Delta: 0,
		})
		assertGRPCCode(t, err, codes.InvalidArgument)
	})
}

func TestSyncLegBookedSeats_Handler(t *testing.T) {
	t.Run("succès", func(t *testing.T) {
		handler, svc := newHandler()
		svc.On("SyncLegBookedSeats", mock.Anything, mock.Anything).Return(nil)
		resp, err := handler.SyncLegBookedSeats(context.Background(), &trippb.SyncLegBookedSeatsRequest{
			TripId: "t",
			Legs:   []*trippb.LegBookedSeatsEntry{{SequencerOrder: 1, BookedSeats: 2}},
		})
		require.NoError(t, err)
		assert.True(t, resp.Success)
	})

	t.Run("legs vide → InvalidArgument", func(t *testing.T) {
		handler, _ := newHandler()
		_, err := handler.SyncLegBookedSeats(context.Background(), &trippb.SyncLegBookedSeatsRequest{TripId: "t"})
		assertGRPCCode(t, err, codes.InvalidArgument)
	})

	t.Run("TripID vide → InvalidArgument", func(t *testing.T) {
		handler, _ := newHandler()
		_, err := handler.SyncLegBookedSeats(context.Background(), &trippb.SyncLegBookedSeatsRequest{
			TripId: "", Legs: []*trippb.LegBookedSeatsEntry{{SequencerOrder: 1}},
		})
		assertGRPCCode(t, err, codes.InvalidArgument)
	})
}
