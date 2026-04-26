// Package cache fournit un wrapper Redis pour les routes OSRM et les résultats
// de geocoding Nominatim, avec graceful degradation : si Redis est indisponible
// (nil client ou erreur réseau), les méthodes retournent un cache miss et le
// service appelle directement le backend.
//
// Les méthodes ne retournent jamais d'erreur Redis au caller — elles loguent
// un warning et continuent. C'est le pattern voulu par le plan d'implémentation
// (cf. rating-service pour le précédent).
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/client/osrm"
	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/service/interfaces"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	keyPrefix      = "geo:"
	routeTTL       = 1 * time.Hour
	geocodeTTL     = 24 * time.Hour
	reverseGeocodeTTL = 24 * time.Hour
)

// Cache wrappe les opérations Redis du geolocation-service.
// Si client == nil, toutes les méthodes deviennent des no-ops (cache miss / silent set).
type Cache struct {
	client *redis.Client
	logger *zap.Logger
}

// New construit le cache. client peut être nil — dans ce cas le service tournera
// sans cache (graceful degradation au démarrage si Redis n'est pas joignable).
func New(client *redis.Client, logger *zap.Logger) *Cache {
	return &Cache{client: client, logger: logger}
}

// =============================================================================
// Route cache
// =============================================================================

// GetRoute retourne le RouteResult OSRM mis en cache, ou (nil, false) si miss /
// erreur Redis. Aucune erreur n'est jamais propagée au caller.
func (c *Cache) GetRoute(ctx context.Context, waypoints []osrm.Coordinate, profile string) (*osrm.RouteResult, bool) {
	if c == nil || c.client == nil {
		return nil, false
	}
	key := routeKey(waypoints, profile)

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			c.logger.Warn("cache get route failed", zap.Error(err), zap.String("key", key))
		}
		return nil, false
	}

	var result osrm.RouteResult
	if err := json.Unmarshal(data, &result); err != nil {
		c.logger.Warn("cache unmarshal route failed", zap.Error(err), zap.String("key", key))
		c.client.Del(ctx, key) //nolint:errcheck
		return nil, false
	}
	return &result, true
}

// SetRoute stocke un RouteResult avec TTL 1h. Erreurs silencieuses.
func (c *Cache) SetRoute(ctx context.Context, waypoints []osrm.Coordinate, profile string, result *osrm.RouteResult) {
	if c == nil || c.client == nil || result == nil {
		return
	}
	key := routeKey(waypoints, profile)

	data, err := json.Marshal(result)
	if err != nil {
		c.logger.Warn("cache marshal route failed", zap.Error(err))
		return
	}
	if err := c.client.Set(ctx, key, data, routeTTL).Err(); err != nil {
		c.logger.Warn("cache set route failed", zap.Error(err), zap.String("key", key))
	}
}

// =============================================================================
// Geocode cache (Nominatim search)
// =============================================================================

func (c *Cache) GetGeocode(ctx context.Context, query, countryFilter string, limit int32) ([]*interfaces.GeocodeResult, bool) {
	if c == nil || c.client == nil {
		return nil, false
	}
	key := geocodeKey(query, countryFilter, limit)

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			c.logger.Warn("cache get geocode failed", zap.Error(err), zap.String("key", key))
		}
		return nil, false
	}

	var results []*interfaces.GeocodeResult
	if err := json.Unmarshal(data, &results); err != nil {
		c.logger.Warn("cache unmarshal geocode failed", zap.Error(err), zap.String("key", key))
		c.client.Del(ctx, key) //nolint:errcheck
		return nil, false
	}
	return results, true
}

