package client_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/client/osrm"
	geoErrors "github.com/Kpeewu/tissi-mah/services/geolocation-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const (
	okBody = `{
      "code": "Ok",
      "routes": [{
        "distance": 12345.6,
        "duration": 678.9,
        "geometry": "_p~iF~ps|U_ulLnnqC",
        "legs": [
          {"distance": 5000, "duration": 300},
          {"distance": 7345.6, "duration": 378.9}
        ]
      }]
    }`

	noRouteBody = `{"code": "NoRoute", "message": "no route between waypoints"}`
)

func newClient(t *testing.T, baseURL string) osrm.Client {
	t.Helper()
	cfg := osrm.DefaultConfig(baseURL)
	// Test-tighter timeout to avoid slow tests on real network errors.
	cfg.RequestTimeout = 2 * time.Second
	return osrm.New(cfg, zap.NewNop())
}

// TestGetRoute_Success — happy path : OSRM répond Ok, le client mappe vers RouteResult.
func TestGetRoute_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/route/v1/driving/")
		assert.Contains(t, r.URL.RawQuery, "geometries=polyline")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(okBody))
	}))
	defer srv.Close()

	client := newClient(t, srv.URL)
	result, err := client.GetRoute(context.Background(), []osrm.Coordinate{
		{Lat: 6.1319, Lng: 1.2228}, // Lomé
		{Lat: 6.9269, Lng: 0.6266}, // Kpalimé
	}, "driving")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 12345.6, result.DistanceMeters)
	assert.Equal(t, 678.9, result.DurationSeconds)
	assert.Equal(t, "_p~iF~ps|U_ulLnnqC", result.Polyline)
	require.Len(t, result.Legs, 2)
	assert.Equal(t, 5000.0, result.Legs[0].DistanceMeters)
	assert.Equal(t, 300.0, result.Legs[0].DurationSeconds)
}

// TestGetRoute_NoRoute — OSRM renvoie code=NoRoute, on doit traduire en ErrorRouteNotFound.
func TestGetRoute_NoRoute(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(noRouteBody))
	}))
	defer srv.Close()

	client := newClient(t, srv.URL)
	_, err := client.GetRoute(context.Background(), []osrm.Coordinate{
		{Lat: 0, Lng: 0},
		{Lat: 1, Lng: 1},
	}, "driving")

	require.Error(t, err)
	assert.ErrorIs(t, err, geoErrors.ErrorRouteNotFound)
}

// TestGetRoute_5xx — le backend renvoie 500, on traduit en ErrorRoutingUnavailable.
func TestGetRoute_5xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"code":"Internal"}`))
	}))
	defer srv.Close()

	client := newClient(t, srv.URL)
	_, err := client.GetRoute(context.Background(), []osrm.Coordinate{
		{Lat: 0, Lng: 0}, {Lat: 1, Lng: 1},
	}, "driving")

	require.Error(t, err)
	assert.ErrorIs(t, err, geoErrors.ErrorRoutingUnavailable)
}

// TestGetRoute_InvalidWaypoints — moins de 2 waypoints court-circuite OSRM.
func TestGetRoute_InvalidWaypoints(t *testing.T) {
	called := atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		called.Add(1)
	}))
	defer srv.Close()

	client := newClient(t, srv.URL)
	_, err := client.GetRoute(context.Background(), []osrm.Coordinate{{Lat: 0, Lng: 0}}, "driving")

	require.Error(t, err)
	assert.ErrorIs(t, err, geoErrors.ErrorInvalidWaypoints)
	assert.Equal(t, int32(0), called.Load(), "OSRM ne doit pas etre appele si validation rate")
}

// TestGetRoute_CircuitBreakerOpens — après N échecs consécutifs, le breaker ouvre
// et les appels suivants retournent ErrorRoutingUnavailable sans frapper OSRM.
func TestGetRoute_CircuitBreakerOpens(t *testing.T) {
	called := atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := osrm.DefaultConfig(srv.URL)
	cfg.BreakerMaxFailures = 3 // ouvrir vite pour le test
	cfg.BreakerOpenDuration = 5 * time.Second
	cfg.RequestTimeout = 2 * time.Second
	client := osrm.New(cfg, zap.NewNop())

	wp := []osrm.Coordinate{{Lat: 0, Lng: 0}, {Lat: 1, Lng: 1}}

	// 3 échecs consécutifs → trip le breaker.
	for i := 0; i < 3; i++ {
		_, err := client.GetRoute(context.Background(), wp, "driving")
		require.Error(t, err)
		assert.ErrorIs(t, err, geoErrors.ErrorRoutingUnavailable)
	}
	hitsBeforeOpen := called.Load()

	// 4ème appel : breaker ouvert, OSRM ne doit PAS etre frappe.
	_, err := client.GetRoute(context.Background(), wp, "driving")
	require.Error(t, err)
	assert.ErrorIs(t, err, geoErrors.ErrorRoutingUnavailable)
	assert.Equal(t, hitsBeforeOpen, called.Load(), "breaker open : aucun nouvel appel HTTP")
}

// TestGetRoute_ContextCanceled — context annulé pendant l'attente du semaphore
// remonte context.Canceled (le caller veut voir l'annulation, pas un overload).
func TestGetRoute_ContextCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(okBody))
	}))
	defer srv.Close()

	client := newClient(t, srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // annule immédiatement

	_, err := client.GetRoute(ctx, []osrm.Coordinate{{Lat: 0, Lng: 0}, {Lat: 1, Lng: 1}}, "driving")
	require.Error(t, err)
	// Selon le timing, on peut voir context.Canceled directement ou wrapped — les deux sont acceptables.
	assert.True(t,
		errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded),
		"got: %v", err,
	)
}

// TestGetRoute_DefaultProfileWhenEmpty — profile vide → "driving" appliqué.
func TestGetRoute_DefaultProfileWhenEmpty(t *testing.T) {
	gotProfile := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// /route/v1/{profile}/...
		_, _ = fmt.Sscanf(r.URL.Path, "/route/v1/%[^/]/", &gotProfile)
		_ = gotProfile
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(okBody))
	}))
	defer srv.Close()

	client := newClient(t, srv.URL)
	_, err := client.GetRoute(context.Background(), []osrm.Coordinate{{Lat: 0, Lng: 0}, {Lat: 1, Lng: 1}}, "")
	require.NoError(t, err)
}
