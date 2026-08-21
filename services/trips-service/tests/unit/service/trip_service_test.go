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
	// Par défaut, résout authID -> userID en retournant l'authID (identity).
	// Les tests peuvent surcharger avec .On("GetUserIDByAuthID", ...) explicite.
	userClient.On("GetUserIDByAuthID", mock.Anything, mock.AnythingOfType("string")).
		Return(func(_ context.Context, authID string) string { return authID }, nil).Maybe()
	svc := service.NewTripService(readRepo, writeRepo, userClient, vehicleClient, nil, nil, nil, nil, zap.NewNop())
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
		Waypoints: []serviceInterfaces.WaypointInput{
			{
				SequencerOrder:    1,
				WaypointType:      "departure",
				LocationName:      "Lomé",
				LocationLng:       1.2228,
				LocationLat:       6.1375,
				City:              "Lomé",
				Country:           "TG",
				PriceFromPrevious: 0,
			},
			{
				SequencerOrder:    2,
				WaypointType:      "arrival",
				LocationName:      "Kpalimé",
				LocationLng:       0.6370,
				LocationLat:       6.8999,
				City:              "Kpalimé",
				Country:           "TG",
				PriceFromPrevious: 1000,
			},
		},
	}
}

// =============================================================================
// TestCreateTrip
// =============================================================================

