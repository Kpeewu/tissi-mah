package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/domain"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/trips-service/internal/service/interfaces"
	tripErrors "github.com/Kpeewu/tissi-mah/services/trips-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildTripWithWaypoints(tripID, driverID, vehicleID string) (*domain.Trip, []*domain.Waypoint) {
	now := time.Now().UTC()
	trip := &domain.Trip{
		TripID:                   tripID,
		DriverID:                 driverID,
		VehicleID:                vehicleID,
		Status:                   domain.TripStatusScheduled,
		TotalSeats:               4,
		AvailableSeats:           4,
		PricePerSeat:             1000,
		DepartureDatetime:        now.Add(1 * time.Hour),
		EstimatedArrivalDatetime: now.Add(3 * time.Hour),
		EstimatedDurationMinutes: 120,
		EstimatedDistanceMeters:  50000,
	}
	waypoints := []*domain.Waypoint{
		{WaypointID: "wp-1", TripID: tripID, SequencerOrder: 1, WaypointType: domain.WaypointTypeDeparture, LocationName: "Lomé", City: "Lomé", Country: "TG"},
		{WaypointID: "wp-2", TripID: tripID, SequencerOrder: 2, WaypointType: domain.WaypointTypeArrival, LocationName: "Kpalimé", City: "Kpalimé", Country: "TG"},
	}
	return trip, waypoints
}

func TestGetDriverTripDetails(t *testing.T) {
	t.Run("succès", func(t *testing.T) {
		readRepo, _, userClient, vehicleClient, svc := newTestService()
		ctx := context.Background()

		trip, wps := buildTripWithWaypoints("trip-1", "driver-1", "vehicle-1")
		readRepo.On("GetTripByID", ctx, "trip-1").Return(trip, wps, nil)
		userClient.On("GetUserIDByAuthID", ctx, "driver-1").Return("driver-1", nil)
		vehicleClient.On("GetVehicleInfo", ctx, "driver-1", "vehicle-1").Return("Toyota", "AA-1234", 4, nil)

		res, err := svc.GetDriverTripDetails(ctx, &serviceInterfaces.GetDriverTripDetailsInput{
			TripID: "trip-1", DriverID: "driver-1",
		})
		require.NoError(t, err)
		assert.Equal(t, "trip-1", res.TripID)
		assert.Equal(t, "Toyota", res.VehicleBrand)
		assert.Len(t, res.Waypoints, 2)
	})

	t.Run("erreur - TripID vide → InvalidInput", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		_, err := svc.GetDriverTripDetails(context.Background(), &serviceInterfaces.GetDriverTripDetailsInput{
			TripID: "", DriverID: "driver-1",
		})
		assert.ErrorIs(t, err, tripErrors.ErrorInvalidInput)
	})

	t.Run("erreur - DriverID vide → InvalidInput", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		_, err := svc.GetDriverTripDetails(context.Background(), &serviceInterfaces.GetDriverTripDetailsInput{
			TripID: "trip-1", DriverID: "",
		})
		assert.ErrorIs(t, err, tripErrors.ErrorInvalidInput)
	})

	t.Run("erreur - trip introuvable", func(t *testing.T) {
		readRepo, _, _, _, svc := newTestService()
		ctx := context.Background()
		readRepo.On("GetTripByID", ctx, "trip-404").Return((*domain.Trip)(nil), []*domain.Waypoint(nil), tripErrors.ErrorTripNotFound)

		_, err := svc.GetDriverTripDetails(ctx, &serviceInterfaces.GetDriverTripDetailsInput{
			TripID: "trip-404", DriverID: "driver-1",
		})
		assert.ErrorIs(t, err, tripErrors.ErrorTripNotFound)
	})

	t.Run("erreur - caller n'est pas propriétaire → Unauthorized", func(t *testing.T) {
		readRepo, _, userClient, _, svc := newTestService()
		ctx := context.Background()
		trip, wps := buildTripWithWaypoints("trip-1", "driver-owner", "vehicle-1")
		readRepo.On("GetTripByID", ctx, "trip-1").Return(trip, wps, nil)
		userClient.On("GetUserIDByAuthID", ctx, "driver-other").Return("driver-other", nil)

		_, err := svc.GetDriverTripDetails(ctx, &serviceInterfaces.GetDriverTripDetailsInput{
			TripID: "trip-1", DriverID: "driver-other",
		})
		assert.ErrorIs(t, err, tripErrors.ErrorUnauthorized)
	})
}

func TestGetPassengerTripDetails(t *testing.T) {
	t.Run("succès", func(t *testing.T) {
		readRepo, _, userClient, vehicleClient, svc := newTestService()
		ctx := context.Background()
		trip, wps := buildTripWithWaypoints("trip-1", "driver-1", "vehicle-1")
		readRepo.On("GetTripByID", ctx, "trip-1").Return(trip, wps, nil)
		vehicleClient.On("GetVehicleInfo", ctx, "driver-1", "vehicle-1").Return("Toyota", "AA-1234", 4, nil).Maybe()
		userClient.On("GetDriverInfo", ctx, "driver-1").Return("Jean", "https://img", nil).Maybe()
		userClient.On("GetDriverName", ctx, "driver-1").Return("Jean", nil).Maybe()

		res, err := svc.GetPassengerTripDetails(ctx, &serviceInterfaces.GetPassengerTripDetailsInput{TripID: "trip-1"})
		require.NoError(t, err)
		assert.Equal(t, "trip-1", res.TripID)
	})

	t.Run("erreur - TripID vide → InvalidInput", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		_, err := svc.GetPassengerTripDetails(context.Background(), &serviceInterfaces.GetPassengerTripDetailsInput{TripID: ""})
		assert.ErrorIs(t, err, tripErrors.ErrorInvalidInput)
	})

	t.Run("erreur - trip introuvable", func(t *testing.T) {
		readRepo, _, _, _, svc := newTestService()
		ctx := context.Background()
		readRepo.On("GetTripByID", ctx, "trip-404").Return((*domain.Trip)(nil), []*domain.Waypoint(nil), tripErrors.ErrorTripNotFound)

		_, err := svc.GetPassengerTripDetails(ctx, &serviceInterfaces.GetPassengerTripDetailsInput{TripID: "trip-404"})
		assert.ErrorIs(t, err, tripErrors.ErrorTripNotFound)
	})
}
