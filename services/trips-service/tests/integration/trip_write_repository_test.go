package integration

import (
	"context"
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/trips-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/domain"
	tripErrors "github.com/Kpeewu/tissi-mah/services/trips-service/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// TestTripWriteRepository_Create
// =============================================================================

func TestTripWriteRepository_Create(t *testing.T) {
	ctx := context.Background()

	t.Run("succès - crée le trajet et ses waypoints", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		trip := fixtures.NewTestTrip()
		dep := fixtures.NewTestDepartureWaypoint(trip.TripID)
		arr := fixtures.NewTestArrivalWaypoint(trip.TripID)

		tripID, err := repo.Create(ctx, trip, []*domain.Waypoint{dep, arr})

		require.NoError(t, err)
		assert.NotEmpty(t, tripID)
		assert.Equal(t, trip.TripID, tripID)
	})

	t.Run("erreur - TripID dupliqué → erreur interne", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		trip := fixtures.NewTestTrip()
		dep := fixtures.NewTestDepartureWaypoint(trip.TripID)
		arr := fixtures.NewTestArrivalWaypoint(trip.TripID)

		// Première insertion
		_, err := repo.Create(ctx, trip, []*domain.Waypoint{dep, arr})
		require.NoError(t, err)

		// Deuxième insertion avec le même tripID — doit échouer
		dep2 := fixtures.NewTestDepartureWaypoint(trip.TripID)
		arr2 := fixtures.NewTestArrivalWaypoint(trip.TripID)
		_, err = repo.Create(ctx, trip, []*domain.Waypoint{dep2, arr2})

		assert.Error(t, err)
	})
}

// =============================================================================
// TestTripWriteRepository_UpdateDepartureDatetime
// =============================================================================

func TestTripWriteRepository_UpdateDepartureDatetime(t *testing.T) {
	ctx := context.Background()

	t.Run("succès - met à jour la date de départ", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		trip := fixtures.NewTestTrip()
		dep := fixtures.NewTestDepartureWaypoint(trip.TripID)
		arr := fixtures.NewTestArrivalWaypoint(trip.TripID)
		_, err := repo.Create(ctx, trip, []*domain.Waypoint{dep, arr})
		require.NoError(t, err)

		newDatetime := trip.DepartureDatetime.Add(30 * time.Minute)
		err = repo.UpdateDepartureDatetime(ctx, trip.TripID, trip.DriverID, newDatetime)

		assert.NoError(t, err)
	})

	t.Run("erreur - trajet non trouvé → ErrorTripNotFound", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		err := repo.UpdateDepartureDatetime(ctx, uuid.New().String(), "driver-1", time.Now().Add(2*time.Hour))

		assert.ErrorIs(t, err, tripErrors.ErrorTripNotFound)
	})

	t.Run("erreur - statut non scheduled → ErrorTripNotScheduled", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		// Insérer un trajet avec statut inProgress
		trip := fixtures.NewTestTrip(fixtures.WithStatus(domain.TripStatusInProgress))
		err := fixtures.InsertTrip(ctx, testPool, trip)
		require.NoError(t, err)
		dep := fixtures.NewTestDepartureWaypoint(trip.TripID)
		arr := fixtures.NewTestArrivalWaypoint(trip.TripID)
		require.NoError(t, fixtures.InsertWaypoint(ctx, testPool, dep))
		require.NoError(t, fixtures.InsertWaypoint(ctx, testPool, arr))

		err = repo.UpdateDepartureDatetime(ctx, trip.TripID, trip.DriverID, time.Now().Add(2*time.Hour))

		assert.ErrorIs(t, err, tripErrors.ErrorTripNotScheduled)
	})
}

// =============================================================================
// TestTripWriteRepository_UpdateAutoApprove
// =============================================================================

