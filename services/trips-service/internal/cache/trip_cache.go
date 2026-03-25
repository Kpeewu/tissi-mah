package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/domain"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// TTL du cache pour la liste paginée des trajets d'un conducteur
	previewsTTL = 2 * time.Minute

	// TTL du cache pour le nom du conducteur (change très rarement)
	driverNameTTL = 15 * time.Minute

	// TTL du cache pour les infos véhicule (marque/plaque, changent rarement)
	vehicleInfoTTL = 10 * time.Minute

	// TTL du compteur de places disponibles — plus long que l'intervalle de réconciliation
	seatCounterTTL = 24 * time.Hour

	// Préfixe des clés Redis pour le trips-service
	keyPrefix = "trip:"
)

// cachedVehicleInfo est la structure JSON stockée pour les infos véhicule.
type cachedVehicleInfo struct {
	Brand string `json:"brand"`
	Plate string `json:"plate"`
}

// TripCache gère le cache Redis pour le trips-service.
type TripCache struct {
	client *redis.Client
	logger *zap.Logger
}

// NewTripCache crée une instance du cache trips.
func NewTripCache(client *redis.Client, logger *zap.Logger) *TripCache {
	return &TripCache{client: client, logger: logger.Named("cache")}
}

// =============================================================================
// Previews (résultat DB brut, paginé par conducteur)
// =============================================================================

// GetTripsPreviews récupère la liste paginée des trajets depuis le cache.
// Retourne nil, nil si la clé n'existe pas (cache miss).
func (c *TripCache) GetTripsPreviews(ctx context.Context, driverID string, pageIndex int) ([]*domain.TripPreview, error) {
	key := c.previewsKey(driverID, pageIndex)

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			c.logger.Debug("cache miss: trip previews", zap.String("driverID", driverID), zap.Int("page", pageIndex))
			return nil, nil
		}
		c.logger.Error("cache get failed", zap.Error(err), zap.String("key", key))
		return nil, fmt.Errorf("cache get: %w", err)
	}

	var previews []*domain.TripPreview
	if err := json.Unmarshal(data, &previews); err != nil {
		c.logger.Error("cache unmarshal failed", zap.Error(err), zap.String("key", key))
		c.client.Del(ctx, key) //nolint:errcheck
		return nil, nil
	}

	c.logger.Debug("cache hit: trip previews", zap.String("driverID", driverID), zap.Int("page", pageIndex))
	return previews, nil
}

// SetTripsPreviews stocke la liste paginée des trajets dans le cache.
func (c *TripCache) SetTripsPreviews(ctx context.Context, driverID string, pageIndex int, previews []*domain.TripPreview) error {
	key := c.previewsKey(driverID, pageIndex)

	data, err := json.Marshal(previews)
	if err != nil {
		c.logger.Error("cache marshal failed", zap.Error(err), zap.String("key", key))
		return fmt.Errorf("cache marshal: %w", err)
	}

	if err := c.client.Set(ctx, key, data, previewsTTL).Err(); err != nil {
		c.logger.Error("cache set failed", zap.Error(err), zap.String("key", key))
		return fmt.Errorf("cache set: %w", err)
	}

	c.logger.Debug("cache set: trip previews", zap.String("driverID", driverID), zap.Int("page", pageIndex))
	return nil
}

// InvalidateDriverPreviews supprime toutes les pages de previews pour un conducteur.
// Utilise SCAN (non-bloquant) + pipeline DEL.
func (c *TripCache) InvalidateDriverPreviews(ctx context.Context, driverID string) {
	pattern := keyPrefix + "previews:" + driverID + ":page:*"
	var keys []string

	iter := c.client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		c.logger.Warn("cache scan failed during invalidation", zap.Error(err), zap.String("driverID", driverID))
		return
	}

	if len(keys) == 0 {
		return
	}

	if err := c.client.Del(ctx, keys...).Err(); err != nil {
		c.logger.Warn("cache invalidation failed", zap.Error(err), zap.String("driverID", driverID), zap.Int("keys", len(keys)))
		return
	}

	c.logger.Debug("cache invalidated: driver previews", zap.String("driverID", driverID), zap.Int("keys", len(keys)))
}

// =============================================================================
// Completed Previews (résultat DB brut, paginé par conducteur)
// =============================================================================

