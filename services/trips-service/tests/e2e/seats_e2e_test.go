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

func TestE2E_IncrementLegBookedSeats_Success(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, mockVehicleClient, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-incr-seats"
	vehicleID := "vehicle-incr-seats"

	stubAuthResolve(mockUserClient, driverID)
	mockUserClient.On("IsVerifiedDriver", mock.Anything, driverID).Return(true, nil)
	mockVehicleClient.On("GetVehicleInfo", mock.Anything, driverID, vehicleID).
		Return("Toyota", "AA-1234", 4, true, nil).Maybe()
	createResp, err := client.CreateTrip(ctx, validCreateTripRequest(driverID, vehicleID))
	require.NoError(t, err)

	resp, err := client.IncrementLegBookedSeats(ctx, &trippb.IncrementLegBookedSeatsRequest{
		TripId: createResp.TripId, FromOrder: 1, ToOrder: 2, Delta: 1,
	})
	require.NoError(t, err)
	assert.True(t, resp.Success)

	// Décrémenter pour libérer un siège.
	resp, err = client.IncrementLegBookedSeats(ctx, &trippb.IncrementLegBookedSeatsRequest{
		TripId: createResp.TripId, FromOrder: 1, ToOrder: 2, Delta: -1,
	})
	require.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestE2E_IncrementLegBookedSeats_InvalidRange(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, _, _, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	_, err := client.IncrementLegBookedSeats(ctx, &trippb.IncrementLegBookedSeatsRequest{
		TripId: "some-trip", FromOrder: 5, ToOrder: 2, Delta: 1,
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestE2E_IncrementLegBookedSeats_ZeroDelta(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, _, _, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	_, err := client.IncrementLegBookedSeats(ctx, &trippb.IncrementLegBookedSeatsRequest{
		TripId: "some-trip", FromOrder: 1, ToOrder: 2, Delta: 0,
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestE2E_SyncLegBookedSeats_Success(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, mockVehicleClient, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-sync-seats"
	vehicleID := "vehicle-sync-seats"

	stubAuthResolve(mockUserClient, driverID)
	mockUserClient.On("IsVerifiedDriver", mock.Anything, driverID).Return(true, nil)
	mockVehicleClient.On("GetVehicleInfo", mock.Anything, driverID, vehicleID).
		Return("Toyota", "AA-1234", 4, true, nil).Maybe()
	createResp, err := client.CreateTrip(ctx, validCreateTripRequest(driverID, vehicleID))
	require.NoError(t, err)

	resp, err := client.SyncLegBookedSeats(ctx, &trippb.SyncLegBookedSeatsRequest{
		TripId: createResp.TripId,
		Legs: []*trippb.LegBookedSeatsEntry{
			{SequencerOrder: 1, BookedSeats: 2},
		},
	})
	require.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestE2E_SyncLegBookedSeats_EmptyLegs(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, _, _, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	_, err := client.SyncLegBookedSeats(ctx, &trippb.SyncLegBookedSeatsRequest{
		TripId: "some-trip", Legs: nil,
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}
