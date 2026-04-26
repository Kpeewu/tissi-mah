// Package service contient l'implémentation du GeolocationService.
//
// Le service compose 4 dépendances :
//   - osrm.Client       : routing self-hosted (avec timeout / semaphore / circuit breaker)
//   - nominatim.Client  : geocoding self-hosted (mêmes protections)
//   - cache.Cache       : Redis avec graceful degradation
//   - logger
//
// Le service lui-même est stateless. Il transforme les inputs métier en appels
// OSRM/Nominatim, applique le cache, et compose les résultats finaux (notamment
// les ETAs cumulés par leg quand DepartureTime est fourni dans ComputeRoute).
package service

import (
	"context"
	"time"

	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/cache"
	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/client/nominatim"
	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/client/osrm"
	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/service/interfaces"
	geoErrors "github.com/Kpeewu/tissi-mah/services/geolocation-service/pkg/errors"
	"go.uber.org/zap"
)

type geolocationServiceImpl struct {
	osrm             osrm.Client
	nominatim        nominatim.Client // peut être nil → Geocode/ReverseGeocode renvoient ErrorGeocodingUnavailable
	cache            *cache.Cache
	logger           *zap.Logger
	defaultCountries string
}

// NewGeolocationService construit l'implémentation par défaut.
//   - cache peut être nil → graceful degradation, pas de cache.
//   - nominatimClient peut être nil → Geocode/ReverseGeocode renvoient une erreur
//     contrôlée (utile pour Phase 1 quand Nominatim n'est pas encore déployé).
//   - defaultCountries est utilisé comme filtre par défaut quand Geocode.CountryFilter
//     est vide (ex: "tg,gh,bj,bf").
func NewGeolocationService(
	osrmClient osrm.Client,
	nominatimClient nominatim.Client,
	c *cache.Cache,
	defaultCountries string,
	logger *zap.Logger,
) interfaces.GeolocationService {
	return &geolocationServiceImpl{
		osrm:             osrmClient,
		nominatim:        nominatimClient,
		cache:            c,
		defaultCountries: defaultCountries,
		logger:           logger,
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
// Geocode (search)
// =============================================================================

func (s *geolocationServiceImpl) Geocode(ctx context.Context, input interfaces.GeocodeInput) ([]*interfaces.GeocodeResult, error) {
	if s.nominatim == nil {
		return nil, geoErrors.ErrorGeocodingUnavailable
	}
	if input.Query == "" {
		return nil, geoErrors.ErrorEmptyQuery
	}

	countryFilter := input.CountryFilter
	if countryFilter == "" {
		countryFilter = s.defaultCountries
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 5
	}

	// --- Cache lookup ---
	if cached, hit := s.cache.GetGeocode(ctx, input.Query, countryFilter, limit); hit {
		s.logger.Debug("cache hit: geocode", zap.String("query", input.Query))
		return cached, nil
	}

	// --- Appel Nominatim ---
	results, err := s.nominatim.Search(ctx, input.Query, countryFilter, limit)
	if err != nil {
		return nil, err
	}

	s.cache.SetGeocode(ctx, input.Query, countryFilter, limit, results)
	return results, nil
}

// =============================================================================
// ReverseGeocode
// =============================================================================

func (s *geolocationServiceImpl) ReverseGeocode(ctx context.Context, input interfaces.ReverseGeocodeInput) (*interfaces.GeocodeResult, error) {
	if s.nominatim == nil {
		return nil, geoErrors.ErrorGeocodingUnavailable
	}
	if input.Lat < -90 || input.Lat > 90 || input.Lng < -180 || input.Lng > 180 {
		return nil, geoErrors.ErrorInvalidWaypoints
	}

	// --- Cache lookup ---
	if cached, hit := s.cache.GetReverseGeocode(ctx, input.Lat, input.Lng); hit {
		s.logger.Debug("cache hit: reverse", zap.Float64("lat", input.Lat), zap.Float64("lng", input.Lng))
		return cached, nil
	}

	// --- Appel Nominatim ---
	result, err := s.nominatim.Reverse(ctx, input.Lat, input.Lng)
	if err != nil {
		return nil, err
	}

	s.cache.SetReverseGeocode(ctx, input.Lat, input.Lng, result)
	return result, nil
}
