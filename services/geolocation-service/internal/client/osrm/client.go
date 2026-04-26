// Package osrm fournit un client HTTP vers une instance OSRM self-hosted,
// avec trois couches de protection runtime :
//
//  1. Timeout HTTP par requête (5s par défaut) — empêche un OSRM lent de bloquer un pod.
//  2. Semaphore (50 requêtes concurrentes par défaut) — protège OSRM en limitant la concurrence
//     par pod. Au-delà : ErrorRoutingOverloaded (503 logique) au lieu de saturer le backend.
//  3. Circuit breaker (sony/gobreaker, 5 échecs consécutifs → open pendant 30s) — évite
//     l'effet retry-storm lorsqu'OSRM tombe.
//
// Ces trois couches sont configurables via Config mais leurs valeurs par défaut sont
// dimensionnées pour le profil de charge attendu en production (cf. plan d'implémentation).
package osrm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	geoErrors "github.com/Kpeewu/tissi-mah/services/geolocation-service/pkg/errors"
	"github.com/sony/gobreaker"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
)

// Client est le contrat appelé par la couche service.
type Client interface {
	GetRoute(ctx context.Context, waypoints []Coordinate, profile string) (*RouteResult, error)
}

// Coordinate utilisé en input du client (le service convertit ses propres types
// vers celui-ci pour ne pas créer de dépendance cyclique).
type Coordinate struct {
	Lat float64
	Lng float64
}

// Config regroupe les paramètres du client OSRM.
type Config struct {
	BaseURL              string        // ex: http://osrm-backend:5000
	RequestTimeout       time.Duration // défaut: 5s
	MaxConcurrentReqs    int64         // défaut: 50 — taille du semaphore
	BreakerMaxFailures   uint32        // défaut: 5  — échecs consécutifs avant open
	BreakerOpenDuration  time.Duration // défaut: 30s — durée open avant half-open
	MaxIdleConnsPerHost  int           // défaut: 50
	IdleConnTimeout      time.Duration // défaut: 90s
}

// DefaultConfig retourne les valeurs recommandées par le plan d'implémentation.
func DefaultConfig(baseURL string) Config {
	return Config{
		BaseURL:             baseURL,
		RequestTimeout:      5 * time.Second,
		MaxConcurrentReqs:   50,
		BreakerMaxFailures:  5,
		BreakerOpenDuration: 30 * time.Second,
		MaxIdleConnsPerHost: 50,
		IdleConnTimeout:     90 * time.Second,
	}
}

// httpClient est l'interface HTTP minimale dont on a besoin (facilite les tests).
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

// New construit un client OSRM prêt à l'emploi avec les trois protections actives.
func New(cfg Config, logger *zap.Logger) Client {
	if cfg.RequestTimeout == 0 {
		cfg.RequestTimeout = 5 * time.Second
	}
	if cfg.MaxConcurrentReqs == 0 {
		cfg.MaxConcurrentReqs = 50
	}
	if cfg.BreakerMaxFailures == 0 {
		cfg.BreakerMaxFailures = 5
	}
	if cfg.BreakerOpenDuration == 0 {
		cfg.BreakerOpenDuration = 30 * time.Second
	}
	if cfg.MaxIdleConnsPerHost == 0 {
		cfg.MaxIdleConnsPerHost = 50
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
		Name:    "osrm",
		Timeout: cfg.BreakerOpenDuration,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= cfg.BreakerMaxFailures
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			logger.Warn("osrm circuit breaker state change",
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

// GetRoute appelle OSRM /route/v1/{profile}/{coords}?overview=full&geometries=polyline.
// Applique semaphore + circuit breaker + timeout.
func (c *client) GetRoute(ctx context.Context, waypoints []Coordinate, profile string) (*RouteResult, error) {
	if len(waypoints) < 2 {
		return nil, geoErrors.ErrorInvalidWaypoints
	}
	if profile == "" {
		profile = "driving"
	}

	// 1. Semaphore — au-delà de MaxConcurrentReqs : 503 immédiat plutôt que saturer OSRM.
	if err := c.sem.Acquire(ctx, 1); err != nil {
		// Acquire annulée = ctx.Done(), on remonte tel quel pour que le caller voie
		// la cause réelle (timeout client / annulation).
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, geoErrors.ErrorRoutingOverloaded
	}
	defer c.sem.Release(1)

	// 2. Circuit breaker — si OSRM est down, on court-circuite sans HTTP.
	result, err := c.breaker.Execute(func() (interface{}, error) {
		return c.doGetRoute(ctx, waypoints, profile)
	})
	if err != nil {
		// Gobreaker renvoie ErrOpenState quand le breaker est ouvert.
		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			return nil, geoErrors.ErrorRoutingUnavailable
		}
		return nil, err
	}
	return result.(*RouteResult), nil
}

// doGetRoute fait l'appel HTTP réel — appelé via le breaker.
func (c *client) doGetRoute(ctx context.Context, waypoints []Coordinate, profile string) (*RouteResult, error) {
	endpoint := buildRouteURL(c.cfg.BaseURL, profile, waypoints)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		c.logger.Warn("osrm HTTP error", zap.Error(err))
		return nil, geoErrors.ErrorRoutingUnavailable
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, geoErrors.ErrorRoutingUnavailable
	}

	if resp.StatusCode >= 500 {
		c.logger.Warn("osrm 5xx", zap.Int("status", resp.StatusCode), zap.String("body", truncate(string(body), 200)))
		return nil, geoErrors.ErrorRoutingUnavailable
	}

	var parsed osrmResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode osrm response: %w", err)
	}

	if parsed.Code != "Ok" {
		// "NoRoute" / "NoSegment" → pas une panne backend, juste pas de route trouvée.
		if parsed.Code == "NoRoute" || parsed.Code == "NoSegment" {
			return nil, geoErrors.ErrorRouteNotFound
		}
		c.logger.Warn("osrm non-Ok response", zap.String("code", parsed.Code), zap.String("message", parsed.Message))
		return nil, geoErrors.ErrorRoutingUnavailable
	}

	if len(parsed.Routes) == 0 {
		return nil, geoErrors.ErrorRouteNotFound
	}

	r := parsed.Routes[0]
	out := &RouteResult{
		DistanceMeters:  r.Distance,
		DurationSeconds: r.Duration,
		Polyline:        r.Geometry,
		Legs:            make([]LegResult, 0, len(r.Legs)),
	}
	for _, leg := range r.Legs {
		out.Legs = append(out.Legs, LegResult{
			DistanceMeters:  leg.Distance,
			DurationSeconds: leg.Duration,
		})
	}
	return out, nil
}

// buildRouteURL construit l'URL OSRM /route/v1/{profile}/{lng,lat;lng,lat;...}
// Le format de coordonnées est {lng,lat} séparées par ; — attention à l'ordre lng/lat.
func buildRouteURL(baseURL, profile string, waypoints []Coordinate) string {
	parts := make([]string, 0, len(waypoints))
	for _, w := range waypoints {
		parts = append(parts,
			strconv.FormatFloat(w.Lng, 'f', -1, 64)+","+strconv.FormatFloat(w.Lat, 'f', -1, 64),
		)
	}
	coords := strings.Join(parts, ";")

	q := url.Values{}
	q.Set("overview", "full")
	q.Set("geometries", "polyline")
	q.Set("steps", "false")
	q.Set("annotations", "false")

	return strings.TrimRight(baseURL, "/") + "/route/v1/" + profile + "/" + coords + "?" + q.Encode()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