func TestCreateTrip(t *testing.T) {
	t.Run("succès - crée le trajet et retourne le tripID", func(t *testing.T) {
		_, writeRepo, userClient, vehicleClient, svc := newTestService()
		ctx := context.Background()

		userClient.On("GetUserIDByAuthID", ctx, "driver-1").Return("driver-1", nil)
		userClient.On("IsVerifiedDriver", ctx, "driver-1").Return(true, nil)
		vehicleClient.On("GetVehicleInfo", ctx, "driver-1", "vehicle-1").
			Return("Toyota", "AB1234", 5, true, nil)
		writeRepo.On("Create", ctx, mock.Anything, mock.Anything).Return("new-trip-id", nil)

		trip, err := svc.CreateTrip(ctx, validCreateTripInput("driver-1", "vehicle-1"))

		require.NoError(t, err)
		require.NotNil(t, trip)
		assert.Equal(t, "new-trip-id", trip.TripID)
		userClient.AssertExpectations(t)
		writeRepo.AssertExpectations(t)
	})

	t.Run("erreur - driverID vide → ErrorUnauthorized", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		ctx := context.Background()

		trip, err := svc.CreateTrip(ctx, validCreateTripInput("", "vehicle-1"))

		assert.Nil(t, trip)
		assert.ErrorIs(t, err, tripErrors.ErrorUnauthorized)
	})

	t.Run("erreur - moins de 2 waypoints → ErrorInvalidWaypoints", func(t *testing.T) {
		_, _, userClient, _, svc := newTestService()
		ctx := context.Background()

		userClient.On("GetUserIDByAuthID", ctx, "driver-1").Return("driver-1", nil)

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

		userClient.On("GetUserIDByAuthID", ctx, "driver-unverified").Return("driver-unverified", nil)
		userClient.On("IsVerifiedDriver", ctx, "driver-unverified").Return(false, nil)

		trip, err := svc.CreateTrip(ctx, validCreateTripInput("driver-unverified", "vehicle-1"))

		assert.Nil(t, trip)
		assert.ErrorIs(t, err, tripErrors.ErrorDriverNotVerified)
		userClient.AssertExpectations(t)
	})

	t.Run("erreur - writeRepo.Create échoue → retourne l'erreur", func(t *testing.T) {
		_, writeRepo, userClient, vehicleClient, svc := newTestService()
		ctx := context.Background()

		userClient.On("GetUserIDByAuthID", ctx, "driver-1").Return("driver-1", nil)
		userClient.On("IsVerifiedDriver", ctx, "driver-1").Return(true, nil)
		vehicleClient.On("GetVehicleInfo", ctx, "driver-1", "vehicle-1").
			Return("Toyota", "AB1234", 5, true, nil)
		writeRepo.On("Create", ctx, mock.Anything, mock.Anything).Return("", tripErrors.ErrorInternalServer)

		trip, err := svc.CreateTrip(ctx, validCreateTripInput("driver-1", "vehicle-1"))

		assert.Nil(t, trip)
		assert.ErrorIs(t, err, tripErrors.ErrorInternalServer)
		writeRepo.AssertExpectations(t)
	})

	t.Run("erreur - véhicule non vérifié → ErrorVehicleNotVerified", func(t *testing.T) {
		_, writeRepo, userClient, vehicleClient, svc := newTestService()
		ctx := context.Background()

		userClient.On("GetUserIDByAuthID", ctx, "driver-1").Return("driver-1", nil)
		userClient.On("IsVerifiedDriver", ctx, "driver-1").Return(true, nil)
		// Véhicule existant mais documents pas tous approuvés par le support.
		vehicleClient.On("GetVehicleInfo", ctx, "driver-1", "vehicle-1").
			Return("Toyota", "AB1234", 5, false, nil)

		trip, err := svc.CreateTrip(ctx, validCreateTripInput("driver-1", "vehicle-1"))

		assert.Nil(t, trip)
		assert.ErrorIs(t, err, tripErrors.ErrorVehicleNotVerified)
		writeRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("erreur - véhicule inconnu ou d'un autre conducteur → ErrorVehicleNotFound", func(t *testing.T) {
		_, writeRepo, userClient, vehicleClient, svc := newTestService()
		ctx := context.Background()

		userClient.On("GetUserIDByAuthID", ctx, "driver-1").Return("driver-1", nil)
		userClient.On("IsVerifiedDriver", ctx, "driver-1").Return(true, nil)
		vehicleClient.On("GetVehicleInfo", ctx, "driver-1", "vehicle-ghost").
			Return("", "", 0, false, nil)

		trip, err := svc.CreateTrip(ctx, validCreateTripInput("driver-1", "vehicle-ghost"))

		assert.Nil(t, trip)
		assert.ErrorIs(t, err, tripErrors.ErrorVehicleNotFound)
		writeRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
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
			Return("Toyota", "AB1234", 5, true, nil)
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
			Return("", "", 0, false, nil)

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
			Return("Suzuki", "XY9999", 2, true, nil)
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

	t.Run("erreur - driverID vide → ErrorUnauthorized", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		ctx := context.Background()

		err := svc.StartTrip(ctx, &serviceInterfaces.StartTripInput{
			DriverID: "",
			TripID:   "trip-1",
		})

		assert.ErrorIs(t, err, tripErrors.ErrorUnauthorized)
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

// =============================================================================
// TestChangeTripDateAndTime
// =============================================================================

func TestChangeTripDateAndTime(t *testing.T) {
	t.Run("succès - met à jour la date/heure de départ", func(t *testing.T) {
		_, writeRepo, _, _, svc := newTestService()
		ctx := context.Background()
		newDatetime := time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339)

		writeRepo.On("UpdateDepartureDatetime", ctx, "trip-1", "driver-1", mock.AnythingOfType("time.Time")).Return(nil)

		err := svc.ChangeTripDateAndTime(ctx, &serviceInterfaces.ChangeTripDateAndTimeInput{
			DriverID:          "driver-1",
			TripID:            "trip-1",
			DepartureDatetime: newDatetime,
		})

		require.NoError(t, err)
		writeRepo.AssertExpectations(t)
	})

	t.Run("erreur - driverID vide → ErrorUnauthorized", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		ctx := context.Background()

		err := svc.ChangeTripDateAndTime(ctx, &serviceInterfaces.ChangeTripDateAndTimeInput{
			DriverID:          "",
			TripID:            "trip-1",
			DepartureDatetime: time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339),
		})

		assert.ErrorIs(t, err, tripErrors.ErrorUnauthorized)
	})

	t.Run("erreur - datetime invalide → ErrorInvalidDatetime", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		ctx := context.Background()

		err := svc.ChangeTripDateAndTime(ctx, &serviceInterfaces.ChangeTripDateAndTimeInput{
			DriverID:          "driver-1",
			TripID:            "trip-1",
			DepartureDatetime: "not-a-date",
		})

		assert.ErrorIs(t, err, tripErrors.ErrorInvalidDatetime)
	})

	t.Run("erreur - repo: ErrorTripNotFound", func(t *testing.T) {
		_, writeRepo, _, _, svc := newTestService()
		ctx := context.Background()
		newDatetime := time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339)

		writeRepo.On("UpdateDepartureDatetime", ctx, "trip-ghost", "driver-1", mock.AnythingOfType("time.Time")).
			Return(tripErrors.ErrorTripNotFound)

		err := svc.ChangeTripDateAndTime(ctx, &serviceInterfaces.ChangeTripDateAndTimeInput{
			DriverID:          "driver-1",
			TripID:            "trip-ghost",
			DepartureDatetime: newDatetime,
		})

		assert.ErrorIs(t, err, tripErrors.ErrorTripNotFound)
		writeRepo.AssertExpectations(t)
	})
}