func TestTripWriteRepository_UpdateAutoApprove(t *testing.T) {
	ctx := context.Background()

	t.Run("succès - active l'auto approve", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		trip := fixtures.NewTestTrip()
		dep := fixtures.NewTestDepartureWaypoint(trip.TripID)
		arr := fixtures.NewTestArrivalWaypoint(trip.TripID)
		_, err := repo.Create(ctx, trip, []*domain.Waypoint{dep, arr})
		require.NoError(t, err)

		err = repo.UpdateAutoApprove(ctx, trip.TripID, trip.DriverID, true)

		assert.NoError(t, err)
	})

	t.Run("erreur - conducteur non autorisé → ErrorUnauthorized", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		trip := fixtures.NewTestTrip()
		dep := fixtures.NewTestDepartureWaypoint(trip.TripID)
		arr := fixtures.NewTestArrivalWaypoint(trip.TripID)
		_, err := repo.Create(ctx, trip, []*domain.Waypoint{dep, arr})
		require.NoError(t, err)

		// Utiliser un driverID différent
		err = repo.UpdateAutoApprove(ctx, trip.TripID, "autre-driver", true)

		assert.ErrorIs(t, err, tripErrors.ErrorUnauthorized)
	})
}

// =============================================================================
// TestTripWriteRepository_StartTrip
// =============================================================================

func TestTripWriteRepository_StartTrip(t *testing.T) {
	ctx := context.Background()

	t.Run("succès - passe le trajet en inProgress", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		trip := fixtures.NewTestTrip()
		dep := fixtures.NewTestDepartureWaypoint(trip.TripID)
		arr := fixtures.NewTestArrivalWaypoint(trip.TripID)
		_, err := repo.Create(ctx, trip, []*domain.Waypoint{dep, arr})
		require.NoError(t, err)

		err = repo.StartTrip(ctx, trip.TripID, trip.DriverID)

		assert.NoError(t, err)
	})

	t.Run("erreur - trajet non trouvé → ErrorTripNotFound", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		err := repo.StartTrip(ctx, uuid.New().String(), "driver-1")

		assert.ErrorIs(t, err, tripErrors.ErrorTripNotFound)
	})

	t.Run("erreur - conducteur a déjà un trajet actif → ErrorDriverAlreadyHasActiveTrip", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		driverID := uuid.New().String()
		vehicleID := uuid.New().String()

		// Créer et démarrer le premier trajet
		trip1 := fixtures.NewTestTrip(
			fixtures.WithDriverID(driverID),
			fixtures.WithVehicleID(vehicleID),
		)
		dep1 := fixtures.NewTestDepartureWaypoint(trip1.TripID)
		arr1 := fixtures.NewTestArrivalWaypoint(trip1.TripID)
		_, err := repo.Create(ctx, trip1, []*domain.Waypoint{dep1, arr1})
		require.NoError(t, err)
		require.NoError(t, repo.StartTrip(ctx, trip1.TripID, driverID))

		// Créer un deuxième trajet pour le même conducteur
		trip2 := fixtures.NewTestTrip(
			fixtures.WithDriverID(driverID),
			fixtures.WithVehicleID(vehicleID),
		)
		dep2 := fixtures.NewTestDepartureWaypoint(trip2.TripID)
		arr2 := fixtures.NewTestArrivalWaypoint(trip2.TripID)
		_, err = repo.Create(ctx, trip2, []*domain.Waypoint{dep2, arr2})
		require.NoError(t, err)

		// Tenter de démarrer le deuxième trajet
		err = repo.StartTrip(ctx, trip2.TripID, driverID)

		assert.ErrorIs(t, err, tripErrors.ErrorDriverAlreadyHasActiveTrip)
	})
}

// =============================================================================
// TestTripWriteRepository_EndTrip
// =============================================================================

