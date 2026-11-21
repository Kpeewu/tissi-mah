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
	"errors"
	"strings"
	"time"

	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/cache"
	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/client/nominatim"
	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/client/osrm"
	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/domain"
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

func (s *geolocationServiceImpl) Geocode(
	ctx context.Context, input interfaces.GeocodeInput,
) (*interfaces.GeocodeOutput, error) {
	if s.nominatim == nil {
		// Distinct d'un backend en panne, qui renvoie la même erreur depuis le client :
		// sans ce message, les deux causes sont indiscernables à l'exploitation.
		s.logger.Error("géocodage indisponible : NOMINATIM_URL n'est pas configurée")
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
		return &interfaces.GeocodeOutput{Results: cached}, nil
	}

	// --- Appel Nominatim ---
	// Une saisie sans résultat remonte ErrorAddressNotFound, pas une liste vide :
	// c'est le cas qui déclenche la correction, il ne doit pas court-circuiter la suite.
	results, err := s.nominatim.Search(ctx, input.Query, countryFilter, limit)
	if err != nil && !errors.Is(err, geoErrors.ErrorAddressNotFound) {
		return nil, err
	}

	if len(results) > 0 {
		s.cache.SetGeocode(ctx, input.Query, countryFilter, limit, results)
		return &interfaces.GeocodeOutput{Results: results}, nil
	}

	// Nominatim ne tolère aucune faute de frappe : « Skode » ne renvoie rien. On
	// rapproche alors la saisie d'une localité connue et on réinterroge avec le nom
	// corrigé, que le client affiche pour rester transparent.
	corrected, score := domain.SuggestLocality(input.Query)
	if corrected == "" || strings.EqualFold(corrected, strings.TrimSpace(input.Query)) {
		s.cache.SetGeocode(ctx, input.Query, countryFilter, limit, results)
		return &interfaces.GeocodeOutput{Results: results}, nil
	}

	s.logger.Info("géocodage : saisie rapprochée d'une localité connue",
		zap.String("query", input.Query),
		zap.String("corrected", corrected),
		zap.Float64("score", score))

	correctedResults, err := s.nominatim.Search(ctx, corrected, countryFilter, limit)
	if err != nil && !errors.Is(err, geoErrors.ErrorAddressNotFound) {
		return nil, err
	}
	if len(correctedResults) == 0 {
		s.cache.SetGeocode(ctx, input.Query, countryFilter, limit, results)
		return &interfaces.GeocodeOutput{Results: results}, nil
	}

	// Mise en cache sous le nom CORRIGÉ, pas sous la saisie fautive : la correction
	// elle-même n'est pas stockée, et une saisie fautive servie depuis le cache
	// reviendrait sans son bandeau « Résultats pour … » — l'utilisateur verrait des
	// résultats inattendus sans explication, et la liste se décalerait d'une ligne.
	s.cache.SetGeocode(ctx, corrected, countryFilter, limit, correctedResults)
	return &interfaces.GeocodeOutput{Results: correctedResults, CorrectedQuery: corrected}, nil
}

// =============================================================================
// ReverseGeocode
// =============================================================================

func (s *geolocationServiceImpl) ReverseGeocode(ctx context.Context, input interfaces.ReverseGeocodeInput) (*interfaces.GeocodeResult, error) {
	if s.nominatim == nil {
		// Distinct d'un backend en panne, qui renvoie la même erreur depuis le client :
		// sans ce message, les deux causes sont indiscernables à l'exploitation.
		s.logger.Error("géocodage indisponible : NOMINATIM_URL n'est pas configurée")
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
