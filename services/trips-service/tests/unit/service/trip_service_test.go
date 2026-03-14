package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/service"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/trips-service/internal/service/interfaces"
	tripErrors "github.com/Kpeewu/tissi-mah/services/trips-service/pkg/errors"
	"github.com/Kpeewu/tissi-mah/services/trips-service/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// =============================================================================
// Helper : création du service avec mocks injectés
// =============================================================================

// newTestService crée les quatre mocks et retourne le service instancié.
func newTestService() (
	*mocks.MockTripRepositoryRead,
	*mocks.MockTripRepositoryWrite,
	*mocks.MockUserClient,
	*mocks.MockVehicleClient,
	serviceInterfaces.TripService,
) {
	readRepo := new(mocks.MockTripRepositoryRead)
	writeRepo := new(mocks.MockTripRepositoryWrite)
	userClient := new(mocks.MockUserClient)
	vehicleClient := new(mocks.MockVehicleClient)
	svc := service.NewTripService(readRepo, writeRepo, userClient, vehicleClient, nil, zap.NewNop())
	return readRepo, writeRepo, userClient, vehicleClient, svc
}

// validCreateTripInput retourne un CreateTripInput valide avec deux waypoints minimaux.
func validCreateTripInput(driverID, vehicleID string) *serviceInterfaces.CreateTripInput {
	now := time.Now().UTC()
	return &serviceInterfaces.CreateTripInput{
		DriverID:                 driverID,
		VehicleID:                vehicleID,
		DepartureDatetime:        now.Add(1 * time.Hour).Format(time.RFC3339),
		EstimatedArrivalDatetime: now.Add(3 * time.Hour).Format(time.RFC3339),
		EstimatedDurationMinutes: 120,
		EstimatedDistanceMeters:  50000,
		TotalSeats:               4,
		PricePerSeat:             1000,
		Waypoints: []serviceInterfaces.WaypointInput{
			{
				SequencerOrder: 1,
				WaypointType:   "departure",
				LocationName:   "Lomé",
				LocationLng:    1.2228,
				LocationLat:    6.1375,
				City:           "Lomé",
				Country:        "TG",
			},
			{
				SequencerOrder: 2,
				WaypointType:   "arrival",
				LocationName:   "Kpalimé",
				LocationLng:    0.6370,
				LocationLat:    6.8999,
				City:           "Kpalimé",
				Country:        "TG",
			},
		},
	}
}

// =============================================================================
// TestCreateTrip
// =============================================================================

func TestCreateTrip(t *testing.T) {
	t.Run("succès - crée le trajet et retourne le tripID", func(t *testing.T) {
		_, writeRepo, userClient, _, svc := newTestService()
		ctx := context.Background()

		userClient.On("IsVerifiedDriver", ctx, "driver-1").Return(true, nil)
		writeRepo.On("Create", ctx, mock.Anything, mock.Anything).Return("new-trip-id", nil)

		trip, err := svc.CreateTrip(ctx, validCreateTripInput("driver-1", "vehicle-1"))

		require.NoError(t, err)
		require.NotNil(t, trip)
		assert.Equal(t, "new-trip-id", trip.TripID)
		userClient.AssertExpectations(t)
		writeRepo.AssertExpectations(t)
	})

	t.Run("erreur - driverID vide → ErrorInvalidInput", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		ctx := context.Background()

		trip, err := svc.CreateTrip(ctx, validCreateTripInput("", "vehicle-1"))

		assert.Nil(t, trip)
		assert.ErrorIs(t, err, tripErrors.ErrorInvalidInput)
	})

	t.Run("erreur - moins de 2 waypoints → ErrorInvalidWaypoints", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		ctx := context.Background()

		input := validCreateTripInput("driver-1", "vehicle-1")
		input.Waypoints = []serviceInterfaces.WaypointInput{
			{SequencerOrder: 1, WaypointType: "departure", LocationName: "Lomé", City: "Lomé", Country: "TG"},
		}

		trip, err := svc.CreateTrip(ctx, input)

		assert.Nil(t, trip)
		assert.ErrorIs(t, err, tripErrors.ErrorInvalidWaypoints)
	})

	t.Run("erreur - conducteur non vérifié → ErrorDriverNotVerified", func(t *testing.T) {
		_, _, userClient, _, svc := newTestService()
		ctx := context.Background()

		userClient.On("IsVerifiedDriver", ctx, "driver-unverified").Return(false, nil)

		trip, err := svc.CreateTrip(ctx, validCreateTripInput("driver-unverified", "vehicle-1"))

		assert.Nil(t, trip)
		assert.ErrorIs(t, err, tripErrors.ErrorDriverNotVerified)
		userClient.AssertExpectations(t)
	})

	t.Run("erreur - writeRepo.Create échoue → retourne l'erreur", func(t *testing.T) {
		_, writeRepo, userClient, _, svc := newTestService()
		ctx := context.Background()

		userClient.On("IsVerifiedDriver", ctx, "driver-1").Return(true, nil)
		writeRepo.On("Create", ctx, mock.Anything, mock.Anything).Return("", tripErrors.ErrorInternalServer)

		trip, err := svc.CreateTrip(ctx, validCreateTripInput("driver-1", "vehicle-1"))

		assert.Nil(t, trip)
		assert.ErrorIs(t, err, tripErrors.ErrorInternalServer)
		writeRepo.AssertExpectations(t)
	})
}