func TestTripWriteRepository_EndTrip(t *testing.T) {
	ctx := context.Background()

	t.Run("succès - passe le trajet en completed", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		trip := fixtures.NewTestTrip()
		dep := fixtures.NewTestDepartureWaypoint(trip.TripID)
		arr := fixtures.NewTestArrivalWaypoint(trip.TripID)
		_, err := repo.Create(ctx, trip, []*domain.Waypoint{dep, arr})
		require.NoError(t, err)
		require.NoError(t, repo.StartTrip(ctx, trip.TripID, trip.DriverID))

		err = repo.EndTrip(ctx, trip.TripID, trip.DriverID)

		assert.NoError(t, err)
	})

	t.Run("erreur - trajet pas inProgress → ErrorTripNotInProgress", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		// Insérer un trajet scheduled (pas inProgress)
		trip := fixtures.NewTestTrip()
		dep := fixtures.NewTestDepartureWaypoint(trip.TripID)
		arr := fixtures.NewTestArrivalWaypoint(trip.TripID)
		_, err := repo.Create(ctx, trip, []*domain.Waypoint{dep, arr})
		require.NoError(t, err)

		err = repo.EndTrip(ctx, trip.TripID, trip.DriverID)

		assert.ErrorIs(t, err, tripErrors.ErrorTripNotInProgress)
	})
}

// =============================================================================
// TestTripWriteRepository_ConfirmWaypointArrival
// =============================================================================

func TestTripWriteRepository_ConfirmWaypointArrival(t *testing.T) {
	ctx := context.Background()

	t.Run("succès - confirme l'arrivée à un stop", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		// Créer un trajet avec départ(1), stop(2), arrivée(3)
		trip := fixtures.NewTestTrip()
		dep := fixtures.NewTestDepartureWaypoint(trip.TripID)
		stop := fixtures.NewTestWaypoint(trip.TripID,
			fixtures.WithWaypointType(domain.WaypointTypeStop),
			fixtures.WithSequencerOrder(2),
		)
		arr := fixtures.NewTestWaypoint(trip.TripID,
			fixtures.WithWaypointType(domain.WaypointTypeArrival),
			fixtures.WithSequencerOrder(3),
		)
		_, err := repo.Create(ctx, trip, []*domain.Waypoint{dep, stop, arr})
		require.NoError(t, err)

		// Démarrer le trajet (cela confirme le waypoint de départ)
		require.NoError(t, repo.StartTrip(ctx, trip.TripID, trip.DriverID))

		// Confirmer l'arrivée au stop
		err = repo.ConfirmWaypointArrival(ctx, stop.WaypointID, trip.DriverID)

		assert.NoError(t, err)
	})

	t.Run("erreur - waypoint introuvable → ErrorWaypointNotFound", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		err := repo.ConfirmWaypointArrival(ctx, uuid.New().String(), "driver-1")

		assert.ErrorIs(t, err, tripErrors.ErrorWaypointNotFound)
	})

	t.Run("erreur - waypoint pas un stop → ErrorWaypointNotAStop", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		trip := fixtures.NewTestTrip()
		dep := fixtures.NewTestDepartureWaypoint(trip.TripID)
		arr := fixtures.NewTestArrivalWaypoint(trip.TripID)
		_, err := repo.Create(ctx, trip, []*domain.Waypoint{dep, arr})
		require.NoError(t, err)
		require.NoError(t, repo.StartTrip(ctx, trip.TripID, trip.DriverID))

		// Tenter de confirmer l'arrivée sur le waypoint de départ (pas un stop)
		err = repo.ConfirmWaypointArrival(ctx, dep.WaypointID, trip.DriverID)

		assert.ErrorIs(t, err, tripErrors.ErrorWaypointNotAStop)
	})

	t.Run("erreur - trajet pas inProgress → ErrorTripNotInProgress", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		// Créer un trajet avec stop mais sans le démarrer
		trip := fixtures.NewTestTrip()
		dep := fixtures.NewTestDepartureWaypoint(trip.TripID)
		stop := fixtures.NewTestWaypoint(trip.TripID,
			fixtures.WithWaypointType(domain.WaypointTypeStop),
			fixtures.WithSequencerOrder(2),
		)
		arr := fixtures.NewTestWaypoint(trip.TripID,
			fixtures.WithWaypointType(domain.WaypointTypeArrival),
			fixtures.WithSequencerOrder(3),
		)
		_, err := repo.Create(ctx, trip, []*domain.Waypoint{dep, stop, arr})
		require.NoError(t, err)

		// Tenter de confirmer sans avoir démarré le trajet
		err = repo.ConfirmWaypointArrival(ctx, stop.WaypointID, trip.DriverID)

		assert.ErrorIs(t, err, tripErrors.ErrorTripNotInProgress)
	})
}