// =============================================================================
// TestChangeTripAllowances
// =============================================================================

func TestChangeTripAllowances(t *testing.T) {
	t.Run("succès - met à jour les autorisations", func(t *testing.T) {
		_, writeRepo, _, _, svc := newTestService()
		ctx := context.Background()

		writeRepo.On("UpdateAllowances", ctx, "trip-1", "driver-1", true, false, false, true).Return(nil)

		err := svc.ChangeTripAllowances(ctx, &serviceInterfaces.ChangeTripAllowancesInput{
			DriverID:      "driver-1",
			TripID:        "trip-1",
			AllowPets:     true,
			AllowFood:     false,
			AllowSmoking:  false,
			AllowLuggages: true,
		})

		require.NoError(t, err)
		writeRepo.AssertExpectations(t)
	})

	t.Run("erreur - tripID vide → ErrorInvalidInput", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		ctx := context.Background()

		err := svc.ChangeTripAllowances(ctx, &serviceInterfaces.ChangeTripAllowancesInput{
			DriverID: "driver-1",
			TripID:   "",
		})

		assert.ErrorIs(t, err, tripErrors.ErrorInvalidInput)
	})

	t.Run("erreur - repo: ErrorTripDepartureTooSoon", func(t *testing.T) {
		_, writeRepo, _, _, svc := newTestService()
		ctx := context.Background()

		writeRepo.On("UpdateAllowances", ctx, "trip-1", "driver-1", false, false, false, false).
			Return(tripErrors.ErrorTripDepartureTooSoon)

		err := svc.ChangeTripAllowances(ctx, &serviceInterfaces.ChangeTripAllowancesInput{
			DriverID: "driver-1",
			TripID:   "trip-1",
		})

		assert.ErrorIs(t, err, tripErrors.ErrorTripDepartureTooSoon)
		writeRepo.AssertExpectations(t)
	})
}

// =============================================================================
// TestChangeAutoApprove
// =============================================================================

func TestChangeAutoApprove(t *testing.T) {
	t.Run("succès - active l'approbation automatique", func(t *testing.T) {
		_, writeRepo, _, _, svc := newTestService()
		ctx := context.Background()

		writeRepo.On("UpdateAutoApprove", ctx, "trip-1", "driver-1", true).Return(nil)

		err := svc.ChangeAutoApprove(ctx, &serviceInterfaces.ChangeAutoApproveInput{
			DriverID:    "driver-1",
			TripID:      "trip-1",
			AutoApprove: true,
		})

		require.NoError(t, err)
		writeRepo.AssertExpectations(t)
	})

	t.Run("erreur - tripID vide → ErrorInvalidInput", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		ctx := context.Background()

		err := svc.ChangeAutoApprove(ctx, &serviceInterfaces.ChangeAutoApproveInput{
			DriverID: "driver-1",
			TripID:   "",
		})

		assert.ErrorIs(t, err, tripErrors.ErrorInvalidInput)
	})
}

// =============================================================================
// TestGetTripByID
// =============================================================================

