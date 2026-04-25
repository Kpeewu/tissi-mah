package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/cache"
	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/client/osrm"
	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/service"
	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/service/interfaces"
	geoErrors "github.com/Kpeewu/tissi-mah/services/geolocation-service/pkg/errors"
	"github.com/Kpeewu/tissi-mah/services/geolocation-service/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakeRoute renvoie un RouteResult déterministe pour les mocks.
func fakeRoute() *osrm.RouteResult {
	return &osrm.RouteResult{
		DistanceMeters:  120000, // 120 km
		DurationSeconds: 7200,   // 2h
		Polyline:        "_p~iF~ps|U_ulLnnqC",
		Legs: []osrm.LegResult{
			{DistanceMeters: 60000, DurationSeconds: 3000}, // 50 min
			{DistanceMeters: 60000, DurationSeconds: 4200}, // 70 min
		},
	}
}

func newTestService(osrmClient osrm.Client) interfaces.GeolocationService {
	return service.NewGeolocationService(osrmClient, cache.New(nil, zap.NewNop()), zap.NewNop())
}

// =============================================================================
// Validation
// =============================================================================

func TestComputeRoute_LessThanTwoWaypoints_ReturnsInvalidWaypoints(t *testing.T) {
	svc := newTestService(&mocks.MockOSRMClient{})

	_, err := svc.ComputeRoute(context.Background(), interfaces.ComputeRouteInput{
		Waypoints: []interfaces.Coordinate{{Lat: 0, Lng: 0}},
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, geoErrors.ErrorInvalidWaypoints)
}

func TestComputeRoute_OutOfRangeLat_ReturnsInvalidWaypoints(t *testing.T) {
	svc := newTestService(&mocks.MockOSRMClient{})

	_, err := svc.ComputeRoute(context.Background(), interfaces.ComputeRouteInput{
		Waypoints: []interfaces.Coordinate{{Lat: 95, Lng: 0}, {Lat: 0, Lng: 0}},
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, geoErrors.ErrorInvalidWaypoints)
}

func TestComputeRoute_UnsupportedProfile_ReturnsInvalidProfile(t *testing.T) {
	svc := newTestService(&mocks.MockOSRMClient{})

	_, err := svc.ComputeRoute(context.Background(), interfaces.ComputeRouteInput{
		Waypoints: []interfaces.Coordinate{{Lat: 6, Lng: 1}, {Lat: 7, Lng: 1}},
		Profile:   "biking",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, geoErrors.ErrorInvalidProfile)
}

func TestComputeRoute_InvalidDepartureTime_ReturnsError(t *testing.T) {
	svc := newTestService(&mocks.MockOSRMClient{})

	_, err := svc.ComputeRoute(context.Background(), interfaces.ComputeRouteInput{
		Waypoints:     []interfaces.Coordinate{{Lat: 6, Lng: 1}, {Lat: 7, Lng: 1}},
		DepartureTime: "not a date",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, geoErrors.ErrorInvalidDepartureTime)
}

// =============================================================================
// Happy paths
// =============================================================================

func TestComputeRoute_NoDepartureTime_ReturnsLegsWithoutETA(t *testing.T) {
	mockClient := &mocks.MockOSRMClient{}
	mockClient.On("GetRoute", mock.Anything, mock.Anything, "driving").
		Return(fakeRoute(), nil).Once()
	svc := newTestService(mockClient)

	result, err := svc.ComputeRoute(context.Background(), interfaces.ComputeRouteInput{
		Waypoints: []interfaces.Coordinate{
			{Lat: 6.1319, Lng: 1.2228},
			{Lat: 6.5, Lng: 1.0},
			{Lat: 6.9269, Lng: 0.6266},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 120000.0, result.DistanceMeters)
	assert.Equal(t, int64(7200), result.DurationSeconds)
	assert.Equal(t, "_p~iF~ps|U_ulLnnqC", result.PolylineEncoded)
	require.Len(t, result.Legs, 2)

	// MinutesFromDeparture est cumulé : leg 0 = 50 min, leg 1 = 50 + 70 = 120 min.
	assert.Equal(t, int32(50), result.Legs[0].MinutesFromDeparture)
	assert.Equal(t, int32(120), result.Legs[1].MinutesFromDeparture)
	// Pas de DepartureTime → ETA vide.
	assert.Empty(t, result.Legs[0].EstimatedArrivalTime)
	assert.Empty(t, result.Legs[1].EstimatedArrivalTime)

	mockClient.AssertExpectations(t)
}

func TestComputeRoute_WithDepartureTime_ReturnsLegsWithETA(t *testing.T) {
	mockClient := &mocks.MockOSRMClient{}
	mockClient.On("GetRoute", mock.Anything, mock.Anything, "driving").
		Return(fakeRoute(), nil).Once()
	svc := newTestService(mockClient)

	departure := "2026-04-26T08:00:00Z"
	result, err := svc.ComputeRoute(context.Background(), interfaces.ComputeRouteInput{
		Waypoints: []interfaces.Coordinate{
			{Lat: 6.1319, Lng: 1.2228},
			{Lat: 6.5, Lng: 1.0},
			{Lat: 6.9269, Lng: 0.6266},
		},
		DepartureTime: departure,
	})

	require.NoError(t, err)
	require.Len(t, result.Legs, 2)

	// Leg 0 : 50 min après 08:00 → 08:50.
	expected0, _ := time.Parse(time.RFC3339, "2026-04-26T08:50:00Z")
	got0, _ := time.Parse(time.RFC3339, result.Legs[0].EstimatedArrivalTime)
	assert.True(t, got0.Equal(expected0), "leg 0 ETA: got %s, want %s", got0, expected0)

	// Leg 1 : 120 min après 08:00 → 10:00.
	expected1, _ := time.Parse(time.RFC3339, "2026-04-26T10:00:00Z")
	got1, _ := time.Parse(time.RFC3339, result.Legs[1].EstimatedArrivalTime)
	assert.True(t, got1.Equal(expected1), "leg 1 ETA: got %s, want %s", got1, expected1)

	mockClient.AssertExpectations(t)
}

func TestComputeRoute_OSRMError_PropagatesError(t *testing.T) {
	mockClient := &mocks.MockOSRMClient{}
	mockClient.On("GetRoute", mock.Anything, mock.Anything, "driving").
		Return(nil, geoErrors.ErrorRoutingUnavailable).Once()
	svc := newTestService(mockClient)

	_, err := svc.ComputeRoute(context.Background(), interfaces.ComputeRouteInput{
		Waypoints: []interfaces.Coordinate{{Lat: 0, Lng: 0}, {Lat: 1, Lng: 1}},
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, geoErrors.ErrorRoutingUnavailable)
}

// =============================================================================
// Geocoding stubs (Phase 4 placeholder — vérifie qu'on retourne bien
// ErrorGeocodingUnavailable en attendant l'implémentation Nominatim)
// =============================================================================

func TestGeocode_PlaceholderReturnsUnavailable(t *testing.T) {
	svc := newTestService(&mocks.MockOSRMClient{})

	_, err := svc.Geocode(context.Background(), interfaces.GeocodeInput{Query: "Lomé"})

	require.Error(t, err)
	assert.ErrorIs(t, err, geoErrors.ErrorGeocodingUnavailable)
}

func TestReverseGeocode_PlaceholderReturnsUnavailable(t *testing.T) {
	svc := newTestService(&mocks.MockOSRMClient{})

	_, err := svc.ReverseGeocode(context.Background(), interfaces.ReverseGeocodeInput{Lat: 6, Lng: 1})

	require.Error(t, err)
	assert.ErrorIs(t, err, geoErrors.ErrorGeocodingUnavailable)
}