// =============================================================================
// TestTripWriteRepository_ConfirmWaypointDeparture
// =============================================================================

func TestTripWriteRepository_ConfirmWaypointDeparture(t *testing.T) {
	ctx := context.Background()

	t.Run("succès - confirme le départ d'un stop", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		trip := fixtures.NewTestTrip()
		dep := fixtures.NewTestDepartureWaypoint(trip.TripID)
		stop := fixtures.NewTestWaypoint(trip.TripID,
			fixtures.WithWaypointType(domain.WaypointTypeStop),
			fixtures.WithSequencerOrder(2),
		)
		arr := fixtures.NewTestWaypoint(trip.TripID,
			fixtures.WithWaypointType(domain.WaypointTypeArrival),
			fixtures.WithSequencerOrder(3),
		)
		_, err := repo.Create(ctx, trip, []*domain.Waypoint{dep, stop, arr})
		require.NoError(t, err)
		require.NoError(t, repo.StartTrip(ctx, trip.TripID, trip.DriverID))
		require.NoError(t, repo.ConfirmWaypointArrival(ctx, stop.WaypointID, trip.DriverID))

		err = repo.ConfirmWaypointDeparture(ctx, stop.WaypointID, trip.DriverID)

		assert.NoError(t, err)
	})

	t.Run("erreur - arrivée pas encore confirmée → ErrorWaypointNotArrived", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		trip := fixtures.NewTestTrip()
		dep := fixtures.NewTestDepartureWaypoint(trip.TripID)
		stop := fixtures.NewTestWaypoint(trip.TripID,
			fixtures.WithWaypointType(domain.WaypointTypeStop),
			fixtures.WithSequencerOrder(2),
		)
		arr := fixtures.NewTestWaypoint(trip.TripID,
			fixtures.WithWaypointType(domain.WaypointTypeArrival),
			fixtures.WithSequencerOrder(3),
		)
		_, err := repo.Create(ctx, trip, []*domain.Waypoint{dep, stop, arr})
		require.NoError(t, err)
		require.NoError(t, repo.StartTrip(ctx, trip.TripID, trip.DriverID))

		// Tenter le départ sans avoir confirmé l'arrivée au stop
		err = repo.ConfirmWaypointDeparture(ctx, stop.WaypointID, trip.DriverID)

		assert.ErrorIs(t, err, tripErrors.ErrorWaypointNotArrived)
	})
}

// =============================================================================
// TestTripWriteRepository_UpdateVehicle
// =============================================================================

func TestTripWriteRepository_UpdateVehicle(t *testing.T) {
	ctx := context.Background()

	t.Run("succès - change le vehicleID", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		trip := fixtures.NewTestTrip()
		dep := fixtures.NewTestDepartureWaypoint(trip.TripID)
		arr := fixtures.NewTestArrivalWaypoint(trip.TripID)
		_, err := repo.Create(ctx, trip, []*domain.Waypoint{dep, arr})
		require.NoError(t, err)

		newVehicleID := uuid.New().String()
		err = repo.UpdateVehicle(ctx, trip.TripID, trip.DriverID, newVehicleID)

		assert.NoError(t, err)
	})

	t.Run("erreur - trajet non trouvé → ErrorTripNotFound", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		err := repo.UpdateVehicle(ctx, uuid.New().String(), "driver-1", "vehicle-new")

		assert.ErrorIs(t, err, tripErrors.ErrorTripNotFound)
	})

	t.Run("erreur - conducteur non autorisé → ErrorUnauthorized", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		trip := fixtures.NewTestTrip()
		dep := fixtures.NewTestDepartureWaypoint(trip.TripID)
		arr := fixtures.NewTestArrivalWaypoint(trip.TripID)
		_, err := repo.Create(ctx, trip, []*domain.Waypoint{dep, arr})
		require.NoError(t, err)

		err = repo.UpdateVehicle(ctx, trip.TripID, "autre-driver", "vehicle-new")

		assert.ErrorIs(t, err, tripErrors.ErrorUnauthorized)
	})
}