// =============================================================================
// TestChangeTripVehicle
// =============================================================================

func TestChangeTripVehicle(t *testing.T) {
	t.Run("succès - met à jour le véhicule", func(t *testing.T) {
		readRepo, writeRepo, _, vehicleClient, svc := newTestService()
		ctx := context.Background()

		vehicleClient.On("GetVehicleInfo", ctx, "driver-1", "vehicle-2").
			Return("Toyota", "AB1234", 5, nil)
		readRepo.On("GetTripTotalSeats", ctx, "trip-1").Return(int16(4), nil).Once()
		writeRepo.On("UpdateVehicle", ctx, "trip-1", "driver-1", "vehicle-2").Return(nil)

		err := svc.ChangeTripVehicle(ctx, &serviceInterfaces.ChangeTripVehicleInput{
			DriverID:  "driver-1",
			TripID:    "trip-1",
			VehicleID: "vehicle-2",
		})

		require.NoError(t, err)
		vehicleClient.AssertExpectations(t)
		readRepo.AssertExpectations(t)
		writeRepo.AssertExpectations(t)
	})

	t.Run("erreur - véhicule non trouvé → ErrorVehicleNotFound", func(t *testing.T) {
		_, _, _, vehicleClient, svc := newTestService()
		ctx := context.Background()

		// brand vide signifie véhicule non trouvé
		vehicleClient.On("GetVehicleInfo", ctx, "driver-1", "vehicle-ghost").
			Return("", "", 0, nil)

		err := svc.ChangeTripVehicle(ctx, &serviceInterfaces.ChangeTripVehicleInput{
			DriverID:  "driver-1",
			TripID:    "trip-1",
			VehicleID: "vehicle-ghost",
		})

		assert.ErrorIs(t, err, tripErrors.ErrorVehicleNotFound)
		vehicleClient.AssertExpectations(t)
	})

	t.Run("erreur - places insuffisantes → ErrorVehicleInsufficientSeats", func(t *testing.T) {
		readRepo, _, _, vehicleClient, svc := newTestService()
		ctx := context.Background()

		vehicleClient.On("GetVehicleInfo", ctx, "driver-1", "vehicle-small").
			Return("Suzuki", "XY9999", 2, nil)
		// Le trajet a besoin de 4 places, le véhicule n'en a que 2
		readRepo.On("GetTripTotalSeats", ctx, "trip-1").Return(int16(4), nil).Once()

		err := svc.ChangeTripVehicle(ctx, &serviceInterfaces.ChangeTripVehicleInput{
			DriverID:  "driver-1",
			TripID:    "trip-1",
			VehicleID: "vehicle-small",
		})

		assert.ErrorIs(t, err, tripErrors.ErrorVehicleInsufficientSeats)
		vehicleClient.AssertExpectations(t)
		readRepo.AssertExpectations(t)
	})
}

// =============================================================================
// TestStartTrip
// =============================================================================

func TestStartTrip(t *testing.T) {
	t.Run("succès - démarre le trajet", func(t *testing.T) {
		_, writeRepo, _, _, svc := newTestService()
		ctx := context.Background()

		writeRepo.On("StartTrip", ctx, "trip-1", "driver-1").Return(nil)

		err := svc.StartTrip(ctx, &serviceInterfaces.StartTripInput{
			DriverID: "driver-1",
			TripID:   "trip-1",
		})

		require.NoError(t, err)
		writeRepo.AssertExpectations(t)
	})

	t.Run("erreur - driverID vide → ErrorInvalidInput", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		ctx := context.Background()

		err := svc.StartTrip(ctx, &serviceInterfaces.StartTripInput{
			DriverID: "",
			TripID:   "trip-1",
		})

		assert.ErrorIs(t, err, tripErrors.ErrorInvalidInput)
	})

	t.Run("erreur - conducteur a déjà un trajet actif → ErrorDriverAlreadyHasActiveTrip", func(t *testing.T) {
		_, writeRepo, _, _, svc := newTestService()
		ctx := context.Background()

		writeRepo.On("StartTrip", ctx, "trip-1", "driver-1").
			Return(tripErrors.ErrorDriverAlreadyHasActiveTrip)

		err := svc.StartTrip(ctx, &serviceInterfaces.StartTripInput{
			DriverID: "driver-1",
			TripID:   "trip-1",
		})

		assert.ErrorIs(t, err, tripErrors.ErrorDriverAlreadyHasActiveTrip)
		writeRepo.AssertExpectations(t)
	})
}

// =============================================================================
// TestEndTrip
// =============================================================================

