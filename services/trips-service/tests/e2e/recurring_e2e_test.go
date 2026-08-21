package e2e

import (
	"testing"
	"time"

	trippb "github.com/Kpeewu/tissi-mah/services/trips-service/proto/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// validRecurringRequest construit une requête récurrente hebdomadaire valide.
func validRecurringRequest(driverID, vehicleID string) *trippb.CreateRecurringTripRequest {
	now := time.Now().UTC()
	return &trippb.CreateRecurringTripRequest{
		DriverId:              driverID,
		VehicleId:             vehicleID,
		DepartureTime:         now.Add(24 * time.Hour).Format(time.RFC3339),
		RecurrenceType:        "weekly",
		DaysOfWeek:            &trippb.DaysOfWeekInput{Days: []int32{1, 3, 5}},
		StartDate:             now.Add(24 * time.Hour).Format("2006-01-02"),
		EndDate:               now.Add(30 * 24 * time.Hour).Format("2006-01-02"),
		TotalSeats:            4,
		PricePerSeat:          1000,
		GenerationHorizonDays: 7,
		TripWaypoints: []*trippb.WaypointInput{
			{SequencerOrder: 1, WaypointType: "departure", LocationName: "Lomé", LocationLng: 1.2228, LocationLat: 6.1375, City: "Lomé", Country: "TG"},
			{SequencerOrder: 2, WaypointType: "arrival", LocationName: "Kpalimé", LocationLng: 0.6370, LocationLat: 6.8999, City: "Kpalimé", Country: "TG", ScheduledDatetime: now.Add(26 * time.Hour).Format(time.RFC3339)},
		},
	}
}

func TestE2E_CreateRecurringTrip_WeeklySuccess(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, mockVehicleClient, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-recurring-ok"
	vehicleID := "vehicle-recurring-ok"

	stubAuthResolve(mockUserClient, driverID)
	mockUserClient.On("IsVerifiedDriver", mock.Anything, driverID).Return(true, nil)
	mockVehicleClient.On("GetVehicleInfo", mock.Anything, driverID, vehicleID).
		Return("Toyota", "AA-1234", 4, true, nil).Maybe()

	resp, err := client.CreateRecurringTrip(ctx, validRecurringRequest(driverID, vehicleID))
	require.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestE2E_CreateRecurringTrip_UnverifiedDriver(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, _, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-recurring-unverified"

	stubAuthResolve(mockUserClient, driverID)
	mockUserClient.On("IsVerifiedDriver", mock.Anything, driverID).Return(false, nil)

	_, err := client.CreateRecurringTrip(ctx, validRecurringRequest(driverID, "vehicle-x"))
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}

func TestE2E_CreateRecurringTrip_InvalidRecurrenceType(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, _, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-recurring-bad-type"

	stubAuthResolve(mockUserClient, driverID)

	req := validRecurringRequest(driverID, "vehicle-x")
	req.RecurrenceType = "hourly" // invalide

	_, err := client.CreateRecurringTrip(ctx, req)
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestE2E_CreateRecurringTrip_TooFewWaypoints(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, _, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-recurring-short-wp"

	stubAuthResolve(mockUserClient, driverID)

	req := validRecurringRequest(driverID, "vehicle-x")
	req.TripWaypoints = req.TripWaypoints[:1] // 1 seul waypoint

	_, err := client.CreateRecurringTrip(ctx, req)
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestE2E_CreateRecurringTrip_EndBeforeStart(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, _, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-recurring-dates"

	stubAuthResolve(mockUserClient, driverID)

	now := time.Now().UTC()
	req := validRecurringRequest(driverID, "vehicle-x")
	req.StartDate = now.Add(10 * 24 * time.Hour).Format("2006-01-02")
	req.EndDate = now.Add(5 * 24 * time.Hour).Format("2006-01-02")

	_, err := client.CreateRecurringTrip(ctx, req)
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}
