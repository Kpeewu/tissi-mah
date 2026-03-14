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
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

var (
	testPool *pgxpool.Pool
	lis      *bufconn.Listener
)

// =============================================================================
// TestMain — démarrage de PostgreSQL et du serveur gRPC en mémoire
// =============================================================================

func TestMain(m *testing.M) {
	ctx := context.Background()

	// Setup : démarrer PostgreSQL avec les migrations du trips-service
	testPostgres, err := postgresHelper.SetupTestPostgres(ctx, "../../migrations/up",
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

	readRepo := implementations.NewTripReadRepository(testPool, logger)
	writeRepo := implementations.NewTripWriteRepository(testPool, logger)

	mockUserClient := new(mocks.MockUserClient)
	mockVehicleClient := new(mocks.MockVehicleClient)

	svc := service.NewTripService(readRepo, writeRepo, mockUserClient, mockVehicleClient, nil, logger)
	handler := grpcHandler.NewTripHandler(svc, logger)

	lis = bufconn.Listen(bufSize)
	grpcServer := grpc.NewServer()
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
	ctx := context.Background()
	conn, mockUserClient, _, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-e2e-1"
	vehicleID := "vehicle-e2e-1"

	// Conducteur vérifié
	mockUserClient.On("IsVerifiedDriver", mock.Anything, driverID).Return(true, nil)

	resp, err := client.CreateTrip(ctx, validCreateTripRequest(driverID, vehicleID))

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.TripId)
	mockUserClient.AssertExpectations(t)
}

func TestE2E_StartTrip_Success(t *testing.T) {
	ctx := context.Background()
	conn, mockUserClient, _, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-e2e-start"
	vehicleID := "vehicle-e2e-start"

	// Créer un trajet
	mockUserClient.On("IsVerifiedDriver", mock.Anything, driverID).Return(true, nil)
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
	ctx := context.Background()
	conn, mockUserClient, _, cleanup := setupServer(t)
	defer cleanup()
	cleanupTripsE2E(t, ctx)

	client := trippb.NewTripServiceClient(conn)
	driverID := "driver-e2e-active"
	vehicleID := "vehicle-e2e-active"

	mockUserClient.On("IsVerifiedDriver", mock.Anything, driverID).Return(true, nil)

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
	ctx := context.Background()
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