// =============================================================================
// TestTripWriteRepository_UpdateAllowances
// =============================================================================

func TestTripWriteRepository_UpdateAllowances(t *testing.T) {
	ctx := context.Background()

	t.Run("succès - départ > 24h, change les flags", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		// Créer un trajet avec départ dans 25h (> 24h requis)
		trip := fixtures.NewTestTrip()
		trip.DepartureDatetime = time.Now().UTC().Add(25 * time.Hour)
		trip.EstimatedArrivalDatetime = time.Now().UTC().Add(27 * time.Hour)
		require.NoError(t, fixtures.InsertTrip(ctx, testPool, trip))
		dep := fixtures.NewTestDepartureWaypoint(trip.TripID)
		arr := fixtures.NewTestArrivalWaypoint(trip.TripID)
		require.NoError(t, fixtures.InsertWaypoint(ctx, testPool, dep))
		require.NoError(t, fixtures.InsertWaypoint(ctx, testPool, arr))

		err := repo.UpdateAllowances(ctx, trip.TripID, trip.DriverID, true, true, false, true)

		assert.NoError(t, err)
	})

	t.Run("erreur - départ < 24h → ErrorTripDepartureTooSoon", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		// Créer un trajet avec départ dans 1h (< 24h)
		trip := fixtures.NewTestTrip()
		dep := fixtures.NewTestDepartureWaypoint(trip.TripID)
		arr := fixtures.NewTestArrivalWaypoint(trip.TripID)
		_, err := repo.Create(ctx, trip, []*domain.Waypoint{dep, arr})
		require.NoError(t, err)

		err = repo.UpdateAllowances(ctx, trip.TripID, trip.DriverID, true, false, false, false)

		assert.ErrorIs(t, err, tripErrors.ErrorTripDepartureTooSoon)
	})

	t.Run("erreur - trajet non trouvé → ErrorTripNotFound", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		err := repo.UpdateAllowances(ctx, uuid.New().String(), "driver-1", false, false, false, false)

		assert.ErrorIs(t, err, tripErrors.ErrorTripNotFound)
	})
}

// =============================================================================
// TestTripWriteRepository_UpdateAvailableSeats
// =============================================================================

func TestTripWriteRepository_UpdateAvailableSeats(t *testing.T) {
	ctx := context.Background()

	t.Run("succès - met à jour available_seats", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		trip := fixtures.NewTestTrip(fixtures.WithTotalSeats(4))
		dep := fixtures.NewTestDepartureWaypoint(trip.TripID)
		arr := fixtures.NewTestArrivalWaypoint(trip.TripID)
		_, err := repo.Create(ctx, trip, []*domain.Waypoint{dep, arr})
		require.NoError(t, err)

		err = repo.UpdateAvailableSeats(ctx, trip.TripID, 3)

		assert.NoError(t, err)

		// Vérification directe en base
		var seats int16
		require.NoError(t, testPool.QueryRow(ctx,
			"SELECT available_seats FROM trips WHERE trip_id = $1", trip.TripID,
		).Scan(&seats))
		assert.Equal(t, int16(3), seats)
	})
}

// =============================================================================
// TestTripWriteRepository_CancelWaypoint
// =============================================================================

