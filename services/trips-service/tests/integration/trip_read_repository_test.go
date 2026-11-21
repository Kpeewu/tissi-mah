package integration

import (
	"context"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/trips-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/domain"
	tripErrors "github.com/Kpeewu/tissi-mah/services/trips-service/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// TestTripReadRepository_GetTripByID
// =============================================================================

func TestTripReadRepository_GetTripByID(t *testing.T) {
	ctx := context.Background()

	t.Run("succès - retourne trip + 2 waypoints", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		readRepo := newTestReadRepository()
		writeRepo := newTestWriteRepository()

		trip := fixtures.NewTestTrip()
		dep := fixtures.NewTestDepartureWaypoint(trip.TripID)
		arr := fixtures.NewTestArrivalWaypoint(trip.TripID)
		_, err := writeRepo.Create(ctx, trip, []*domain.Waypoint{dep, arr})
		require.NoError(t, err)

		gotTrip, gotWaypoints, err := readRepo.GetTripByID(ctx, trip.TripID)

		require.NoError(t, err)
		require.NotNil(t, gotTrip)
		assert.Equal(t, trip.TripID, gotTrip.TripID)
		assert.Equal(t, trip.DriverID, gotTrip.DriverID)
		assert.Len(t, gotWaypoints, 2)
	})

	t.Run("erreur - trajet non trouvé → ErrorTripNotFound", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		readRepo := newTestReadRepository()

		gotTrip, gotWaypoints, err := readRepo.GetTripByID(ctx, uuid.New().String())

		assert.Nil(t, gotTrip)
		assert.Nil(t, gotWaypoints)
		assert.ErrorIs(t, err, tripErrors.ErrorTripNotFound)
	})
}

// =============================================================================
// TestTripReadRepository_GetDriverTripsPreviews
// =============================================================================

func TestTripReadRepository_GetDriverTripsPreviews(t *testing.T) {
	ctx := context.Background()

	t.Run("liste vide pour driver sans trajets", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		readRepo := newTestReadRepository()

		previews, err := readRepo.GetDriverTripsPreviews(ctx, uuid.New().String(), 0)

		require.NoError(t, err)
		assert.Empty(t, previews)
	})

	t.Run("1 trajet - preview avec locations", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		readRepo := newTestReadRepository()
		writeRepo := newTestWriteRepository()

		trip := fixtures.NewTestTrip()
		dep := fixtures.NewTestDepartureWaypoint(trip.TripID)
		arr := fixtures.NewTestArrivalWaypoint(trip.TripID)
		_, err := writeRepo.Create(ctx, trip, []*domain.Waypoint{dep, arr})
		require.NoError(t, err)

		previews, err := readRepo.GetDriverTripsPreviews(ctx, trip.DriverID, 0)

		require.NoError(t, err)
		require.Len(t, previews, 1)
		assert.Equal(t, trip.TripID, previews[0].TripID)
		assert.NotEmpty(t, previews[0].DepartureLocationName)
		assert.NotEmpty(t, previews[0].ArrivalLocationName)
	})
}

// =============================================================================
// TestTripReadRepository_GetTripTotalSeats
// =============================================================================

func TestTripReadRepository_GetTripTotalSeats(t *testing.T) {
	ctx := context.Background()

	t.Run("succès - retourne le bon TotalSeats", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		readRepo := newTestReadRepository()
		writeRepo := newTestWriteRepository()

		trip := fixtures.NewTestTrip(fixtures.WithTotalSeats(5))
		dep := fixtures.NewTestDepartureWaypoint(trip.TripID)
		arr := fixtures.NewTestArrivalWaypoint(trip.TripID)
		_, err := writeRepo.Create(ctx, trip, []*domain.Waypoint{dep, arr})
		require.NoError(t, err)

		seats, err := readRepo.GetTripTotalSeats(ctx, trip.TripID)

		require.NoError(t, err)
		assert.Equal(t, int16(5), seats)
	})

	t.Run("erreur - trajet non trouvé → ErrorTripNotFound", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		readRepo := newTestReadRepository()

		_, err := readRepo.GetTripTotalSeats(ctx, uuid.New().String())

		assert.ErrorIs(t, err, tripErrors.ErrorTripNotFound)
	})
}

// =============================================================================
// TestTripReadRepository_GetWaypointIDByType
// =============================================================================

func TestTripReadRepository_GetWaypointIDByType(t *testing.T) {
	ctx := context.Background()

	t.Run("succès - retourne l'ID du waypoint departure", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		readRepo := newTestReadRepository()
		writeRepo := newTestWriteRepository()

		trip := fixtures.NewTestTrip()
		dep := fixtures.NewTestDepartureWaypoint(trip.TripID)
		arr := fixtures.NewTestArrivalWaypoint(trip.TripID)
		_, err := writeRepo.Create(ctx, trip, []*domain.Waypoint{dep, arr})
		require.NoError(t, err)

		waypointID, err := readRepo.GetWaypointIDByType(ctx, trip.TripID, "departure")

		require.NoError(t, err)
		assert.Equal(t, dep.WaypointID, waypointID)
	})

	t.Run("succès - retourne l'ID du waypoint arrival", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		readRepo := newTestReadRepository()
		writeRepo := newTestWriteRepository()

		trip := fixtures.NewTestTrip()
		dep := fixtures.NewTestDepartureWaypoint(trip.TripID)
		arr := fixtures.NewTestArrivalWaypoint(trip.TripID)
		_, err := writeRepo.Create(ctx, trip, []*domain.Waypoint{dep, arr})
		require.NoError(t, err)

		waypointID, err := readRepo.GetWaypointIDByType(ctx, trip.TripID, "arrival")

		require.NoError(t, err)
		assert.Equal(t, arr.WaypointID, waypointID)
	})

	t.Run("erreur - tripID inexistant → erreur", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		readRepo := newTestReadRepository()

		_, err := readRepo.GetWaypointIDByType(ctx, uuid.New().String(), "departure")

		assert.Error(t, err)
	})
}

// =============================================================================
// TestTripReadRepository_GetTripIDByWaypointID
// =============================================================================

func TestTripReadRepository_GetTripIDByWaypointID(t *testing.T) {
	ctx := context.Background()

	t.Run("succès - retourne le tripID", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		readRepo := newTestReadRepository()
		writeRepo := newTestWriteRepository()

		trip := fixtures.NewTestTrip()
		dep := fixtures.NewTestDepartureWaypoint(trip.TripID)
		arr := fixtures.NewTestArrivalWaypoint(trip.TripID)
		_, err := writeRepo.Create(ctx, trip, []*domain.Waypoint{dep, arr})
		require.NoError(t, err)

		tripID, err := readRepo.GetTripIDByWaypointID(ctx, dep.WaypointID)

		require.NoError(t, err)
		assert.Equal(t, trip.TripID, tripID)
	})

	t.Run("erreur - waypointID inconnu → erreur", func(t *testing.T) {
		cleanupTripsTable(t, ctx)
		readRepo := newTestReadRepository()

		_, err := readRepo.GetTripIDByWaypointID(ctx, uuid.New().String())

		assert.Error(t, err)
	})
}
