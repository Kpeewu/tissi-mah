// Package nominatim fournit un client HTTP vers une instance Nominatim self-hosted,
// avec les mêmes 3 couches de protection runtime que le client OSRM : timeout,
// semaphore (concurrence par pod), circuit breaker.
//
// Les semaphore et breaker sont DÉDIÉS à Nominatim — un Nominatim en panne ne
// doit pas couper le routing, et inversement.
package nominatim

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/service/interfaces"
	geoErrors "github.com/Kpeewu/tissi-mah/services/geolocation-service/pkg/errors"
	"github.com/sony/gobreaker"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
)

// Client est le contrat appelé par la couche service.
type Client interface {
	Search(ctx context.Context, query string, countryFilter string, limit int32) ([]*interfaces.GeocodeResult, error)
	Reverse(ctx context.Context, lat, lng float64) (*interfaces.GeocodeResult, error)
}

type Config struct {
	BaseURL              string
	RequestTimeout       time.Duration
	MaxConcurrentReqs    int64
	BreakerMaxFailures   uint32
	BreakerOpenDuration  time.Duration
	MaxIdleConnsPerHost  int
	IdleConnTimeout      time.Duration
}

// DefaultConfig retourne les valeurs recommandées (cf. plan d'implémentation).
// Nominatim est plus lent que OSRM (Postgres) — on accepte un timeout un peu plus large.
func DefaultConfig(baseURL string) Config {
	return Config{
		BaseURL:             baseURL,
		RequestTimeout:      8 * time.Second,
		MaxConcurrentReqs:   30,
		BreakerMaxFailures:  5,
		BreakerOpenDuration: 30 * time.Second,
		MaxIdleConnsPerHost: 30,
		IdleConnTimeout:     90 * time.Second,
	}
}

type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type client struct {
	cfg     Config
	http    httpClient
	sem     *semaphore.Weighted
	breaker *gobreaker.CircuitBreaker
	logger  *zap.Logger
}

// New construit un client Nominatim prêt à l'emploi.
// Si baseURL est vide, retourne nil → le service traitera Geocode comme
// non disponible (graceful degradation pour les déploiements Phase 4).
func New(cfg Config, logger *zap.Logger) Client {
	if cfg.BaseURL == "" {
		return nil
	}
	if cfg.RequestTimeout == 0 {
		cfg.RequestTimeout = 8 * time.Second
	}
	if cfg.MaxConcurrentReqs == 0 {
		cfg.MaxConcurrentReqs = 30
	}
	if cfg.BreakerMaxFailures == 0 {
		cfg.BreakerMaxFailures = 5
	}
	if cfg.BreakerOpenDuration == 0 {
		cfg.BreakerOpenDuration = 30 * time.Second
	}
	if cfg.MaxIdleConnsPerHost == 0 {
		cfg.MaxIdleConnsPerHost = 30
	}
	if cfg.IdleConnTimeout == 0 {
		cfg.IdleConnTimeout = 90 * time.Second
	}

	httpC := &http.Client{
		Timeout: cfg.RequestTimeout,
		Transport: &http.Transport{
			MaxIdleConns:        cfg.MaxIdleConnsPerHost * 2,
			MaxIdleConnsPerHost: cfg.MaxIdleConnsPerHost,
			IdleConnTimeout:     cfg.IdleConnTimeout,
		},
	}

	cbSettings := gobreaker.Settings{
		Name:    "nominatim",
		Timeout: cfg.BreakerOpenDuration,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= cfg.BreakerMaxFailures
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			logger.Warn("nominatim circuit breaker state change",
				zap.String("name", name),
				zap.String("from", from.String()),
				zap.String("to", to.String()),
			)
		},
	}

	return &client{
		cfg:     cfg,
		http:    httpC,
		sem:     semaphore.NewWeighted(cfg.MaxConcurrentReqs),
		breaker: gobreaker.NewCircuitBreaker(cbSettings),
		logger:  logger,
	}
}

// Search appelle GET /search?q=...&format=json&addressdetails=1&countrycodes=...&limit=...
func (c *client) Search(ctx context.Context, query string, countryFilter string, limit int32) ([]*interfaces.GeocodeResult, error) {
	if query == "" {
		return nil, geoErrors.ErrorEmptyQuery
	}
	if limit <= 0 {
		limit = 5
	}

	if err := c.sem.Acquire(ctx, 1); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, geoErrors.ErrorGeocodingOverloaded
	}
	defer c.sem.Release(1)

	result, err := c.breaker.Execute(func() (interface{}, error) {
		return c.doSearch(ctx, query, countryFilter, limit)
	})
	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			return nil, geoErrors.ErrorGeocodingUnavailable
		}
		return nil, err
	}
	return result.([]*interfaces.GeocodeResult), nil
}

