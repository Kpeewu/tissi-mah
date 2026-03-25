package handler_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/domain"
	grpcHandler "github.com/Kpeewu/tissi-mah/services/trips-service/internal/grpc"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/trips-service/internal/service/interfaces"
	tripErrors "github.com/Kpeewu/tissi-mah/services/trips-service/pkg/errors"
	trippb "github.com/Kpeewu/tissi-mah/services/trips-service/proto/gen"
	"github.com/Kpeewu/tissi-mah/services/trips-service/tests/mocks"
	"go.uber.org/zap"
)

// =============================================================================
// Helpers
// =============================================================================

// newHandler crée un handler avec un mock service frais.
func newHandler() (*grpcHandler.TripHandler, *mocks.MockTripService) {
	mockService := new(mocks.MockTripService)
	handler := grpcHandler.NewTripHandler(mockService, zap.NewNop())
	return handler, mockService
}

// assertGRPCCode vérifie que l'erreur est un statut gRPC avec le code attendu.
func assertGRPCCode(t *testing.T, err error, expected codes.Code) {
	t.Helper()
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok, "l'erreur doit être un statut gRPC")
	assert.Equal(t, expected, st.Code())
}

// =============================================================================
// TestToGRPCError — mapping exhaustif des erreurs domaine → codes gRPC
// =============================================================================

func TestToGRPCError(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		expectedCode codes.Code
	}{
		// InvalidArgument
		{"ErrorInvalidInput → InvalidArgument", tripErrors.ErrorInvalidInput, codes.InvalidArgument},
		{"ErrorInvalidDatetime → InvalidArgument", tripErrors.ErrorInvalidDatetime, codes.InvalidArgument},
		{"ErrorInvalidWaypoints → InvalidArgument", tripErrors.ErrorInvalidWaypoints, codes.InvalidArgument},
		// NotFound
		{"ErrorDriverNotFound → NotFound", tripErrors.ErrorDriverNotFound, codes.NotFound},
		{"ErrorTripNotFound → NotFound", tripErrors.ErrorTripNotFound, codes.NotFound},
		{"ErrorVehicleNotFound → NotFound", tripErrors.ErrorVehicleNotFound, codes.NotFound},
		{"ErrorWaypointNotFound → NotFound", tripErrors.ErrorWaypointNotFound, codes.NotFound},
		// PermissionDenied
		{"ErrorDriverNotVerified → PermissionDenied", tripErrors.ErrorDriverNotVerified, codes.PermissionDenied},
		{"ErrorUnauthorized → PermissionDenied", tripErrors.ErrorUnauthorized, codes.PermissionDenied},
		// FailedPrecondition
		{"ErrorTripNotScheduled → FailedPrecondition", tripErrors.ErrorTripNotScheduled, codes.FailedPrecondition},
		{"ErrorVehicleInsufficientSeats → FailedPrecondition", tripErrors.ErrorVehicleInsufficientSeats, codes.FailedPrecondition},
		{"ErrorTripDepartureTooSoon → FailedPrecondition", tripErrors.ErrorTripDepartureTooSoon, codes.FailedPrecondition},
		{"ErrorDriverAlreadyHasActiveTrip → FailedPrecondition", tripErrors.ErrorDriverAlreadyHasActiveTrip, codes.FailedPrecondition},
		{"ErrorTripNotInProgress → FailedPrecondition", tripErrors.ErrorTripNotInProgress, codes.FailedPrecondition},
		{"ErrorWaypointNotAStop → FailedPrecondition", tripErrors.ErrorWaypointNotAStop, codes.FailedPrecondition},
		{"ErrorWaypointAlreadyArrived → FailedPrecondition", tripErrors.ErrorWaypointAlreadyArrived, codes.FailedPrecondition},
		{"ErrorAnotherStopAlreadyActive → FailedPrecondition", tripErrors.ErrorAnotherStopAlreadyActive, codes.FailedPrecondition},
		{"ErrorPreviousWaypointNotConfirmed → FailedPrecondition", tripErrors.ErrorPreviousWaypointNotConfirmed, codes.FailedPrecondition},
		{"ErrorWaypointNotArrived → FailedPrecondition", tripErrors.ErrorWaypointNotArrived, codes.FailedPrecondition},
		{"ErrorWaypointAlreadyDeparted → FailedPrecondition", tripErrors.ErrorWaypointAlreadyDeparted, codes.FailedPrecondition},
		// Internal
		{"ErrorDataRetrievalFailed → Internal", tripErrors.ErrorDataRetrievalFailed, codes.Internal},
		{"ErrorInternalServer → Internal", tripErrors.ErrorInternalServer, codes.Internal},
		// Erreur inconnue → Internal
		{"erreur inconnue → Internal", errors.New("erreur inattendue"), codes.Internal},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler, mockSvc := newHandler()
			ctx := context.Background()

			// Utilise StartTrip pour déclencher toGRPCError avec n'importe quelle erreur domaine
			mockSvc.On("StartTrip", mock.Anything, mock.Anything).Return(tc.err)

			_, err := handler.StartTrip(ctx, &trippb.StartTripRequest{
				DriverId: "driver-1",
				TripId:   "trip-1",
			})

			assertGRPCCode(t, err, tc.expectedCode)
			mockSvc.AssertExpectations(t)
		})
	}
}