// GetCompletedTripsPreviews récupère la liste paginée des trajets complétés depuis le cache.
func (c *TripCache) GetCompletedTripsPreviews(ctx context.Context, driverID string, pageIndex int) ([]*domain.TripPreview, error) {
	key := c.completedPreviewsKey(driverID, pageIndex)

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			c.logger.Debug("cache miss: completed trip previews", zap.String("driverID", driverID), zap.Int("page", pageIndex))
			return nil, nil
		}
		c.logger.Error("cache get failed", zap.Error(err), zap.String("key", key))
		return nil, fmt.Errorf("cache get: %w", err)
	}

	var previews []*domain.TripPreview
	if err := json.Unmarshal(data, &previews); err != nil {
		c.logger.Error("cache unmarshal failed", zap.Error(err), zap.String("key", key))
		c.client.Del(ctx, key) //nolint:errcheck
		return nil, nil
	}

	c.logger.Debug("cache hit: completed trip previews", zap.String("driverID", driverID), zap.Int("page", pageIndex))
	return previews, nil
}

// SetCompletedTripsPreviews stocke la liste paginée des trajets complétés dans le cache.
func (c *TripCache) SetCompletedTripsPreviews(ctx context.Context, driverID string, pageIndex int, previews []*domain.TripPreview) error {
	key := c.completedPreviewsKey(driverID, pageIndex)

	data, err := json.Marshal(previews)
	if err != nil {
		c.logger.Error("cache marshal failed", zap.Error(err), zap.String("key", key))
		return fmt.Errorf("cache marshal: %w", err)
	}

	if err := c.client.Set(ctx, key, data, previewsTTL).Err(); err != nil {
		c.logger.Error("cache set failed", zap.Error(err), zap.String("key", key))
		return fmt.Errorf("cache set: %w", err)
	}

	c.logger.Debug("cache set: completed trip previews", zap.String("driverID", driverID), zap.Int("page", pageIndex))
	return nil
}

// InvalidateDriverCompletedPreviews supprime toutes les pages de previews complétés pour un conducteur.
func (c *TripCache) InvalidateDriverCompletedPreviews(ctx context.Context, driverID string) {
	pattern := keyPrefix + "completed-previews:" + driverID + ":page:*"
	var keys []string

	iter := c.client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		c.logger.Warn("cache scan failed during invalidation", zap.Error(err), zap.String("driverID", driverID))
		return
	}

	if len(keys) == 0 {
		return
	}

	if err := c.client.Del(ctx, keys...).Err(); err != nil {
		c.logger.Warn("cache invalidation failed", zap.Error(err), zap.String("driverID", driverID), zap.Int("keys", len(keys)))
		return
	}

	c.logger.Debug("cache invalidated: driver completed previews", zap.String("driverID", driverID), zap.Int("keys", len(keys)))
}

// =============================================================================
// Driver Name (enrichissement inter-service)
// =============================================================================

// GetDriverName récupère le nom du conducteur depuis le cache.
// Retourne ("", false, nil) si la clé n'existe pas (cache miss).
func (c *TripCache) GetDriverName(ctx context.Context, driverID string) (string, bool, error) {
	key := c.driverNameKey(driverID)

	name, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			c.logger.Debug("cache miss: driver name", zap.String("driverID", driverID))
			return "", false, nil
		}
		c.logger.Error("cache get failed", zap.Error(err), zap.String("key", key))
		return "", false, fmt.Errorf("cache get: %w", err)
	}

	c.logger.Debug("cache hit: driver name", zap.String("driverID", driverID))
	return name, true, nil
}

// SetDriverName stocke le nom du conducteur dans le cache.
func (c *TripCache) SetDriverName(ctx context.Context, driverID string, name string) error {
	key := c.driverNameKey(driverID)

	if err := c.client.Set(ctx, key, name, driverNameTTL).Err(); err != nil {
		c.logger.Error("cache set failed", zap.Error(err), zap.String("key", key))
		return fmt.Errorf("cache set: %w", err)
	}

	c.logger.Debug("cache set: driver name", zap.String("driverID", driverID))
	return nil
}

// =============================================================================
// Vehicle Info (enrichissement inter-service)
// =============================================================================

