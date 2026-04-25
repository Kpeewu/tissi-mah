// Package service contient l'implémentation du GeolocationService.
//
// Le service compose 3 dépendances :
//   - osrm.Client    : routing self-hosted (avec timeout / semaphore / circuit breaker)
//   - cache.Cache    : Redis avec graceful degradation
//   - logger
//
// Le service lui-même est stateless. Il transforme les inputs métier en appels OSRM,
// applique le cache, et compose le résultat final (notamment les ETAs cumulés par leg
// quand DepartureTime est fourni).
package service

import (
	"context"
	"time"

	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/cache"
	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/client/osrm"
	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/service/interfaces"
	geoErrors "github.com/Kpeewu/tissi-mah/services/geolocation-service/pkg/errors"
	"go.uber.org/zap"
)

type geolocationServiceImpl struct {
	osrm   osrm.Client
	cache  *cache.Cache
	logger *zap.Logger
}

// NewGeolocationService construit l'implémentation par défaut.
// cache peut être nil — le service tournera sans cache (graceful degradation).
func NewGeolocationService(
	osrmClient osrm.Client,
	c *cache.Cache,
	logger *zap.Logger,
) interfaces.GeolocationService {
	return &geolocationServiceImpl{
		osrm:   osrmClient,
		cache:  c,
		logger: logger,
	}
}

// =============================================================================
// ComputeRoute
// =============================================================================

func (s *geolocationServiceImpl) ComputeRoute(ctx context.Context, input interfaces.ComputeRouteInput) (*interfaces.ComputeRouteResult, error) {
	// --- Validation ---
	if len(input.Waypoints) < 2 {
		return nil, geoErrors.ErrorInvalidWaypoints
	}
	for _, w := range input.Waypoints {
		if w.Lat < -90 || w.Lat > 90 || w.Lng < -180 || w.Lng > 180 {
			return nil, geoErrors.ErrorInvalidWaypoints
		}
	}

	profile := input.Profile
	if profile == "" {
		profile = "driving"
	}
	if profile != "driving" {
		return nil, geoErrors.ErrorInvalidProfile
	}

	var departureTime time.Time
	hasDeparture := input.DepartureTime != ""
	if hasDeparture {
		t, err := time.Parse(time.RFC3339, input.DepartureTime)
		if err != nil {
			return nil, geoErrors.ErrorInvalidDepartureTime
		}
		departureTime = t
	}

	// --- Conversion vers le type du client OSRM ---
	osrmWaypoints := make([]osrm.Coordinate, 0, len(input.Waypoints))
	for _, w := range input.Waypoints {
		osrmWaypoints = append(osrmWaypoints, osrm.Coordinate{Lat: w.Lat, Lng: w.Lng})
	}

	// --- Cache lookup ---
	var routeResult *osrm.RouteResult
	if cached, hit := s.cache.GetRoute(ctx, osrmWaypoints, profile); hit {
		s.logger.Debug("cache hit: route", zap.Int("waypoints", len(input.Waypoints)))
		routeResult = cached
	} else {
		// --- Appel OSRM (avec timeout / semaphore / circuit breaker) ---
		fresh, err := s.osrm.GetRoute(ctx, osrmWaypoints, profile)
		if err != nil {
			return nil, err
		}
		s.cache.SetRoute(ctx, osrmWaypoints, profile, fresh)
		routeResult = fresh
	}

	// --- Composition du résultat avec ETAs cumulés ---
	out := &interfaces.ComputeRouteResult{
		DistanceMeters:  routeResult.DistanceMeters,
		DurationSeconds: int64(routeResult.DurationSeconds),
		PolylineEncoded: routeResult.Polyline,
		Legs:            make([]interfaces.RouteLeg, 0, len(routeResult.Legs)),
	}

	cumulativeSeconds := 0.0
	for _, leg := range routeResult.Legs {
		cumulativeSeconds += leg.DurationSeconds
		minutesFromDeparture := int32(cumulativeSeconds / 60)

		eta := ""
		if hasDeparture {
			eta = departureTime.
				Add(time.Duration(cumulativeSeconds * float64(time.Second))).
				UTC().
				Format(time.RFC3339)
		}

		out.Legs = append(out.Legs, interfaces.RouteLeg{
			DistanceMeters:       leg.DistanceMeters,
			DurationSeconds:      int64(leg.DurationSeconds),
			MinutesFromDeparture: minutesFromDeparture,
			EstimatedArrivalTime: eta,
		})
	}

	return out, nil
}

// =============================================================================
// Geocode (placeholder Phase 4)
// =============================================================================

func (s *geolocationServiceImpl) Geocode(_ context.Context, _ interfaces.GeocodeInput) ([]*interfaces.GeocodeResult, error) {
	return nil, geoErrors.ErrorGeocodingUnavailable
}

func (s *geolocationServiceImpl) ReverseGeocode(_ context.Context, _ interfaces.ReverseGeocodeInput) (*interfaces.GeocodeResult, error) {
	return nil, geoErrors.ErrorGeocodingUnavailable
}
