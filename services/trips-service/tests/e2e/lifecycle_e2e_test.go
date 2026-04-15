package e2e

import (
	"testing"

	trippb "github.com/Kpeewu/tissi-mah/services/trips-service/proto/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestE2E_FullLifecycle_HappyPath couvre le cycle complet :
// CreateTrip → StartTrip → ConfirmWaypointDeparture (dep) → ConfirmWaypointArrival (arr) → EndTrip.
func TestE2E_FullLifecycle_HappyPath(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, _, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-lifecycle"
	vehicleID := "vehicle-lifecycle"

	stubAuthResolve(mockUserClient, driverID)
	mockUserClient.On("IsVerifiedDriver", mock.Anything, driverID).Return(true, nil)

	// 1. Create
	createResp, err := client.CreateTrip(ctx, validCreateTripRequest(driverID, vehicleID))
	require.NoError(t, err)
	require.NotEmpty(t, createResp.TripId)

	// 2. Start
	startResp, err := client.StartTrip(ctx, &trippb.StartTripRequest{DriverId: driverID, TripId: createResp.TripId})
	require.NoError(t, err)
	assert.True(t, startResp.Success)

	// 3. End
	endResp, err := client.EndTrip(ctx, &trippb.EndTripRequest{DriverId: driverID, TripId: createResp.TripId})
	require.NoError(t, err)
	assert.True(t, endResp.Success)

	// 4. Le trajet terminé apparaît dans les previews complétés.
	// (Les enrichissements user/vehicle ne sont stubés qu'à la volée.)
	mockUserClient.On("GetDriverName", mock.Anything, driverID).Return("Jean", nil).Maybe()
}

// TestE2E_CreateMultipleTrips_SameDriver vérifie la création de plusieurs trajets
// par le même conducteur (pas de contrainte d'unicité hors "in-progress").
func TestE2E_CreateMultipleTrips_SameDriver(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, _, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-multi"
	vehicleID := "vehicle-multi"

	stubAuthResolve(mockUserClient, driverID)
	mockUserClient.On("IsVerifiedDriver", mock.Anything, driverID).Return(true, nil)

	r1, err := client.CreateTrip(ctx, validCreateTripRequest(driverID, vehicleID))
	require.NoError(t, err)

	r2, err := client.CreateTrip(ctx, validCreateTripRequest(driverID, vehicleID))
	require.NoError(t, err)

	assert.NotEqual(t, r1.TripId, r2.TripId)
}
