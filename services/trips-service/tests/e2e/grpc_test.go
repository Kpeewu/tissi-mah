package e2e

import (
	"context"
	"log"
	"net"
	"os"
	"testing"
	"time"

	postgresHelper "github.com/Kpeewu/tissi-mah/pkg-test/postgres"
	grpcHandler "github.com/Kpeewu/tissi-mah/services/trips-service/internal/grpc"
	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/middleware"
	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/repository/implementations"
	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/service"
	trippb "github.com/Kpeewu/tissi-mah/services/trips-service/proto/gen"
	"github.com/Kpeewu/tissi-mah/services/trips-service/tests/mocks"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

var (
	testPool    *pgxpool.Pool
	lis         *bufconn.Listener
	authResolve = map[string]string{}
)

// =============================================================================
// TestMain — démarrage de PostgreSQL et du serveur gRPC en mémoire
// =============================================================================

func TestMain(m *testing.M) {
	ctx := context.Background() //nolint:testmain

	// Setup : démarrer PostgreSQL avec les migrations du trips-service
	testPostgres, err := postgresHelper.SetupTestPostgres(ctx, "../../migrations",
		postgresHelper.WithImage("postgis/postgis:16-3.4"),
	)
	if err != nil {
		log.Fatalf("Failed to setup test database: %v", err)
	}
	testPool = testPostgres.Pool

	code := m.Run()

	cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := testPostgres.CleanUp(cleanupCtx); err != nil {
		log.Printf("Failed to cleanup test database: %v", err)
	}

	os.Exit(code)
}

// =============================================================================
// Helpers E2E
// =============================================================================

// setupServer crée un serveur gRPC bufconn avec des mocks pour les clients externes.
// Retourne le serveur, les mocks et une fonction de nettoyage.
func setupServer(t *testing.T) (*grpc.ClientConn, *mocks.MockUserClient, *mocks.MockVehicleClient, func()) {
	t.Helper()
	logger := zap.NewNop()

	// Reset de la map d'auth resolution entre tests.
	for k := range authResolve {
		delete(authResolve, k)
	}

	readRepo := implementations.NewTripReadRepository(testPool, logger)
	writeRepo := implementations.NewTripWriteRepository(testPool, logger)

	mockUserClient := new(mocks.MockUserClient)
	mockVehicleClient := new(mocks.MockVehicleClient)

	// Default stub : résolution authID → userID via map partagée (fallback identity).
	// Permet aux tests de mapper "e2e-test-user" → driverID via stubAuthResolve.
	mockUserClient.On("GetUserIDByAuthID", mock.Anything, mock.AnythingOfType("string")).
		Return(func(_ context.Context, authID string) string {
			if v, ok := authResolve[authID]; ok {
				return v
			}
			return authID
		}, nil).Maybe()

	svc := service.NewTripService(readRepo, writeRepo, mockUserClient, mockVehicleClient, nil, nil, nil, nil, logger)
	handler := grpcHandler.NewTripHandler(svc, logger)

	lis = bufconn.Listen(bufSize)
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(middleware.TripInterceptor(nil)))
	trippb.RegisterTripServiceServer(grpcServer, handler)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Printf("bufconn server stopped: %v", err)
		}
	}()

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	cleanup := func() {
		conn.Close()
		grpcServer.Stop()
		lis.Close()
	}

	return conn, mockUserClient, mockVehicleClient, cleanup
}

// ctxWithUID retourne un contexte avec le Firebase UID dans les metadata gRPC
func ctxWithUID(uid string) context.Context {
	md := metadata.Pairs("x-firebase-uid", uid)
	return metadata.NewOutgoingContext(context.Background(), md)
}

// stubAuthResolve map l'authID "e2e-test-user" vers le driverID interne donné.
// À appeler dans chaque test qui invoque CreateTrip/CreateRecurringTrip.
// Les tests e2e sont séquentiels (pas de t.Parallel), la map partagée est safe.
func stubAuthResolve(_ *mocks.MockUserClient, driverID string) {
	authResolve["e2e-test-user"] = driverID
}