func TestGetTripByID(t *testing.T) {
	t.Run("succès - retourne TripDetailResult avec waypoints", func(t *testing.T) {
		readRepo, _, _, vehicleClient, svc := newTestService()
		ctx := context.Background()

		trip := &domain.Trip{
			TripID:     "trip-1",
			DriverID:   "driver-1",
			Status:     domain.TripStatusScheduled,
			TotalSeats: 4,
		}
		waypoints := []*domain.Waypoint{
			{WaypointID: "wp-1", TripID: "trip-1", WaypointType: domain.WaypointTypeDeparture, SequencerOrder: 1, LocationName: "Lomé"},
			{WaypointID: "wp-2", TripID: "trip-1", WaypointType: domain.WaypointTypeArrival, SequencerOrder: 2, LocationName: "Kpalimé"},
		}

		readRepo.On("GetTripByID", ctx, "trip-1").Return(trip, waypoints, nil)
		vehicleClient.On("GetVehicleInfo", mock.Anything, "driver-1", mock.Anything).Return("", "", 0, false, nil).Maybe()

		result, err := svc.GetTripByID(ctx, &serviceInterfaces.GetTripByIDInput{TripID: "trip-1"})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "trip-1", result.TripID)
		assert.Equal(t, "driver-1", result.DriverID)
		assert.Len(t, result.Waypoints, 2)
		assert.Equal(t, "wp-1", result.Waypoints[0].WaypointID)
		readRepo.AssertExpectations(t)
	})

	t.Run("erreur - tripID vide → ErrorInvalidInput", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		ctx := context.Background()

		result, err := svc.GetTripByID(ctx, &serviceInterfaces.GetTripByIDInput{TripID: ""})

		assert.Nil(t, result)
		assert.ErrorIs(t, err, tripErrors.ErrorInvalidInput)
	})

	t.Run("erreur - repo: ErrorTripNotFound", func(t *testing.T) {
		readRepo, _, _, _, svc := newTestService()
		ctx := context.Background()

		readRepo.On("GetTripByID", ctx, "trip-ghost").Return(nil, nil, tripErrors.ErrorTripNotFound)

		result, err := svc.GetTripByID(ctx, &serviceInterfaces.GetTripByIDInput{TripID: "trip-ghost"})

		assert.Nil(t, result)
		assert.ErrorIs(t, err, tripErrors.ErrorTripNotFound)
		readRepo.AssertExpectations(t)
	})
}

// =============================================================================
// TestUpdateAvailableSeats
// =============================================================================

func TestUpdateAvailableSeats(t *testing.T) {
	t.Run("succès - met à jour les places disponibles", func(t *testing.T) {
		_, writeRepo, _, _, svc := newTestService()
		ctx := context.Background()

		writeRepo.On("UpdateAvailableSeats", ctx, "trip-1", int16(3)).Return(nil)

		err := svc.UpdateAvailableSeats(ctx, &serviceInterfaces.UpdateAvailableSeatsInput{
			TripID:            "trip-1",
			NewAvailableSeats: 3,
		})

		require.NoError(t, err)
		writeRepo.AssertExpectations(t)
	})

	t.Run("erreur - tripID vide → ErrorInvalidInput", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		ctx := context.Background()

		err := svc.UpdateAvailableSeats(ctx, &serviceInterfaces.UpdateAvailableSeatsInput{
			TripID:            "",
			NewAvailableSeats: 3,
		})

		assert.ErrorIs(t, err, tripErrors.ErrorInvalidInput)
	})

	t.Run("erreur - newAvailableSeats négatif → ErrorInvalidInput", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		ctx := context.Background()

		err := svc.UpdateAvailableSeats(ctx, &serviceInterfaces.UpdateAvailableSeatsInput{
			TripID:            "trip-1",
			NewAvailableSeats: -1,
		})

		assert.ErrorIs(t, err, tripErrors.ErrorInvalidInput)
	})
}

// =============================================================================
// TestGetTripsPreviews
// =============================================================================