func (c *Cache) SetGeocode(ctx context.Context, query, countryFilter string, limit int32, results []*interfaces.GeocodeResult) {
	if c == nil || c.client == nil {
		return
	}
	key := geocodeKey(query, countryFilter, limit)

	data, err := json.Marshal(results)
	if err != nil {
		c.logger.Warn("cache marshal geocode failed", zap.Error(err))
		return
	}
	if err := c.client.Set(ctx, key, data, geocodeTTL).Err(); err != nil {
		c.logger.Warn("cache set geocode failed", zap.Error(err), zap.String("key", key))
	}
}

// =============================================================================
// Reverse geocode cache
// =============================================================================

func (c *Cache) GetReverseGeocode(ctx context.Context, lat, lng float64) (*interfaces.GeocodeResult, bool) {
	if c == nil || c.client == nil {
		return nil, false
	}
	key := reverseGeocodeKey(lat, lng)

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			c.logger.Warn("cache get reverse failed", zap.Error(err), zap.String("key", key))
		}
		return nil, false
	}

	var result interfaces.GeocodeResult
	if err := json.Unmarshal(data, &result); err != nil {
		c.logger.Warn("cache unmarshal reverse failed", zap.Error(err), zap.String("key", key))
		c.client.Del(ctx, key) //nolint:errcheck
		return nil, false
	}
	return &result, true
}

func (c *Cache) SetReverseGeocode(ctx context.Context, lat, lng float64, result *interfaces.GeocodeResult) {
	if c == nil || c.client == nil || result == nil {
		return
	}
	key := reverseGeocodeKey(lat, lng)

	data, err := json.Marshal(result)
	if err != nil {
		c.logger.Warn("cache marshal reverse failed", zap.Error(err))
		return
	}
	if err := c.client.Set(ctx, key, data, reverseGeocodeTTL).Err(); err != nil {
		c.logger.Warn("cache set reverse failed", zap.Error(err), zap.String("key", key))
	}
}

// =============================================================================
// Key builders
// =============================================================================

// routeKey hashe les waypoints (ordre + coordonnées) + profile pour produire
// une clé déterministe et compacte. SHA-256 garantit l'absence de collision
// pratique et la même clé pour les mêmes inputs (ce qui est tout l'intérêt
// du cache : 100 chauffeurs qui demandent Lomé→Cotonou hit la même entrée).
func routeKey(waypoints []osrm.Coordinate, profile string) string {
	h := sha256.New()
	h.Write([]byte(profile))
	h.Write([]byte{0})
	for _, w := range waypoints {
		h.Write([]byte(strconv.FormatFloat(w.Lat, 'f', 6, 64)))
		h.Write([]byte{','})
		h.Write([]byte(strconv.FormatFloat(w.Lng, 'f', 6, 64)))
		h.Write([]byte{';'})
	}
	return keyPrefix + "route:" + hex.EncodeToString(h.Sum(nil))[:32]
}

func geocodeKey(query, countryFilter string, limit int32) string {
	h := sha256.New()
	h.Write([]byte(query))
	h.Write([]byte{0})
	h.Write([]byte(countryFilter))
	h.Write([]byte{0})
	h.Write([]byte(strconv.FormatInt(int64(limit), 10)))
	return keyPrefix + "geocode:" + hex.EncodeToString(h.Sum(nil))[:32]
}

// reverseGeocodeKey arrondit à 5 décimales (~1m de précision) — au-delà,
// chaque clic différent sur la carte donnerait une nouvelle clé et le cache
// serait inutile pour ce cas d'usage.
func reverseGeocodeKey(lat, lng float64) string {
	return fmt.Sprintf("%sreverse:%s,%s",
		keyPrefix,
		strconv.FormatFloat(roundTo(lat, 5), 'f', 5, 64),
		strconv.FormatFloat(roundTo(lng, 5), 'f', 5, 64),
	)
}

func roundTo(f float64, decimals int) float64 {
	mult := 1.0
	for i := 0; i < decimals; i++ {
		mult *= 10
	}
	if f >= 0 {
		return float64(int64(f*mult+0.5)) / mult
	}
	return float64(int64(f*mult-0.5)) / mult
}
