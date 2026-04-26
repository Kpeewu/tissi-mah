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
	// Phase 4 : signature étendue avec nominatim client + defaultCountries.
	// Les tests focalisés sur ComputeRoute passent nominatim=nil (Geocode/Reverse
	// renverront ErrorGeocodingUnavailable, ce qu'on teste ailleurs).
	return service.NewGeolocationService(osrmClient, nil, cache.New(nil, zap.NewNop()), "tg,gh,bj,bf", zap.NewNop())
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
// Geocoding (Phase 4)
// =============================================================================

// newTestServiceWithGeocode wrap newTestService avec un mock Nominatim injecté.
func newTestServiceWithGeocode(osrmClient osrm.Client, nominatimClient *mocks.MockNominatimClient) interfaces.GeolocationService {
	return service.NewGeolocationService(osrmClient, nominatimClient, cache.New(nil, zap.NewNop()), "tg,gh,bj,bf", zap.NewNop())
}

func TestGeocode_NominatimNil_ReturnsUnavailable(t *testing.T) {
	// newTestService passe nominatim=nil → ErrorGeocodingUnavailable.
	svc := newTestService(&mocks.MockOSRMClient{})

	_, err := svc.Geocode(context.Background(), interfaces.GeocodeInput{Query: "Lomé"})

	require.Error(t, err)
	assert.ErrorIs(t, err, geoErrors.ErrorGeocodingUnavailable)
}

func TestGeocode_EmptyQuery_ReturnsError(t *testing.T) {
	svc := newTestServiceWithGeocode(&mocks.MockOSRMClient{}, &mocks.MockNominatimClient{})

	_, err := svc.Geocode(context.Background(), interfaces.GeocodeInput{Query: ""})

	require.Error(t, err)
	assert.ErrorIs(t, err, geoErrors.ErrorEmptyQuery)
}

func TestGeocode_DefaultCountriesUsedWhenFilterEmpty(t *testing.T) {
	mockNomi := &mocks.MockNominatimClient{}
	mockNomi.On("Search", mock.Anything, "Lomé", "tg,gh,bj,bf", int32(5)).
		Return([]*interfaces.GeocodeResult{
			{DisplayName: "Lomé, Togo", Lat: 6.13, Lng: 1.22, Country: "tg"},
		}, nil).Once()
	svc := newTestServiceWithGeocode(&mocks.MockOSRMClient{}, mockNomi)

	results, err := svc.Geocode(context.Background(), interfaces.GeocodeInput{Query: "Lomé"})

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Lomé, Togo", results[0].DisplayName)
	mockNomi.AssertExpectations(t)
}

func TestGeocode_CustomCountryFilterPropagated(t *testing.T) {
	mockNomi := &mocks.MockNominatimClient{}
	mockNomi.On("Search", mock.Anything, "Accra", "gh", int32(3)).
		Return([]*interfaces.GeocodeResult{{DisplayName: "Accra, Ghana"}}, nil).Once()
	svc := newTestServiceWithGeocode(&mocks.MockOSRMClient{}, mockNomi)

	_, err := svc.Geocode(context.Background(), interfaces.GeocodeInput{
		Query:         "Accra",
		CountryFilter: "gh",
		Limit:         3,
	})
	require.NoError(t, err)
	mockNomi.AssertExpectations(t)
}

func TestReverseGeocode_NominatimNil_ReturnsUnavailable(t *testing.T) {
	svc := newTestService(&mocks.MockOSRMClient{})

	_, err := svc.ReverseGeocode(context.Background(), interfaces.ReverseGeocodeInput{Lat: 6, Lng: 1})

	require.Error(t, err)
	assert.ErrorIs(t, err, geoErrors.ErrorGeocodingUnavailable)
}

func TestReverseGeocode_OutOfRange_ReturnsInvalidWaypoints(t *testing.T) {
	svc := newTestServiceWithGeocode(&mocks.MockOSRMClient{}, &mocks.MockNominatimClient{})

	_, err := svc.ReverseGeocode(context.Background(), interfaces.ReverseGeocodeInput{Lat: 95, Lng: 0})

	require.Error(t, err)
	assert.ErrorIs(t, err, geoErrors.ErrorInvalidWaypoints)
}

func TestReverseGeocode_HappyPath(t *testing.T) {
	mockNomi := &mocks.MockNominatimClient{}
	mockNomi.On("Reverse", mock.Anything, 6.13, 1.22).
		Return(&interfaces.GeocodeResult{DisplayName: "Lomé, Togo", Lat: 6.13, Lng: 1.22, Country: "tg"}, nil).Once()
	svc := newTestServiceWithGeocode(&mocks.MockOSRMClient{}, mockNomi)

	r, err := svc.ReverseGeocode(context.Background(), interfaces.ReverseGeocodeInput{Lat: 6.13, Lng: 1.22})

	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, "Lomé, Togo", r.DisplayName)
	mockNomi.AssertExpectations(t)
}
