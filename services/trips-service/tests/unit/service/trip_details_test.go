package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/service"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/trips-service/internal/service/interfaces"
	tripErrors "github.com/Kpeewu/tissi-mah/services/trips-service/pkg/errors"
	"github.com/Kpeewu/tissi-mah/services/trips-service/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
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
		vehicleClient.On("GetVehicleInfo", ctx, "driver-1", "vehicle-1").Return("Toyota", "Corolla", "AA-1234", 4, true, nil)

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
		vehicleClient.On("GetVehicleInfo", ctx, "driver-1", "vehicle-1").
			Return("Toyota", "Corolla", "AA-1234", 4, true, nil).Maybe()
		userClient.On("GetDriverInfo", ctx, "driver-1").Return("Jean", "https://img", nil).Maybe()
		userClient.On("GetDriverName", ctx, "driver-1").Return("Jean", nil).Maybe()

		res, err := svc.GetPassengerTripDetails(ctx, &serviceInterfaces.GetPassengerTripDetailsInput{TripID: "trip-1"})
		require.NoError(t, err)
		assert.Equal(t, "trip-1", res.TripID)
	})

	// Sans coordonnées, le client place les marqueurs de la carte à (0, 0), au large
	// du Golfe de Guinée : la carte du détail restait vide (constaté le 23/09).
	t.Run("succès - les étapes portent leurs coordonnées", func(t *testing.T) {
		readRepo, _, userClient, vehicleClient, svc := newTestService()
		ctx := context.Background()
		trip, wps := buildTripWithWaypoints("trip-1", "driver-1", "vehicle-1")
		wps[0].LocationLat, wps[0].LocationLng = 6.1319, 1.2228
		wps[1].LocationLat, wps[1].LocationLng = 8.9834, 1.1437
		readRepo.On("GetTripByID", ctx, "trip-1").Return(trip, wps, nil)
		vehicleClient.On("GetVehicleInfo", ctx, "driver-1", "vehicle-1").
			Return("Toyota", "Corolla", "AA-1234", 4, true, nil).Maybe()
		userClient.On("GetDriverInfo", ctx, "driver-1").Return("Jean", "https://img", nil).Maybe()

		res, err := svc.GetPassengerTripDetails(ctx, &serviceInterfaces.GetPassengerTripDetailsInput{TripID: "trip-1"})

		require.NoError(t, err)
		require.Len(t, res.Waypoints, 2)
		assert.Equal(t, 6.1319, res.Waypoints[0].LocationLat)
		assert.Equal(t, 1.2228, res.Waypoints[0].LocationLng)
		assert.Equal(t, 8.9834, res.Waypoints[1].LocationLat)
		assert.Equal(t, 1.1437, res.Waypoints[1].LocationLng)
	})

	// L'écran de détail passager affiche le modèle du véhicule, le nombre de notes
	// du conducteur et indique si la réservation est confirmée automatiquement.
	// Ces trois champs ont été ajoutés après un audit de conformité avec le client
	// mobile, qui les attendait déjà.
	t.Run("succès - modèle du véhicule, nombre de notes et approbation automatique", func(t *testing.T) {
		readRepo := new(mocks.MockTripRepositoryRead)
		writeRepo := new(mocks.MockTripRepositoryWrite)
		userClient := new(mocks.MockUserClient)
		vehicleClient := new(mocks.MockVehicleClient)
		ratingClient := new(mocks.MockRatingClient)
		svc := service.NewTripService(
			readRepo, writeRepo, userClient, vehicleClient, nil, ratingClient, nil, nil, zap.NewNop(),
		)

		ctx := context.Background()
		trip, wps := buildTripWithWaypoints("trip-1", "driver-1", "vehicle-1")
		trip.AutoApproveEnabled = true

		readRepo.On("GetTripByID", ctx, "trip-1").Return(trip, wps, nil)
		vehicleClient.On("GetVehicleInfo", ctx, "driver-1", "vehicle-1").
			Return("Toyota", "Corolla", "AA-1234", 4, true, nil)
		userClient.On("GetDriverInfo", ctx, "driver-1").Return("Jean", "https://img", nil)
		ratingClient.On("GetDriverRatingAverage", ctx, "driver-1").Return(4.5, 12, nil)

		res, err := svc.GetPassengerTripDetails(ctx, &serviceInterfaces.GetPassengerTripDetailsInput{TripID: "trip-1"})
		require.NoError(t, err)
		assert.Equal(t, "Toyota", res.VehicleBrand)
		assert.Equal(t, "Corolla", res.VehicleModel)
		assert.Equal(t, 4.5, res.DriverRatingAverage)
		assert.Equal(t, int32(12), res.DriverRatingsCount)
		assert.True(t, res.AutoApproveEnabled)
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