func TestTripWriteRepository_CancelWaypoint(t *testing.T) {
	ctx := context.Background()

	// Helper pour créer un trip scheduled avec dep/stop/arr
	createScheduledTripWithStop := func(t *testing.T, repo interface {
		Create(context.Context, *domain.Trip, []*domain.Waypoint) (string, error)
	}) (*domain.Trip, *domain.Waypoint) {
		t.Helper()
		trip := fixtures.NewTestTrip()
		dep := fixtures.NewTestDepartureWaypoint(trip.TripID)
		stop := fixtures.NewTestWaypoint(trip.TripID,
			fixtures.WithWaypointType(domain.WaypointTypeStop),
			fixtures.WithSequencerOrder(2),
		)
		arr := fixtures.NewTestWaypoint(trip.TripID,
			fixtures.WithWaypointType(domain.WaypointTypeArrival),
			fixtures.WithSequencerOrder(3),
		)
		_, err := repo.Create(ctx, trip, []*domain.Waypoint{dep, stop, arr})
		require.NoError(t, err)
		return trip, stop
	}

	t.Run("succès - annule le stop d'un trip scheduled", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		trip, stop := createScheduledTripWithStop(t, repo)

		err := repo.CancelWaypoint(ctx, stop.WaypointID, trip.DriverID, "route modifiée")

		require.NoError(t, err)

		// Vérification : cancelled_at IS NOT NULL et cancellation_reason correct
		var cancelledAt *time.Time
		var reason *string
		require.NoError(t, testPool.QueryRow(ctx,
			"SELECT cancelled_at, cancellation_reason FROM trips_waypoints WHERE waypoint_id = $1",
			stop.WaypointID,
		).Scan(&cancelledAt, &reason))
		assert.NotNil(t, cancelledAt)
		require.NotNil(t, reason)
		assert.Equal(t, "route modifiée", *reason)
	})

	t.Run("erreur - waypoint de type departure → ErrorWaypointNotAStop", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		trip := fixtures.NewTestTrip()
		dep := fixtures.NewTestDepartureWaypoint(trip.TripID)
		arr := fixtures.NewTestArrivalWaypoint(trip.TripID)
		_, err := repo.Create(ctx, trip, []*domain.Waypoint{dep, arr})
		require.NoError(t, err)

		err = repo.CancelWaypoint(ctx, dep.WaypointID, trip.DriverID, "raison")

		assert.ErrorIs(t, err, tripErrors.ErrorWaypointNotAStop)
	})

	t.Run("erreur - waypoint de type arrival → ErrorWaypointNotAStop", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		trip := fixtures.NewTestTrip()
		dep := fixtures.NewTestDepartureWaypoint(trip.TripID)
		arr := fixtures.NewTestArrivalWaypoint(trip.TripID)
		_, err := repo.Create(ctx, trip, []*domain.Waypoint{dep, arr})
		require.NoError(t, err)

		err = repo.CancelWaypoint(ctx, arr.WaypointID, trip.DriverID, "raison")

		assert.ErrorIs(t, err, tripErrors.ErrorWaypointNotAStop)
	})

	t.Run("erreur - trip inProgress (après StartTrip) → ErrorTripNotScheduled", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		trip, stop := createScheduledTripWithStop(t, repo)
		require.NoError(t, repo.StartTrip(ctx, trip.TripID, trip.DriverID))

		err := repo.CancelWaypoint(ctx, stop.WaypointID, trip.DriverID, "raison")

		assert.ErrorIs(t, err, tripErrors.ErrorTripNotScheduled)
	})

	t.Run("erreur - driverID différent → ErrorUnauthorized", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		_, stop := createScheduledTripWithStop(t, repo)

		err := repo.CancelWaypoint(ctx, stop.WaypointID, "wrong-driver", "raison")

		assert.ErrorIs(t, err, tripErrors.ErrorUnauthorized)
	})

	t.Run("erreur - appel en double → ErrorWaypointAlreadyCancelled", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		trip, stop := createScheduledTripWithStop(t, repo)
		require.NoError(t, repo.CancelWaypoint(ctx, stop.WaypointID, trip.DriverID, "raison"))

		err := repo.CancelWaypoint(ctx, stop.WaypointID, trip.DriverID, "raison")

		assert.ErrorIs(t, err, tripErrors.ErrorWaypointAlreadyCancelled)
	})

	t.Run("erreur - waypointID inexistant → ErrorWaypointNotFound", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		repo := newTestWriteRepository()

		err := repo.CancelWaypoint(ctx, uuid.New().String(), "driver-1", "raison")

		assert.ErrorIs(t, err, tripErrors.ErrorWaypointNotFound)
	})
}