func (c *client) doSearch(ctx context.Context, query, countryFilter string, limit int32) ([]*interfaces.GeocodeResult, error) {
	q := url.Values{}
	q.Set("q", query)
	q.Set("format", "json")
	q.Set("addressdetails", "1")
	q.Set("limit", strconv.FormatInt(int64(limit), 10))
	if countryFilter != "" {
		q.Set("countrycodes", countryFilter)
	}

	endpoint := c.cfg.BaseURL + "/search?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		c.logger.Warn("nominatim HTTP error", zap.Error(err))
		return nil, geoErrors.ErrorGeocodingUnavailable
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, geoErrors.ErrorGeocodingUnavailable
	}
	if resp.StatusCode >= 500 {
		c.logger.Warn("nominatim 5xx", zap.Int("status", resp.StatusCode))
		return nil, geoErrors.ErrorGeocodingUnavailable
	}

	var raw []nominatimSearchResult
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode nominatim search response: %w", err)
	}
	if len(raw) == 0 {
		return nil, geoErrors.ErrorAddressNotFound
	}

	out := make([]*interfaces.GeocodeResult, 0, len(raw))
	for _, r := range raw {
		out = append(out, mapSearchResult(r))
	}
	return out, nil
}

// Reverse appelle GET /reverse?lat=...&lon=...&format=json&addressdetails=1
func (c *client) Reverse(ctx context.Context, lat, lng float64) (*interfaces.GeocodeResult, error) {
	if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		return nil, geoErrors.ErrorInvalidWaypoints
	}

	if err := c.sem.Acquire(ctx, 1); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, geoErrors.ErrorGeocodingOverloaded
	}
	defer c.sem.Release(1)

	result, err := c.breaker.Execute(func() (interface{}, error) {
		return c.doReverse(ctx, lat, lng)
	})
	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			return nil, geoErrors.ErrorGeocodingUnavailable
		}
		return nil, err
	}
	return result.(*interfaces.GeocodeResult), nil
}

func (c *client) doReverse(ctx context.Context, lat, lng float64) (*interfaces.GeocodeResult, error) {
	q := url.Values{}
	q.Set("lat", strconv.FormatFloat(lat, 'f', -1, 64))
	q.Set("lon", strconv.FormatFloat(lng, 'f', -1, 64))
	q.Set("format", "json")
	q.Set("addressdetails", "1")

	endpoint := c.cfg.BaseURL + "/reverse?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		c.logger.Warn("nominatim HTTP error", zap.Error(err))
		return nil, geoErrors.ErrorGeocodingUnavailable
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, geoErrors.ErrorGeocodingUnavailable
	}
	if resp.StatusCode >= 500 {
		c.logger.Warn("nominatim 5xx", zap.Int("status", resp.StatusCode))
		return nil, geoErrors.ErrorGeocodingUnavailable
	}

	var raw nominatimReverseResult
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode nominatim reverse response: %w", err)
	}
	if raw.Error != "" || raw.DisplayName == "" {
		return nil, geoErrors.ErrorAddressNotFound
	}
	return mapReverseResult(raw), nil
}

// mapSearchResult convertit un résultat Nominatim brut vers le type métier.
func mapSearchResult(r nominatimSearchResult) *interfaces.GeocodeResult {
	lat, _ := strconv.ParseFloat(r.Lat, 64)
	lng, _ := strconv.ParseFloat(r.Lon, 64)
	return &interfaces.GeocodeResult{
		DisplayName: r.DisplayName,
		Lat:         lat,
		Lng:         lng,
		Country:     countryFromAddress(r.Address),
		City:        addressCity(r.Address),
		Type:        firstNonEmpty(r.Type, r.Class),
	}
}

func mapReverseResult(r nominatimReverseResult) *interfaces.GeocodeResult {
	lat, _ := strconv.ParseFloat(r.Lat, 64)
	lng, _ := strconv.ParseFloat(r.Lon, 64)
	return &interfaces.GeocodeResult{
		DisplayName: r.DisplayName,
		Lat:         lat,
		Lng:         lng,
		Country:     countryFromAddress(r.Address),
		City:        addressCity(r.Address),
		Type:        firstNonEmpty(r.Type, r.Class),
	}
}

func countryFromAddress(a *nominatimAddress) string {
	if a == nil {
		return ""
	}
	if a.CountryCode != "" {
		return a.CountryCode
	}
	return a.Country
}

func addressCity(a *nominatimAddress) string {
	return a.pickCity()
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
