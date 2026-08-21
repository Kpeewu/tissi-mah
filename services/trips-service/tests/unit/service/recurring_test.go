package service_test

import (
	"context"
	"testing"
	"time"

	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/trips-service/internal/service/interfaces"
	tripErrors "github.com/Kpeewu/tissi-mah/services/trips-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func validRecurringInput(driverID, vehicleID string) *serviceInterfaces.CreateRecurringTripInput {
	now := time.Now().UTC()
	return &serviceInterfaces.CreateRecurringTripInput{
		DriverID:              driverID,
		VehicleID:             vehicleID,
		DepartureTime:         now.Add(24 * time.Hour).Format(time.RFC3339),
		RecurrenceType:        "weekly",
		DaysOfWeek:            []int32{1, 3, 5},
		StartDate:             now.Add(24 * time.Hour).Format("2006-01-02"),
		EndDate:               now.Add(30 * 24 * time.Hour).Format("2006-01-02"),
		TotalSeats:            4,
		GenerationHorizonDays: 7,
		Waypoints: []serviceInterfaces.WaypointInput{
			{SequencerOrder: 1, WaypointType: "departure", LocationName: "Lomé", City: "Lomé", Country: "TG"},
			{SequencerOrder: 2, WaypointType: "arrival", LocationName: "Kpalimé", City: "Kpalimé", Country: "TG", ScheduledDatetime: now.Add(26 * time.Hour).Format(time.RFC3339), PriceFromPrevious: 1000},
		},
	}
}

func TestCreateRecurringTrip(t *testing.T) {
	t.Run("succès weekly", func(t *testing.T) {
		_, writeRepo, userClient, vehicleClient, svc := newTestService()
		ctx := context.Background()
		userClient.On("GetUserIDByAuthID", ctx, "driver-1").Return("driver-1", nil)
		userClient.On("IsVerifiedDriver", ctx, "driver-1").Return(true, nil)
		vehicleClient.On("GetVehicleInfo", ctx, "driver-1", "vehicle-1").
			Return("Toyota", "AB1234", 5, true, nil)
		writeRepo.On("CreateRecurringPattern", ctx, mock.Anything, mock.Anything).Return("pattern-1", nil)

		id, err := svc.CreateRecurringTrip(ctx, validRecurringInput("driver-1", "vehicle-1"))
		require.NoError(t, err)
		assert.Equal(t, "pattern-1", id)
	})

	t.Run("succès daily (days_of_week ignoré)", func(t *testing.T) {
		_, writeRepo, userClient, vehicleClient, svc := newTestService()
		ctx := context.Background()
		userClient.On("GetUserIDByAuthID", ctx, "driver-1").Return("driver-1", nil)
		userClient.On("IsVerifiedDriver", ctx, "driver-1").Return(true, nil)
		vehicleClient.On("GetVehicleInfo", ctx, "driver-1", "vehicle-1").
			Return("Toyota", "AB1234", 5, true, nil)
		writeRepo.On("CreateRecurringPattern", ctx, mock.Anything, mock.Anything).Return("pattern-daily", nil)

		input := validRecurringInput("driver-1", "vehicle-1")
		input.RecurrenceType = "daily"
		input.DaysOfWeek = nil

		id, err := svc.CreateRecurringTrip(ctx, input)
		require.NoError(t, err)
		assert.Equal(t, "pattern-daily", id)
	})

	t.Run("erreur - driverID vide → Unauthorized", func(t *testing.T) {
		_, _, userClient, _, svc := newTestService()
		ctx := context.Background()
		userClient.On("GetUserIDByAuthID", ctx, "").Return("", nil).Maybe()

		_, err := svc.CreateRecurringTrip(ctx, validRecurringInput("", "vehicle-1"))
		assert.ErrorIs(t, err, tripErrors.ErrorUnauthorized)
	})

	t.Run("erreur - RecurrenceType invalide → InvalidInput", func(t *testing.T) {
		_, _, userClient, _, svc := newTestService()
		ctx := context.Background()
		userClient.On("GetUserIDByAuthID", ctx, "driver-1").Return("driver-1", nil)

		input := validRecurringInput("driver-1", "vehicle-1")
		input.RecurrenceType = "hourly"

		_, err := svc.CreateRecurringTrip(ctx, input)
		assert.ErrorIs(t, err, tripErrors.ErrorInvalidInput)
	})

	t.Run("erreur - weekly sans days_of_week → InvalidInput", func(t *testing.T) {
		_, _, userClient, _, svc := newTestService()
		ctx := context.Background()
		userClient.On("GetUserIDByAuthID", ctx, "driver-1").Return("driver-1", nil)

		input := validRecurringInput("driver-1", "vehicle-1")
		input.DaysOfWeek = nil

		_, err := svc.CreateRecurringTrip(ctx, input)
		assert.ErrorIs(t, err, tripErrors.ErrorInvalidInput)
	})

	t.Run("erreur - < 2 waypoints → InvalidWaypoints", func(t *testing.T) {
		_, _, userClient, _, svc := newTestService()
		ctx := context.Background()
		userClient.On("GetUserIDByAuthID", ctx, "driver-1").Return("driver-1", nil)

		input := validRecurringInput("driver-1", "vehicle-1")
		input.Waypoints = input.Waypoints[:1]

		_, err := svc.CreateRecurringTrip(ctx, input)
		assert.ErrorIs(t, err, tripErrors.ErrorInvalidWaypoints)
	})

	t.Run("erreur - driver non vérifié → propagation", func(t *testing.T) {
		_, _, userClient, _, svc := newTestService()
		ctx := context.Background()
		userClient.On("GetUserIDByAuthID", ctx, "driver-1").Return("driver-1", nil)
		userClient.On("IsVerifiedDriver", ctx, "driver-1").Return(false, nil)

		_, err := svc.CreateRecurringTrip(ctx, validRecurringInput("driver-1", "vehicle-1"))
		assert.ErrorIs(t, err, tripErrors.ErrorDriverNotVerified)
	})

	t.Run("erreur - EndDate < StartDate → InvalidDatetime", func(t *testing.T) {
		_, _, userClient, _, svc := newTestService()
		ctx := context.Background()
		userClient.On("GetUserIDByAuthID", ctx, "driver-1").Return("driver-1", nil)

		input := validRecurringInput("driver-1", "vehicle-1")
		now := time.Now().UTC()
		input.StartDate = now.Add(10 * 24 * time.Hour).Format("2006-01-02")
		input.EndDate = now.Add(5 * 24 * time.Hour).Format("2006-01-02")

		_, err := svc.CreateRecurringTrip(ctx, input)
		assert.ErrorIs(t, err, tripErrors.ErrorInvalidDatetime)
	})

	t.Run("erreur - DepartureTime mal formé → InvalidDatetime", func(t *testing.T) {
		_, _, userClient, _, svc := newTestService()
		ctx := context.Background()
		userClient.On("GetUserIDByAuthID", ctx, "driver-1").Return("driver-1", nil)

		input := validRecurringInput("driver-1", "vehicle-1")
		input.DepartureTime = "not-a-date"

		_, err := svc.CreateRecurringTrip(ctx, input)
		assert.ErrorIs(t, err, tripErrors.ErrorInvalidDatetime)
	})
}