// cleanupTripsE2E vide la table trips pour isoler les tests.
func cleanupTripsE2E(t *testing.T, ctx context.Context) {
	t.Helper()
	_, err := testPool.Exec(ctx, "TRUNCATE trips CASCADE")
	require.NoError(t, err)
}

// validCreateTripRequest retourne une requête CreateTrip valide avec 2 waypoints.
func validCreateTripRequest(driverID, vehicleID string) *trippb.CreateTripRequest {
	now := time.Now().UTC()
	return &trippb.CreateTripRequest{
		DriverId:                 driverID,
		VehicleId:                vehicleID,
		DepartureDatetime:        now.Add(1 * time.Hour).Format(time.RFC3339),
		EstimatedArrivalDatetime: now.Add(3 * time.Hour).Format(time.RFC3339),
		EstimatedDurationMinutes: 120,
		EstimatedDistanceMeters:  50000,
		TotalSeats:               4,
		PricePerSeat:             1000,
		PaymentMethodsAccepted:   []string{"cash"},
		TripWaypoints: []*trippb.WaypointInput{
			{
				SequencerOrder: 1,
				WaypointType:   "departure",
				LocationName:   "Lomé Centre",
				LocationLng:    1.2228,
				LocationLat:    6.1375,
				City:           "Lomé",
				Country:        "TG",
			},
			{
				SequencerOrder: 2,
				WaypointType:   "arrival",
				LocationName:   "Kpalimé Marché",
				LocationLng:    0.6370,
				LocationLat:    6.8999,
				City:           "Kpalimé",
				Country:        "TG",
			},
		},
	}
}

// =============================================================================
// Tests E2E
// =============================================================================

func TestE2E_CreateTrip_Success(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, mockVehicleClient, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-e2e-1"
	vehicleID := "vehicle-e2e-1"

	// Conducteur vérifié
	stubAuthResolve(mockUserClient, driverID)
	mockUserClient.On("IsVerifiedDriver", mock.Anything, driverID).Return(true, nil)
	mockVehicleClient.On("GetVehicleInfo", mock.Anything, driverID, vehicleID).
		Return("Toyota", "AA-1234", 4, true, nil).Maybe()

	resp, err := client.CreateTrip(ctx, validCreateTripRequest(driverID, vehicleID))

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.TripId)
	mockUserClient.AssertExpectations(t)
}

func TestE2E_StartTrip_Success(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, mockVehicleClient, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-e2e-start"
	vehicleID := "vehicle-e2e-start"

	// Créer un trajet
	stubAuthResolve(mockUserClient, driverID)
	mockUserClient.On("IsVerifiedDriver", mock.Anything, driverID).Return(true, nil)
	mockVehicleClient.On("GetVehicleInfo", mock.Anything, driverID, vehicleID).
		Return("Toyota", "AA-1234", 4, true, nil).Maybe()
	createResp, err := client.CreateTrip(ctx, validCreateTripRequest(driverID, vehicleID))
	require.NoError(t, err)
	require.NotEmpty(t, createResp.TripId)

	// Démarrer le trajet
	startResp, err := client.StartTrip(ctx, &trippb.StartTripRequest{
		DriverId: driverID,
		TripId:   createResp.TripId,
	})

	require.NoError(t, err)
	require.NotNil(t, startResp)
	assert.True(t, startResp.Success)
	mockUserClient.AssertExpectations(t)
}