// =============================================================================
// TestCreateTrip_Handler
// =============================================================================

func TestCreateTrip_Handler(t *testing.T) {
	now := time.Now().UTC()
	departure := now.Add(1 * time.Hour)
	arrival := now.Add(3 * time.Hour)

	validReq := &trippb.CreateTripRequest{
		DriverId:                 "driver-1",
		VehicleId:                "vehicle-1",
		DepartureDatetime:        departure.Format(time.RFC3339),
		EstimatedArrivalDatetime: arrival.Format(time.RFC3339),
		EstimatedDurationMinutes: 120,
		EstimatedDistanceMeters:  50000,
		TotalSeats:               4,
		PricePerSeat:             1000,
		TripWaypoints: []*trippb.WaypointInput{
			{
				SequencerOrder: 1,
				WaypointType:   "departure",
				LocationName:   "Lomé",
				LocationLng:    1.2228,
				LocationLat:    6.1375,
				City:           "Lomé",
				Country:        "TG",
			},
			{
				SequencerOrder: 2,
				WaypointType:   "arrival",
				LocationName:   "Kpalimé",
				LocationLng:    0.6370,
				LocationLat:    6.8999,
				City:           "Kpalimé",
				Country:        "TG",
			},
		},
	}

	t.Run("succès - retourne le tripID dans la réponse", func(t *testing.T) {
		handler, mockSvc := newHandler()
		ctx := context.Background()

		returnedTrip := &domain.Trip{TripID: "new-trip-id"}
		mockSvc.On("CreateTrip", ctx, mock.AnythingOfType("*interfaces.CreateTripInput")).
			Return(returnedTrip, nil)

		resp, err := handler.CreateTrip(ctx, validReq)

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "new-trip-id", resp.TripId)
		mockSvc.AssertExpectations(t)
	})

	t.Run("erreur service - propagée comme code gRPC", func(t *testing.T) {
		handler, mockSvc := newHandler()
		ctx := context.Background()

		mockSvc.On("CreateTrip", ctx, mock.AnythingOfType("*interfaces.CreateTripInput")).
			Return(nil, tripErrors.ErrorDriverNotVerified)

		_, err := handler.CreateTrip(ctx, validReq)

		assertGRPCCode(t, err, codes.PermissionDenied)
		mockSvc.AssertExpectations(t)
	})

	t.Run("erreur - conducteur non trouvé → NotFound", func(t *testing.T) {
		handler, mockSvc := newHandler()
		ctx := context.Background()

		mockSvc.On("CreateTrip", ctx, mock.AnythingOfType("*interfaces.CreateTripInput")).
			Return(nil, tripErrors.ErrorDriverNotFound)

		_, err := handler.CreateTrip(ctx, validReq)

		assertGRPCCode(t, err, codes.NotFound)
		mockSvc.AssertExpectations(t)
	})
}

// =============================================================================
// TestStartTrip_Handler
// =============================================================================