func TestGetTripsPreviews(t *testing.T) {
	t.Run("liste vide - retourne slice vide sans erreur", func(t *testing.T) {
		readRepo, _, _, _, svc := newTestService()
		ctx := context.Background()

		readRepo.On("GetDriverTripsPreviews", ctx, "driver-1", 0).Return([]*domain.TripPreview{}, nil)

		results, err := svc.GetTripsPreviews(ctx, &serviceInterfaces.GetTripsPreviewsInput{
			DriverID:  "driver-1",
			PageIndex: 0,
		})

		require.NoError(t, err)
		assert.Empty(t, results)
		readRepo.AssertExpectations(t)
	})

	t.Run("1 trajet - enrichi avec driverName et vehicleInfo", func(t *testing.T) {
		readRepo, _, userClient, vehicleClient, svc := newTestService()
		ctx := context.Background()

		previews := []*domain.TripPreview{
			{
				TripID:                "trip-1",
				DriverID:              "driver-1",
				VehicleID:             "vehicle-1",
				DepartureLocationName: "Lomé",
				ArrivalLocationName:   "Kpalimé",
				TotalSeats:            4,
				AvailableSeats:        3,
			},
		}

		readRepo.On("GetDriverTripsPreviews", ctx, "driver-1", 0).Return(previews, nil)
		userClient.On("GetDriverName", ctx, "driver-1").Return("Kwame Mensah", nil)
		vehicleClient.On("GetVehicleInfo", ctx, "driver-1", "vehicle-1").Return("Toyota", "TG-1234", 5, true, nil)

		results, err := svc.GetTripsPreviews(ctx, &serviceInterfaces.GetTripsPreviewsInput{
			DriverID:  "driver-1",
			PageIndex: 0,
		})

		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, "Kwame Mensah", results[0].DriverName)
		assert.Equal(t, "Toyota", results[0].VehicleBrand)
		assert.Equal(t, "TG-1234", results[0].VehiclePlate)
		assert.Equal(t, "Lomé", results[0].DepartureLocationName)
		readRepo.AssertExpectations(t)
		userClient.AssertExpectations(t)
		vehicleClient.AssertExpectations(t)
	})

	t.Run("erreur - driverID vide → ErrorUnauthorized", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		ctx := context.Background()

		results, err := svc.GetTripsPreviews(ctx, &serviceInterfaces.GetTripsPreviewsInput{DriverID: ""})

		assert.Nil(t, results)
		assert.ErrorIs(t, err, tripErrors.ErrorUnauthorized)
	})
}

// =============================================================================
// TestCancelWaypoint
// =============================================================================