// GetVehicleInfo récupère la marque et la plaque d'un véhicule depuis le cache.
// Retourne ("", "", false, nil) si la clé n'existe pas (cache miss).
func (c *TripCache) GetVehicleInfo(ctx context.Context, vehicleID string) (brand, plate string, found bool, err error) {
	key := c.vehicleInfoKey(vehicleID)

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			c.logger.Debug("cache miss: vehicle info", zap.String("vehicleID", vehicleID))
			return "", "", false, nil
		}
		c.logger.Error("cache get failed", zap.Error(err), zap.String("key", key))
		return "", "", false, fmt.Errorf("cache get: %w", err)
	}

	var info cachedVehicleInfo
	if err := json.Unmarshal(data, &info); err != nil {
		c.logger.Error("cache unmarshal failed", zap.Error(err), zap.String("key", key))
		c.client.Del(ctx, key) //nolint:errcheck
		return "", "", false, nil
	}

	c.logger.Debug("cache hit: vehicle info", zap.String("vehicleID", vehicleID))
	return info.Brand, info.Plate, true, nil
}

// SetVehicleInfo stocke la marque et la plaque d'un véhicule dans le cache.
func (c *TripCache) SetVehicleInfo(ctx context.Context, vehicleID string, brand, plate string) error {
	key := c.vehicleInfoKey(vehicleID)

	data, err := json.Marshal(cachedVehicleInfo{Brand: brand, Plate: plate})
	if err != nil {
		c.logger.Error("cache marshal failed", zap.Error(err), zap.String("key", key))
		return fmt.Errorf("cache marshal: %w", err)
	}

	if err := c.client.Set(ctx, key, data, vehicleInfoTTL).Err(); err != nil {
		c.logger.Error("cache set failed", zap.Error(err), zap.String("key", key))
		return fmt.Errorf("cache set: %w", err)
	}

	c.logger.Debug("cache set: vehicle info", zap.String("vehicleID", vehicleID))
	return nil
}

// =============================================================================
// Seat Counter (places disponibles en temps réel)
// =============================================================================

// SetSeatCounter stocke le nombre de places disponibles d'un trajet en cache.
// Appelé par UpdateAvailableSeats après chaque réconciliation.
func (c *TripCache) SetSeatCounter(ctx context.Context, tripID string, seats int) error {
	key := c.seatCounterKey(tripID)
	if err := c.client.Set(ctx, key, seats, seatCounterTTL).Err(); err != nil {
		c.logger.Error("cache set failed", zap.Error(err), zap.String("key", key))
		return fmt.Errorf("cache set: %w", err)
	}
	c.logger.Debug("cache set: seat counter", zap.String("tripID", tripID), zap.Int("seats", seats))
	return nil
}

// GetSeatCounter retourne (seats, found, error).
// found=false si la clé n'existe pas (cache miss → fallback sur la valeur DB).
func (c *TripCache) GetSeatCounter(ctx context.Context, tripID string) (int, bool, error) {
	key := c.seatCounterKey(tripID)
	val, err := c.client.Get(ctx, key).Int()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			c.logger.Debug("cache miss: seat counter", zap.String("tripID", tripID))
			return 0, false, nil
		}
		c.logger.Error("cache get failed", zap.Error(err), zap.String("key", key))
		return 0, false, fmt.Errorf("cache get: %w", err)
	}
	c.logger.Debug("cache hit: seat counter", zap.String("tripID", tripID), zap.Int("seats", val))
	return val, true, nil
}

// =============================================================================
// Key helpers
// =============================================================================

func (c *TripCache) seatCounterKey(tripID string) string {
	return keyPrefix + tripID + ":available_seats"
}

func (c *TripCache) previewsKey(driverID string, pageIndex int) string {
	return keyPrefix + "previews:" + driverID + ":page:" + strconv.Itoa(pageIndex)
}

func (c *TripCache) driverNameKey(driverID string) string {
	return keyPrefix + "driver-name:" + driverID
}

func (c *TripCache) vehicleInfoKey(vehicleID string) string {
	return keyPrefix + "vehicle-info:" + vehicleID
}

func (c *TripCache) completedPreviewsKey(driverID string, pageIndex int) string {
	return keyPrefix + "completed-previews:" + driverID + ":page:" + strconv.Itoa(pageIndex)
}
