package e2e

import (
	"context"
	"testing"

	trippb "github.com/Kpeewu/tissi-mah/services/trips-service/proto/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestE2E_GetDriverTripDetails_Success(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, mockVehicleClient, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-details-ok"
	vehicleID := "vehicle-details-ok"

	stubAuthResolve(mockUserClient, driverID)
	mockUserClient.On("IsVerifiedDriver", mock.Anything, driverID).Return(true, nil)
	mockVehicleClient.On("GetVehicleInfo", mock.Anything, driverID, vehicleID).
		Return("Toyota", "AA-1234", 4, true, nil).Maybe()
	createResp, err := client.CreateTrip(ctx, validCreateTripRequest(driverID, vehicleID))
	require.NoError(t, err)

	resp, err := client.GetDriverTripDetails(ctx, &trippb.GetDriverTripDetailsRequest{TripId: createResp.TripId})
	require.NoError(t, err)
	assert.Equal(t, createResp.TripId, resp.TripId)
	assert.Equal(t, driverID, resp.DriverId)
	assert.Len(t, resp.Waypoints, 2)
}

func TestE2E_GetDriverTripDetails_NotOwner(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, mockVehicleClient, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	ownerDriverID := "driver-details-owner"
	vehicleID := "vehicle-details-owner"

	stubAuthResolve(mockUserClient, ownerDriverID)
	mockUserClient.On("IsVerifiedDriver", mock.Anything, ownerDriverID).Return(true, nil)
	mockVehicleClient.On("GetVehicleInfo", mock.Anything, ownerDriverID, vehicleID).
		Return("Toyota", "AA-1234", 4, true, nil).Maybe()
	createResp, err := client.CreateTrip(ctx, validCreateTripRequest(ownerDriverID, vehicleID))
	require.NoError(t, err)

	// Changer la résolution — l'appelant n'est plus propriétaire
	stubAuthResolve(mockUserClient, "driver-other")

	_, err = client.GetDriverTripDetails(ctx, &trippb.GetDriverTripDetailsRequest{TripId: createResp.TripId})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}

func TestE2E_GetDriverTripDetails_TripNotFound(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, _, _, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	_, err := client.GetDriverTripDetails(ctx, &trippb.GetDriverTripDetailsRequest{TripId: "00000000-0000-0000-0000-000000000000"})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestE2E_GetPassengerTripDetails_Success(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, mockVehicleClient, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-pax-details"
	vehicleID := "vehicle-pax-details"

	stubAuthResolve(mockUserClient, driverID)
	mockUserClient.On("IsVerifiedDriver", mock.Anything, driverID).Return(true, nil)
	mockVehicleClient.On("GetVehicleInfo", mock.Anything, driverID, vehicleID).
		Return("Toyota", "AA-1234", 4, true, nil).Maybe()
	createResp, err := client.CreateTrip(ctx, validCreateTripRequest(driverID, vehicleID))
	require.NoError(t, err)

	mockUserClient.On("GetDriverInfo", mock.Anything, driverID).Return("Jean", "https://img", nil).Maybe()
	mockUserClient.On("GetDriverName", mock.Anything, driverID).Return("Jean Test", nil).Maybe()
	mockVehicleClient.On("GetVehicleInfo", mock.Anything, driverID, vehicleID).
		Return("Toyota", "AA-1234", 4, true, nil).Maybe()

	// Route passager = publique, pas besoin d'UID metadata
	resp, err := client.GetPassengerTripDetails(context.Background(), &trippb.GetPassengerTripDetailsRequest{TripId: createResp.TripId})
	require.NoError(t, err)
	assert.Equal(t, createResp.TripId, resp.TripId)
}

func TestE2E_GetPassengerTripDetails_EmptyTripID(t *testing.T) {
	conn, _, _, cleanup := setupServer(t)
	defer cleanup()

	client := trippb.NewTripServiceClient(conn)
	_, err := client.GetPassengerTripDetails(context.Background(), &trippb.GetPassengerTripDetailsRequest{TripId: ""})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestE2E_GetScheduledTripsPreviews_Empty(t *testing.T) {
	conn, _, _, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, context.Background())

	client := trippb.NewTripServiceClient(conn)
	resp, err := client.GetScheduledTripsPreviews(context.Background(), &trippb.GetScheduledTripsPreviewsRequest{
		DepartureLocationName: "Lomé",
		ArrivalLocationName:   "Kpalimé",
	})
	require.NoError(t, err)
	assert.Empty(t, resp.TripsPreviews)
}