func TestStartTrip_Handler(t *testing.T) {
	t.Run("succès - retourne Success=true", func(t *testing.T) {
		handler, mockSvc := newHandler()
		ctx := context.Background()

		mockSvc.On("StartTrip", mock.Anything, mock.Anything).Return(nil)

		resp, err := handler.StartTrip(ctx, &trippb.StartTripRequest{
			DriverId: "driver-1",
			TripId:   "trip-1",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.Success)
		mockSvc.AssertExpectations(t)
	})

	t.Run("erreur - conducteur a déjà un trajet actif → FailedPrecondition", func(t *testing.T) {
		handler, mockSvc := newHandler()
		ctx := context.Background()

		mockSvc.On("StartTrip", mock.Anything, mock.Anything).
			Return(tripErrors.ErrorDriverAlreadyHasActiveTrip)

		_, err := handler.StartTrip(ctx, &trippb.StartTripRequest{
			DriverId: "driver-1",
			TripId:   "trip-1",
		})

		assertGRPCCode(t, err, codes.FailedPrecondition)
		mockSvc.AssertExpectations(t)
	})
}

//=============================================================================
// TestEndTrip_Handler
// =============================================================================

func TestEndTrip_Handler(t *testing.T) {
	t.Run("succès - retourne Success=true", func(t *testing.T) {
		handler, mockSvc := newHandler()
		ctx := context.Background()

		mockSvc.On("EndTrip", mock.Anything, mock.Anything).Return(nil)

		resp, err := handler.EndTrip(ctx, &trippb.EndTripRequest{
			DriverId: "driver-1",
			TripId:   "trip-1",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.Success)
		mockSvc.AssertExpectations(t)
	})

	t.Run("erreur - trajet non en cours → FailedPrecondition", func(t *testing.T) {
		handler, mockSvc := newHandler()
		ctx := context.Background()

		mockSvc.On("EndTrip", mock.Anything, mock.Anything).
			Return(tripErrors.ErrorTripNotInProgress)

		_, err := handler.EndTrip(ctx, &trippb.EndTripRequest{
			DriverId: "driver-1",
			TripId:   "trip-1",
		})

		assertGRPCCode(t, err, codes.FailedPrecondition)
		mockSvc.AssertExpectations(t)
	})
}

// =============================================================================
// TestConfirmWaypointArrival_Handler
// =============================================================================

func TestConfirmWaypointArrival_Handler(t *testing.T) {
	t.Run("succès - retourne Success=true", func(t *testing.T) {
		handler, mockSvc := newHandler()
		ctx := context.Background()

		mockSvc.On("ConfirmWaypointArrival", mock.Anything, mock.Anything).Return(nil)

		resp, err := handler.ConfirmWaypointArrival(ctx, &trippb.ConfirmWaypointArrivalRequest{
			DriverId:   "driver-1",
			WaypointId: "waypoint-1",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.Success)
		mockSvc.AssertExpectations(t)
	})

	t.Run("erreur - waypoint non trouvé → NotFound", func(t *testing.T) {
		handler, mockSvc := newHandler()
		ctx := context.Background()

		mockSvc.On("ConfirmWaypointArrival", mock.Anything, mock.Anything).
			Return(tripErrors.ErrorWaypointNotFound)

		_, err := handler.ConfirmWaypointArrival(ctx, &trippb.ConfirmWaypointArrivalRequest{
			DriverId:   "driver-1",
			WaypointId: "waypoint-inexistant",
		})

		assertGRPCCode(t, err, codes.NotFound)
		mockSvc.AssertExpectations(t)
	})
}

// =============================================================================
// TestConfirmWaypointDeparture_Handler
// =============================================================================

func TestConfirmWaypointDeparture_Handler(t *testing.T) {
	t.Run("succès - retourne Success=true", func(t *testing.T) {
		handler, mockSvc := newHandler()
		ctx := context.Background()

		mockSvc.On("ConfirmWaypointDeparture", mock.Anything, mock.Anything).Return(nil)

		resp, err := handler.ConfirmWaypointDeparture(ctx, &trippb.ConfirmWaypointDepartureRequest{
			DriverId:   "driver-1",
			WaypointId: "waypoint-1",
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.Success)
		mockSvc.AssertExpectations(t)
	})

	t.Run("erreur - arrivée pas encore confirmée → FailedPrecondition", func(t *testing.T) {
		handler, mockSvc := newHandler()
		ctx := context.Background()

		mockSvc.On("ConfirmWaypointDeparture", mock.Anything, mock.Anything).
			Return(tripErrors.ErrorWaypointNotArrived)

		_, err := handler.ConfirmWaypointDeparture(ctx, &trippb.ConfirmWaypointDepartureRequest{
			DriverId:   "driver-1",
			WaypointId: "waypoint-1",
		})

		assertGRPCCode(t, err, codes.FailedPrecondition)
		mockSvc.AssertExpectations(t)
	})
}

// =============================================================================
// TestGetTripByID_Handler
// =============================================================================

func TestGetTripByID_Handler(t *testing.T) {
	t.Run("succès - retourne TripID, DriverID et waypoints", func(t *testing.T) {
		handler, mockSvc := newHandler()
		ctx := context.Background()

		now := time.Now().UTC()
		result := &serviceInterfaces.TripDetailResult{
			TripID:                   "trip-1",
			DriverID:                 "driver-1",
			Status:                   "scheduled",
			TotalSeats:               4,
			AvailableSeats:           3,
			DepartureDatetime:        now.Add(time.Hour),
			EstimatedArrivalDatetime: now.Add(3 * time.Hour),
			Waypoints: []serviceInterfaces.WaypointDetailResult{
				{WaypointID: "wp-1", WaypointType: "departure", SequencerOrder: 1, LocationName: "Lomé"},
				{WaypointID: "wp-2", WaypointType: "arrival", SequencerOrder: 2, LocationName: "Kpalimé"},
			},
		}

		mockSvc.On("GetTripByID", ctx, mock.AnythingOfType("*interfaces.GetTripByIDInput")).
			Return(result, nil)

		resp, err := handler.GetTripByID(ctx, &trippb.GetTripByIDRequest{TripId: "trip-1"})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "trip-1", resp.TripId)
		assert.Equal(t, "driver-1", resp.DriverId)
		assert.Len(t, resp.Waypoints, 2)
		mockSvc.AssertExpectations(t)
	})

	t.Run("erreur - trajet non trouvé → NotFound", func(t *testing.T) {
		handler, mockSvc := newHandler()
		ctx := context.Background()

		mockSvc.On("GetTripByID", ctx, mock.AnythingOfType("*interfaces.GetTripByIDInput")).
			Return(nil, tripErrors.ErrorTripNotFound)

		_, err := handler.GetTripByID(ctx, &trippb.GetTripByIDRequest{TripId: "trip-ghost"})

		assertGRPCCode(t, err, codes.NotFound)
		mockSvc.AssertExpectations(t)
	})

	t.Run("erreur - service interne → Internal", func(t *testing.T) {
		handler, mockSvc := newHandler()
		ctx := context.Background()

		mockSvc.On("GetTripByID", ctx, mock.AnythingOfType("*interfaces.GetTripByIDInput")).
			Return(nil, tripErrors.ErrorInternalServer)

		_, err := handler.GetTripByID(ctx, &trippb.GetTripByIDRequest{TripId: "trip-1"})

		assertGRPCCode(t, err, codes.Internal)
		mockSvc.AssertExpectations(t)
	})
}

// =============================================================================
// TestUpdateAvailableSeats_Handler
// =============================================================================

func TestUpdateAvailableSeats_Handler(t *testing.T) {
	t.Run("succès - retourne Success=true", func(t *testing.T) {
		handler, mockSvc := newHandler()
		ctx := context.Background()

		mockSvc.On("UpdateAvailableSeats", ctx, mock.AnythingOfType("*interfaces.UpdateAvailableSeatsInput")).
			Return(nil)

		resp, err := handler.UpdateAvailableSeats(ctx, &trippb.UpdateAvailableSeatsRequest{
			TripId:            "trip-1",
			NewAvailableSeats: 2,
		})

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, resp.Success)
		mockSvc.AssertExpectations(t)
	})

	t.Run("erreur - trajet non trouvé → NotFound", func(t *testing.T) {
		handler, mockSvc := newHandler()
		ctx := context.Background()

		mockSvc.On("UpdateAvailableSeats", ctx, mock.AnythingOfType("*interfaces.UpdateAvailableSeatsInput")).
			Return(tripErrors.ErrorTripNotFound)

		_, err := handler.UpdateAvailableSeats(ctx, &trippb.UpdateAvailableSeatsRequest{
			TripId:            "trip-ghost",
			NewAvailableSeats: 2,
		})

		assertGRPCCode(t, err, codes.NotFound)
		mockSvc.AssertExpectations(t)
	})
}