func TestCancelWaypoint(t *testing.T) {
	t.Run("succès - annule le waypoint", func(t *testing.T) {
		readRepo, writeRepo, _, _, svc := newTestService()
		ctx := context.Background()

		readRepo.On("GetTripIDByWaypointID", ctx, "waypoint-1").Return("trip-1", nil)
		writeRepo.On("CancelWaypoint", ctx, "waypoint-1", "driver-1", "route modifiée").Return(nil)

		err := svc.CancelWaypoint(ctx, &serviceInterfaces.CancelWaypointInput{
			DriverID:           "driver-1",
			WaypointID:         "waypoint-1",
			CancellationReason: "route modifiée",
		})

		require.NoError(t, err)
		writeRepo.AssertExpectations(t)
	})

	t.Run("erreur - driverID vide → ErrorUnauthorized", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		ctx := context.Background()

		err := svc.CancelWaypoint(ctx, &serviceInterfaces.CancelWaypointInput{
			DriverID:           "",
			WaypointID:         "waypoint-1",
			CancellationReason: "route modifiée",
		})

		assert.ErrorIs(t, err, tripErrors.ErrorUnauthorized)
	})

	t.Run("erreur - waypointID vide → ErrorInvalidInput", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		ctx := context.Background()

		err := svc.CancelWaypoint(ctx, &serviceInterfaces.CancelWaypointInput{
			DriverID:           "driver-1",
			WaypointID:         "",
			CancellationReason: "route modifiée",
		})

		assert.ErrorIs(t, err, tripErrors.ErrorInvalidInput)
	})

	t.Run("erreur - cancellationReason vide → ErrorInvalidInput", func(t *testing.T) {
		_, _, _, _, svc := newTestService()
		ctx := context.Background()

		err := svc.CancelWaypoint(ctx, &serviceInterfaces.CancelWaypointInput{
			DriverID:           "driver-1",
			WaypointID:         "waypoint-1",
			CancellationReason: "",
		})

		assert.ErrorIs(t, err, tripErrors.ErrorInvalidInput)
	})

	t.Run("erreur - repo renvoie ErrorWaypointNotFound → propagé", func(t *testing.T) {
		readRepo, writeRepo, _, _, svc := newTestService()
		ctx := context.Background()

		readRepo.On("GetTripIDByWaypointID", ctx, "waypoint-ghost").Return("trip-1", nil)
		writeRepo.On("CancelWaypoint", ctx, "waypoint-ghost", "driver-1", "raison").
			Return(tripErrors.ErrorWaypointNotFound)

		err := svc.CancelWaypoint(ctx, &serviceInterfaces.CancelWaypointInput{
			DriverID:           "driver-1",
			WaypointID:         "waypoint-ghost",
			CancellationReason: "raison",
		})

		assert.ErrorIs(t, err, tripErrors.ErrorWaypointNotFound)
		writeRepo.AssertExpectations(t)
	})

	t.Run("erreur - repo renvoie ErrorUnauthorized → propagé", func(t *testing.T) {
		readRepo, writeRepo, _, _, svc := newTestService()
		ctx := context.Background()

		readRepo.On("GetTripIDByWaypointID", ctx, "waypoint-1").Return("trip-1", nil)
		writeRepo.On("CancelWaypoint", ctx, "waypoint-1", "wrong-driver", "raison").
			Return(tripErrors.ErrorUnauthorized)

		err := svc.CancelWaypoint(ctx, &serviceInterfaces.CancelWaypointInput{
			DriverID:           "wrong-driver",
			WaypointID:         "waypoint-1",
			CancellationReason: "raison",
		})

		assert.ErrorIs(t, err, tripErrors.ErrorUnauthorized)
		writeRepo.AssertExpectations(t)
	})

	t.Run("erreur - repo renvoie ErrorTripNotScheduled → propagé", func(t *testing.T) {
		readRepo, writeRepo, _, _, svc := newTestService()
		ctx := context.Background()

		readRepo.On("GetTripIDByWaypointID", ctx, "waypoint-1").Return("trip-1", nil)
		writeRepo.On("CancelWaypoint", ctx, "waypoint-1", "driver-1", "raison").
			Return(tripErrors.ErrorTripNotScheduled)

		err := svc.CancelWaypoint(ctx, &serviceInterfaces.CancelWaypointInput{
			DriverID:           "driver-1",
			WaypointID:         "waypoint-1",
			CancellationReason: "raison",
		})

		assert.ErrorIs(t, err, tripErrors.ErrorTripNotScheduled)
		writeRepo.AssertExpectations(t)
	})

	t.Run("erreur - repo renvoie ErrorWaypointNotAStop → propagé", func(t *testing.T) {
		readRepo, writeRepo, _, _, svc := newTestService()
		ctx := context.Background()

		readRepo.On("GetTripIDByWaypointID", ctx, "waypoint-dep").Return("trip-1", nil)
		writeRepo.On("CancelWaypoint", ctx, "waypoint-dep", "driver-1", "raison").
			Return(tripErrors.ErrorWaypointNotAStop)

		err := svc.CancelWaypoint(ctx, &serviceInterfaces.CancelWaypointInput{
			DriverID:           "driver-1",
			WaypointID:         "waypoint-dep",
			CancellationReason: "raison",
		})

		assert.ErrorIs(t, err, tripErrors.ErrorWaypointNotAStop)
		writeRepo.AssertExpectations(t)
	})

	t.Run("erreur - repo renvoie ErrorWaypointAlreadyCancelled → propagé", func(t *testing.T) {
		readRepo, writeRepo, _, _, svc := newTestService()
		ctx := context.Background()

		readRepo.On("GetTripIDByWaypointID", ctx, "waypoint-1").Return("trip-1", nil)
		writeRepo.On("CancelWaypoint", ctx, "waypoint-1", "driver-1", "raison").
			Return(tripErrors.ErrorWaypointAlreadyCancelled)

		err := svc.CancelWaypoint(ctx, &serviceInterfaces.CancelWaypointInput{
			DriverID:           "driver-1",
			WaypointID:         "waypoint-1",
			CancellationReason: "raison",
		})

		assert.ErrorIs(t, err, tripErrors.ErrorWaypointAlreadyCancelled)
		writeRepo.AssertExpectations(t)
	})
}