func TestE2E_StartTrip_AlreadyActive(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, mockVehicleClient, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-e2e-active"
	vehicleID := "vehicle-e2e-active"

	stubAuthResolve(mockUserClient, driverID)
	mockUserClient.On("IsVerifiedDriver", mock.Anything, driverID).Return(true, nil)
	mockVehicleClient.On("GetVehicleInfo", mock.Anything, driverID, vehicleID).
		Return("Toyota", "AA-1234", 4, true, nil).Maybe()

	// Créer le premier trajet et le démarrer
	createResp1, err := client.CreateTrip(ctx, validCreateTripRequest(driverID, vehicleID))
	require.NoError(t, err)
	_, err = client.StartTrip(ctx, &trippb.StartTripRequest{
		DriverId: driverID,
		TripId:   createResp1.TripId,
	})
	require.NoError(t, err)

	// Créer un deuxième trajet
	req2 := validCreateTripRequest(driverID, vehicleID)
	// Décaler la date pour éviter la contrainte d'unicité éventuelle
	req2.DepartureDatetime = time.Now().Add(5 * time.Hour).Format(time.RFC3339)
	req2.EstimatedArrivalDatetime = time.Now().Add(7 * time.Hour).Format(time.RFC3339)
	createResp2, err := client.CreateTrip(ctx, req2)
	require.NoError(t, err)

	// Tenter de démarrer le deuxième trajet — doit échouer
	_, err = client.StartTrip(ctx, &trippb.StartTripRequest{
		DriverId: driverID,
		TripId:   createResp2.TripId,
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
	mockUserClient.AssertExpectations(t)
}

func TestE2E_ConfirmWaypointArrival_InvalidWaypointID(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, _, _, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)

	// Utiliser un waypointID qui n'existe pas
	_, err := client.ConfirmWaypointArrival(ctx, &trippb.ConfirmWaypointArrivalRequest{
		DriverId:   "driver-1",
		WaypointId: "waypoint-inexistant-" + time.Now().String(),
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

// =============================================================================
// Health
// =============================================================================

func TestE2E_Health(t *testing.T) {
	conn, _, _, cleanup := setupServer(t)
	defer cleanup()

	client := trippb.NewTripServiceClient(conn)
	resp, err := client.Health(context.Background(), &trippb.HealthRequest{})

	require.NoError(t, err)
	assert.Equal(t, "SERVING", resp.Status)
	assert.NotEmpty(t, resp.Version)
	assert.NotZero(t, resp.Timestamp)
}

// =============================================================================
// CreateTrip — cas d'erreur
// =============================================================================

func TestE2E_CreateTrip_UnverifiedDriver(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, mockVehicleClient, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-unverified"
	vehicleID := "vehicle-unverified"

	// Conducteur non certifié
	stubAuthResolve(mockUserClient, driverID)
	mockUserClient.On("IsVerifiedDriver", mock.Anything, driverID).Return(false, nil)

	mockVehicleClient.On("GetVehicleInfo", mock.Anything, driverID, vehicleID).
		Return("Toyota", "AA-1234", 4, true, nil).Maybe()
	_, err := client.CreateTrip(ctx, validCreateTripRequest(driverID, vehicleID))

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, st.Code())
	mockUserClient.AssertExpectations(t)
}

func TestE2E_CreateTrip_InvalidWaypoints(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, _, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-invalid-wp"
	vehicleID := "vehicle-invalid-wp"

	stubAuthResolve(mockUserClient, driverID)
	mockUserClient.On("IsVerifiedDriver", mock.Anything, driverID).Return(true, nil).Maybe()

	req := validCreateTripRequest(driverID, vehicleID)
	req.TripWaypoints = nil // aucun waypoint

	_, err := client.CreateTrip(ctx, req)

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestE2E_CreateTrip_MissingAuthUID(t *testing.T) {
	// Pas de metadata x-firebase-uid → Unauthenticated (intercepteur ou resolveDriverUserID).
	conn, _, _, cleanup := setupServer(t)
	defer cleanup()

	client := trippb.NewTripServiceClient(conn)

	req := validCreateTripRequest("driver-x", "vehicle-x")
	_, err := client.CreateTrip(context.Background(), req)

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

// =============================================================================
// GetTripsPreviews
// =============================================================================

func TestE2E_GetTripsPreviews_EmptyList(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, _, _, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)

	resp, err := client.GetTripsPreviews(ctx, &trippb.GetTripsPreviewsRequest{
		DriverId: "driver-no-trips",
		Index:    0,
	})

	require.NoError(t, err)
	assert.Empty(t, resp.TripsPreviews)
}

func TestE2E_GetTripsPreviews_WithTrips(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, mockVehicleClient, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-preview-list"
	vehicleID := "vehicle-preview-list"

	// Créer 2 trajets via l'API
	stubAuthResolve(mockUserClient, driverID)
	mockUserClient.On("IsVerifiedDriver", mock.Anything, driverID).Return(true, nil)
	mockVehicleClient.On("GetVehicleInfo", mock.Anything, driverID, vehicleID).
		Return("Toyota", "AA-1234", 4, true, nil)

	req1 := validCreateTripRequest(driverID, vehicleID)
	_, err := client.CreateTrip(ctx, req1)
	require.NoError(t, err)

	req2 := validCreateTripRequest(driverID, vehicleID)
	req2.DepartureDatetime = time.Now().Add(5 * time.Hour).Format(time.RFC3339)
	req2.EstimatedArrivalDatetime = time.Now().Add(7 * time.Hour).Format(time.RFC3339)
	_, err = client.CreateTrip(ctx, req2)
	require.NoError(t, err)

	// Enrichissement : GetDriverName + GetVehicleInfo (1 véhicule unique)
	mockUserClient.On("GetDriverName", mock.Anything, driverID).Return("Jean Test", nil)

	resp, err := client.GetTripsPreviews(ctx, &trippb.GetTripsPreviewsRequest{
		DriverId: driverID,
		Index:    0,
	})

	require.NoError(t, err)
	assert.Len(t, resp.TripsPreviews, 2)
	assert.Equal(t, driverID, resp.TripsPreviews[0].DriverId)
	assert.Equal(t, "Jean Test", resp.TripsPreviews[0].DriverName)
}

// =============================================================================
// GetCompletedTripsPreviews
// =============================================================================

func TestE2E_GetCompletedTripsPreviews(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, mockVehicleClient, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-completed"
	vehicleID := "vehicle-completed"

	// Créer un trajet, le démarrer, puis le terminer
	stubAuthResolve(mockUserClient, driverID)
	mockUserClient.On("IsVerifiedDriver", mock.Anything, driverID).Return(true, nil)

	mockVehicleClient.On("GetVehicleInfo", mock.Anything, driverID, vehicleID).
		Return("Toyota", "AA-1234", 4, true, nil).Maybe()
	createResp, err := client.CreateTrip(ctx, validCreateTripRequest(driverID, vehicleID))
	require.NoError(t, err)
	require.NotEmpty(t, createResp.TripId)

	_, err = client.StartTrip(ctx, &trippb.StartTripRequest{DriverId: driverID, TripId: createResp.TripId})
	require.NoError(t, err)

	_, err = client.EndTrip(ctx, &trippb.EndTripRequest{DriverId: driverID, TripId: createResp.TripId})
	require.NoError(t, err)

	// Enrichissement pour GetCompletedTripsPreviews (1 trajet)
	mockUserClient.On("GetDriverName", mock.Anything, driverID).Return("Jean Test", nil)
	mockVehicleClient.On("GetVehicleInfo", mock.Anything, driverID, vehicleID).
		Return("Toyota", "AA-1234", 4, true, nil)

	// Le trajet doit apparaître dans les trajets complétés
	completed, err := client.GetCompletedTripsPreviews(ctx, &trippb.GetCompletedTripsPreviewsRequest{
		DriverId: driverID,
		Index:    0,
	})
	require.NoError(t, err)
	require.Len(t, completed.TripsPreviews, 1)
	assert.Equal(t, createResp.TripId, completed.TripsPreviews[0].TripId)
	assert.Equal(t, "Jean Test", completed.TripsPreviews[0].DriverName)

	// Et ne doit plus apparaître dans les trajets actifs (liste vide → pas d'enrichissement)
	active, err := client.GetTripsPreviews(ctx, &trippb.GetTripsPreviewsRequest{
		DriverId: driverID,
		Index:    0,
	})
	require.NoError(t, err)
	assert.Empty(t, active.TripsPreviews)

	mockUserClient.AssertExpectations(t)
	mockVehicleClient.AssertExpectations(t)
}

// =============================================================================
// ChangeTripDateAndTime
// =============================================================================

func TestE2E_ChangeTripDateAndTime_Success(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, mockVehicleClient, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-change-date"
	vehicleID := "vehicle-change-date"

	stubAuthResolve(mockUserClient, driverID)
	mockUserClient.On("IsVerifiedDriver", mock.Anything, driverID).Return(true, nil)
	mockVehicleClient.On("GetVehicleInfo", mock.Anything, driverID, vehicleID).
		Return("Toyota", "AA-1234", 4, true, nil).Maybe()
	createResp, err := client.CreateTrip(ctx, validCreateTripRequest(driverID, vehicleID))
	require.NoError(t, err)

	// Nouvelle heure de départ : dans 90 min (toujours avant l'arrivée à 3h)
	newDatetime := time.Now().UTC().Add(90 * time.Minute).Format(time.RFC3339)

	resp, err := client.ChangeTripDateAndTime(ctx, &trippb.ChangeTripDateAndTimeRequest{
		DriverId:          driverID,
		TripId:            createResp.TripId,
		DepartureDatetime: newDatetime,
	})

	require.NoError(t, err)
	assert.True(t, resp.Success)
	mockUserClient.AssertExpectations(t)
}

func TestE2E_ChangeTripDateAndTime_TripNotFound(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, _, _, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)

	_, err := client.ChangeTripDateAndTime(ctx, &trippb.ChangeTripDateAndTimeRequest{
		DriverId:          "driver-x",
		TripId:            "nonexistent-trip-id",
		DepartureDatetime: time.Now().Add(2 * time.Hour).Format(time.RFC3339),
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

// =============================================================================
// ChangeTripVehicle
// =============================================================================

func TestE2E_ChangeTripVehicle_Success(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, mockVehicleClient, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-change-vehicle"
	vehicleID := "vehicle-old"
	newVehicleID := "vehicle-new"

	stubAuthResolve(mockUserClient, driverID)
	mockUserClient.On("IsVerifiedDriver", mock.Anything, driverID).Return(true, nil)
	mockVehicleClient.On("GetVehicleInfo", mock.Anything, driverID, vehicleID).
		Return("Toyota", "AA-1234", 4, true, nil).Maybe()
	createResp, err := client.CreateTrip(ctx, validCreateTripRequest(driverID, vehicleID))
	require.NoError(t, err)

	// Nouveau véhicule avec assez de places (trip a 4 places)
	mockVehicleClient.On("GetVehicleInfo", mock.Anything, driverID, newVehicleID).
		Return("Toyota", "AA-1234", 5, true, nil)

	resp, err := client.ChangeTripVehicle(ctx, &trippb.ChangeTripVehicleRequest{
		DriverId:  driverID,
		TripId:    createResp.TripId,
		VehicleId: newVehicleID,
	})

	require.NoError(t, err)
	assert.True(t, resp.Success)
	mockUserClient.AssertExpectations(t)
	mockVehicleClient.AssertExpectations(t)
}

func TestE2E_ChangeTripVehicle_VehicleNotFound(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, mockVehicleClient, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-change-veh-nf"
	vehicleID := "vehicle-existing"
	badVehicleID := "vehicle-ghost"

	stubAuthResolve(mockUserClient, driverID)
	mockUserClient.On("IsVerifiedDriver", mock.Anything, driverID).Return(true, nil)
	mockVehicleClient.On("GetVehicleInfo", mock.Anything, driverID, vehicleID).
		Return("Toyota", "AA-1234", 4, true, nil).Maybe()
	createResp, err := client.CreateTrip(ctx, validCreateTripRequest(driverID, vehicleID))
	require.NoError(t, err)

	// GetVehicleInfo retourne brand="" → véhicule introuvable
	mockVehicleClient.On("GetVehicleInfo", mock.Anything, driverID, badVehicleID).
		Return("", "", 0, false, nil)

	_, err = client.ChangeTripVehicle(ctx, &trippb.ChangeTripVehicleRequest{
		DriverId:  driverID,
		TripId:    createResp.TripId,
		VehicleId: badVehicleID,
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
	mockVehicleClient.AssertExpectations(t)
}

func TestE2E_ChangeTripVehicle_InsufficientSeats(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, mockVehicleClient, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-insuf-seats"
	vehicleID := "vehicle-enough"
	smallVehicleID := "vehicle-too-small"

	stubAuthResolve(mockUserClient, driverID)
	mockUserClient.On("IsVerifiedDriver", mock.Anything, driverID).Return(true, nil)
	mockVehicleClient.On("GetVehicleInfo", mock.Anything, driverID, vehicleID).
		Return("Toyota", "AA-1234", 4, true, nil).Maybe()
	// Créer un trajet avec 4 places
	createResp, err := client.CreateTrip(ctx, validCreateTripRequest(driverID, vehicleID))
	require.NoError(t, err)

	// Nouveau véhicule avec seulement 2 places — insuffisant
	mockVehicleClient.On("GetVehicleInfo", mock.Anything, driverID, smallVehicleID).
		Return("Renault", "BB-5678", 2, true, nil)

	_, err = client.ChangeTripVehicle(ctx, &trippb.ChangeTripVehicleRequest{
		DriverId:  driverID,
		TripId:    createResp.TripId,
		VehicleId: smallVehicleID,
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
	mockVehicleClient.AssertExpectations(t)
}

// =============================================================================
// ChangeTripAllowances
// =============================================================================

func TestE2E_ChangeTripAllowances_Success(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, mockVehicleClient, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-allowances"
	vehicleID := "vehicle-allowances"

	stubAuthResolve(mockUserClient, driverID)
	mockUserClient.On("IsVerifiedDriver", mock.Anything, driverID).Return(true, nil)
	mockVehicleClient.On("GetVehicleInfo", mock.Anything, driverID, vehicleID).
		Return("Toyota", "AA-1234", 4, true, nil).Maybe()

	// Le repo exige departure > NOW() + 24h
	req := validCreateTripRequest(driverID, vehicleID)
	req.DepartureDatetime = time.Now().Add(25 * time.Hour).Format(time.RFC3339)
	req.EstimatedArrivalDatetime = time.Now().Add(27 * time.Hour).Format(time.RFC3339)

	createResp, err := client.CreateTrip(ctx, req)
	require.NoError(t, err)

	resp, err := client.ChangeTripAllowances(ctx, &trippb.ChangeTripAllowancesRequest{
		DriverId:     driverID,
		TripId:       createResp.TripId,
		AllowPets:    true,
		AllowFood:    true,
		AllowSmoking: false,
		AllowLuggage: true,
	})

	require.NoError(t, err)
	assert.True(t, resp.Success)
	mockUserClient.AssertExpectations(t)
}

// =============================================================================
// ChangeAutoApprove
// =============================================================================

func TestE2E_ChangeAutoApprove_Success(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, mockVehicleClient, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-autoapprove"
	vehicleID := "vehicle-autoapprove"

	stubAuthResolve(mockUserClient, driverID)
	mockUserClient.On("IsVerifiedDriver", mock.Anything, driverID).Return(true, nil)
	mockVehicleClient.On("GetVehicleInfo", mock.Anything, driverID, vehicleID).
		Return("Toyota", "AA-1234", 4, true, nil).Maybe()
	createResp, err := client.CreateTrip(ctx, validCreateTripRequest(driverID, vehicleID))
	require.NoError(t, err)

	resp, err := client.ChangeAutoApprove(ctx, &trippb.ChangeAutoApproveRequest{
		DriverId:    driverID,
		TripId:      createResp.TripId,
		AutoApprove: true,
	})

	require.NoError(t, err)
	assert.True(t, resp.Success)
	mockUserClient.AssertExpectations(t)
}

// =============================================================================
// EndTrip
// =============================================================================

func TestE2E_EndTrip_Success(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, mockVehicleClient, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-end-trip"
	vehicleID := "vehicle-end-trip"

	stubAuthResolve(mockUserClient, driverID)
	mockUserClient.On("IsVerifiedDriver", mock.Anything, driverID).Return(true, nil)
	mockVehicleClient.On("GetVehicleInfo", mock.Anything, driverID, vehicleID).
		Return("Toyota", "AA-1234", 4, true, nil).Maybe()

	createResp, err := client.CreateTrip(ctx, validCreateTripRequest(driverID, vehicleID))
	require.NoError(t, err)

	_, err = client.StartTrip(ctx, &trippb.StartTripRequest{
		DriverId: driverID,
		TripId:   createResp.TripId,
	})
	require.NoError(t, err)

	resp, err := client.EndTrip(ctx, &trippb.EndTripRequest{
		DriverId: driverID,
		TripId:   createResp.TripId,
	})

	require.NoError(t, err)
	assert.True(t, resp.Success)
	mockUserClient.AssertExpectations(t)
}

func TestE2E_EndTrip_NotStarted(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, mockVehicleClient, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-end-scheduled"
	vehicleID := "vehicle-end-scheduled"

	stubAuthResolve(mockUserClient, driverID)
	mockUserClient.On("IsVerifiedDriver", mock.Anything, driverID).Return(true, nil)
	mockVehicleClient.On("GetVehicleInfo", mock.Anything, driverID, vehicleID).
		Return("Toyota", "AA-1234", 4, true, nil).Maybe()
	createResp, err := client.CreateTrip(ctx, validCreateTripRequest(driverID, vehicleID))
	require.NoError(t, err)

	// Essayer de terminer un trajet encore planifié
	_, err = client.EndTrip(ctx, &trippb.EndTripRequest{
		DriverId: driverID,
		TripId:   createResp.TripId,
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
	mockUserClient.AssertExpectations(t)
}

// =============================================================================
// ConfirmWaypointArrival et ConfirmWaypointDeparture
// =============================================================================

func TestE2E_ConfirmWaypoint_StopFlow(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, mockVehicleClient, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-waypoint-flow"
	vehicleID := "vehicle-waypoint-flow"

	// Créer un trajet avec 3 waypoints (départ, stop, arrivée)
	stubAuthResolve(mockUserClient, driverID)
	mockUserClient.On("IsVerifiedDriver", mock.Anything, driverID).Return(true, nil)
	mockVehicleClient.On("GetVehicleInfo", mock.Anything, driverID, vehicleID).
		Return("Toyota", "AA-1234", 4, true, nil).Maybe()
	req := validCreateTripRequest(driverID, vehicleID)
	// Remplacer l'arrivée par ordre 3 et ajouter un stop en ordre 2
	req.TripWaypoints = []*trippb.WaypointInput{
		{
			SequencerOrder: 1,
			WaypointType:   "departure",
			LocationName:   "Lomé Centre",
			LocationLng:    1.2228,
			LocationLat:    6.1375,
			City:           "Lomé",
			Country:        "TG",
		},
		{
			SequencerOrder: 2,
			WaypointType:   "stop",
			LocationName:   "Notsé",
			LocationLng:    1.1700,
			LocationLat:    6.9700,
			City:           "Notsé",
			Country:        "TG",
		},
		{
			SequencerOrder: 3,
			WaypointType:   "arrival",
			LocationName:   "Kpalimé Marché",
			LocationLng:    0.6370,
			LocationLat:    6.8999,
			City:           "Kpalimé",
			Country:        "TG",
		},
	}

	createResp, err := client.CreateTrip(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, createResp.TripId)

	// Démarrer le trajet
	_, err = client.StartTrip(ctx, &trippb.StartTripRequest{
		DriverId: driverID,
		TripId:   createResp.TripId,
	})
	require.NoError(t, err)

	// Récupérer l'ID du stop waypoint directement depuis la base
	var stopWaypointID string
	err = testPool.QueryRow(ctx,
		"SELECT waypoint_id FROM trips_waypoints WHERE trip_id = $1 AND waypoint_type = 'stop'",
		createResp.TripId,
	).Scan(&stopWaypointID)
	require.NoError(t, err)
	require.NotEmpty(t, stopWaypointID)

	// Confirmer l'arrivée au stop
	arrivalResp, err := client.ConfirmWaypointArrival(ctx, &trippb.ConfirmWaypointArrivalRequest{
		DriverId:   driverID,
		WaypointId: stopWaypointID,
	})
	require.NoError(t, err)
	assert.True(t, arrivalResp.Success)

	// Confirmer le départ du stop
	departureResp, err := client.ConfirmWaypointDeparture(ctx, &trippb.ConfirmWaypointDepartureRequest{
		DriverId:   driverID,
		WaypointId: stopWaypointID,
	})
	require.NoError(t, err)
	assert.True(t, departureResp.Success)

	mockUserClient.AssertExpectations(t)
}

// =============================================================================
// GetTripByID
// =============================================================================

func TestE2E_GetTripByID_Success(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, mockVehicleClient, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-get-by-id"
	vehicleID := "vehicle-get-by-id"

	stubAuthResolve(mockUserClient, driverID)
	mockUserClient.On("IsVerifiedDriver", mock.Anything, driverID).Return(true, nil)
	mockVehicleClient.On("GetVehicleInfo", mock.Anything, driverID, vehicleID).
		Return("Toyota", "AA-1234", 4, true, nil).Maybe()
	createResp, err := client.CreateTrip(ctx, validCreateTripRequest(driverID, vehicleID))
	require.NoError(t, err)
	require.NotEmpty(t, createResp.TripId)

	// Enrichissement côté GetTripByID
	mockUserClient.On("GetDriverName", mock.Anything, driverID).Return("Jean Test", nil).Maybe()
	mockUserClient.On("GetDriverInfo", mock.Anything, driverID).Return("Jean", "+228", nil).Maybe()
	mockVehicleClient.On("GetVehicleInfo", mock.Anything, driverID, vehicleID).
		Return("Toyota", "AA-1234", 4, true, nil).Maybe()

	resp, err := client.GetTripByID(ctx, &trippb.GetTripByIDRequest{TripId: createResp.TripId})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, createResp.TripId, resp.TripId)
	assert.Equal(t, driverID, resp.DriverId)
	assert.Len(t, resp.Waypoints, 2)
}

func TestE2E_GetTripByID_NotFound(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, _, _, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)

	_, err := client.GetTripByID(ctx, &trippb.GetTripByIDRequest{TripId: "nonexistent-trip-id"})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

// =============================================================================
// UpdateAvailableSeats
// =============================================================================

func TestE2E_UpdateAvailableSeats_Success(t *testing.T) {
	ctx := ctxWithUID("e2e-test-user")
	conn, mockUserClient, mockVehicleClient, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-upd-seats"
	vehicleID := "vehicle-upd-seats"

	stubAuthResolve(mockUserClient, driverID)
	mockUserClient.On("IsVerifiedDriver", mock.Anything, driverID).Return(true, nil)
	mockVehicleClient.On("GetVehicleInfo", mock.Anything, driverID, vehicleID).
		Return("Toyota", "AA-1234", 4, true, nil).Maybe()
	createResp, err := client.CreateTrip(ctx, validCreateTripRequest(driverID, vehicleID))
	require.NoError(t, err)
	require.NotEmpty(t, createResp.TripId)

	resp, err := client.UpdateAvailableSeats(ctx, &trippb.UpdateAvailableSeatsRequest{
		TripId:            createResp.TripId,
		NewAvailableSeats: 2,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Success)
	mockUserClient.AssertExpectations(t)
}
