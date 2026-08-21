package e2e

import (
	"testing"

	trippb "github.com/Kpeewu/tissi-mah/services/trips-service/proto/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestE2E_CancelTrip_Success(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, mockVehicleClient, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-cancel-ok"
	vehicleID := "vehicle-cancel-ok"

	stubAuthResolve(mockUserClient, driverID)
	mockUserClient.On("IsVerifiedDriver", mock.Anything, driverID).Return(true, nil)
	mockVehicleClient.On("GetVehicleInfo", mock.Anything, driverID, vehicleID).
		Return("Toyota", "AA-1234", 4, true, nil).Maybe()

	createResp, err := client.CreateTrip(ctx, validCreateTripRequest(driverID, vehicleID))
	require.NoError(t, err)

	resp, err := client.CancelTrip(ctx, &trippb.CancelTripRequest{
		DriverId:           driverID,
		TripId:             createResp.TripId,
		CancellationReason: "weather",
	})
	require.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestE2E_CancelTrip_MissingReason(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, _, _, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)

	_, err := client.CancelTrip(ctx, &trippb.CancelTripRequest{
		DriverId: "d", TripId: "t", CancellationReason: "",
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestE2E_CancelTrip_NotFound(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, _, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	stubAuthResolve(mockUserClient, "any-driver")

	_, err := client.CancelTrip(ctx, &trippb.CancelTripRequest{
		DriverId:           "any-driver",
		TripId:             "00000000-0000-0000-0000-000000000000",
		CancellationReason: "test",
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestE2E_CancelTrip_AlreadyCancelled(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, mockVehicleClient, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-cancel-twice"
	vehicleID := "vehicle-cancel-twice"

	stubAuthResolve(mockUserClient, driverID)
	mockUserClient.On("IsVerifiedDriver", mock.Anything, driverID).Return(true, nil)
	mockVehicleClient.On("GetVehicleInfo", mock.Anything, driverID, vehicleID).
		Return("Toyota", "AA-1234", 4, true, nil).Maybe()

	createResp, err := client.CreateTrip(ctx, validCreateTripRequest(driverID, vehicleID))
	require.NoError(t, err)

	_, err = client.CancelTrip(ctx, &trippb.CancelTripRequest{
		DriverId: driverID, TripId: createResp.TripId, CancellationReason: "first",
	})
	require.NoError(t, err)

	// Deuxième annulation → erreur
	_, err = client.CancelTrip(ctx, &trippb.CancelTripRequest{
		DriverId: driverID, TripId: createResp.TripId, CancellationReason: "second",
	})
	require.Error(t, err)
}