func TestEndTrip(t *testing.T) {
	t.Run("succès - termine le trajet", func(t *testing.T) {
		_, writeRepo, _, _, svc := newTestService()
		ctx := context.Background()

		writeRepo.On("EndTrip", ctx, "trip-1", "driver-1").Return(nil)

		err := svc.EndTrip(ctx, &serviceInterfaces.EndTripInput{
			DriverID: "driver-1",
			TripID:   "trip-1",
		})

		require.NoError(t, err)
		writeRepo.AssertExpectations(t)
	})

	t.Run("erreur - tripID vide → ErrorInvalidInput", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		ctx := context.Background()

		err := svc.EndTrip(ctx, &serviceInterfaces.EndTripInput{
			DriverID: "driver-1",
			TripID:   "",
		})

		assert.ErrorIs(t, err, tripErrors.ErrorInvalidInput)
	})

	t.Run("erreur - trajet non en cours → ErrorTripNotInProgress", func(t *testing.T) {
		_, writeRepo, _, _, svc := newTestService()
		ctx := context.Background()

		writeRepo.On("EndTrip", ctx, "trip-1", "driver-1").
			Return(tripErrors.ErrorTripNotInProgress)

		err := svc.EndTrip(ctx, &serviceInterfaces.EndTripInput{
			DriverID: "driver-1",
			TripID:   "trip-1",
		})

		assert.ErrorIs(t, err, tripErrors.ErrorTripNotInProgress)
		writeRepo.AssertExpectations(t)
	})
}

// =============================================================================
// TestConfirmWaypointArrival
// =============================================================================

func TestConfirmWaypointArrival(t *testing.T) {
	t.Run("succès - confirme l'arrivée", func(t *testing.T) {
		_, writeRepo, _, _, svc := newTestService()
		ctx := context.Background()

		writeRepo.On("ConfirmWaypointArrival", ctx, "waypoint-1", "driver-1").Return(nil)

		err := svc.ConfirmWaypointArrival(ctx, &serviceInterfaces.ConfirmWaypointArrivalInput{
			DriverID:   "driver-1",
			WaypointID: "waypoint-1",
		})

		require.NoError(t, err)
		writeRepo.AssertExpectations(t)
	})

	t.Run("erreur - waypointID vide → ErrorInvalidInput", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		ctx := context.Background()

		err := svc.ConfirmWaypointArrival(ctx, &serviceInterfaces.ConfirmWaypointArrivalInput{
			DriverID:   "driver-1",
			WaypointID: "",
		})

		assert.ErrorIs(t, err, tripErrors.ErrorInvalidInput)
	})

	t.Run("erreur - repo renvoie ErrorWaypointNotAStop", func(t *testing.T) {
		_, writeRepo, _, _, svc := newTestService()
		ctx := context.Background()

		writeRepo.On("ConfirmWaypointArrival", ctx, "waypoint-dep", "driver-1").
			Return(tripErrors.ErrorWaypointNotAStop)

		err := svc.ConfirmWaypointArrival(ctx, &serviceInterfaces.ConfirmWaypointArrivalInput{
			DriverID:   "driver-1",
			WaypointID: "waypoint-dep",
		})

		assert.ErrorIs(t, err, tripErrors.ErrorWaypointNotAStop)
		writeRepo.AssertExpectations(t)
	})
}

// =============================================================================
// TestConfirmWaypointDeparture
// =============================================================================

func TestConfirmWaypointDeparture(t *testing.T) {
	t.Run("succès - confirme le départ", func(t *testing.T) {
		_, writeRepo, _, _, svc := newTestService()
		ctx := context.Background()

		writeRepo.On("ConfirmWaypointDeparture", ctx, "waypoint-1", "driver-1").Return(nil)

		err := svc.ConfirmWaypointDeparture(ctx, &serviceInterfaces.ConfirmWaypointDepartureInput{
			DriverID:   "driver-1",
			WaypointID: "waypoint-1",
		})

		require.NoError(t, err)
		writeRepo.AssertExpectations(t)
	})

	t.Run("erreur - waypointID vide → ErrorInvalidInput", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		ctx := context.Background()

		err := svc.ConfirmWaypointDeparture(ctx, &serviceInterfaces.ConfirmWaypointDepartureInput{
			DriverID:   "driver-1",
			WaypointID: "",
		})

		assert.ErrorIs(t, err, tripErrors.ErrorInvalidInput)
	})

	t.Run("erreur - repo renvoie ErrorWaypointNotArrived", func(t *testing.T) {
		_, writeRepo, _, _, svc := newTestService()
		ctx := context.Background()

		writeRepo.On("ConfirmWaypointDeparture", ctx, "waypoint-1", "driver-1").
			Return(tripErrors.ErrorWaypointNotArrived)

		err := svc.ConfirmWaypointDeparture(ctx, &serviceInterfaces.ConfirmWaypointDepartureInput{
			DriverID:   "driver-1",
			WaypointID: "waypoint-1",
		})

		assert.ErrorIs(t, err, tripErrors.ErrorWaypointNotArrived)
		writeRepo.AssertExpectations(t)
	})
}
