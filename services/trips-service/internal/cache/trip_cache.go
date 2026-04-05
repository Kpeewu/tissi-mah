package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Kpeewu/tissi-mah/services/trips-service/internal/domain"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// cachedSearchResult est la structure JSON stockée pour les résultats de recherche.
type cachedSearchResult struct {
	Previews   []*domain.TripPreview `json:"previews"`
	TotalCount int                   `json:"total_count"`
}

const (
	// TTL du cache pour les résultats de recherche passager
	searchResultsTTL = 60 * time.Second

	// TTL du cache pour la liste paginée des trajets d'un conducteur
	previewsTTL = 2 * time.Minute

	// TTL du cache pour le nom du conducteur (change très rarement)
	driverNameTTL = 15 * time.Minute

	// TTL du cache pour les infos véhicule (marque/plaque, changent rarement)
	vehicleInfoTTL = 10 * time.Minute

	// TTL du compteur de places disponibles — plus long que l'intervalle de réconciliation
	seatCounterTTL = 24 * time.Hour

	// TTL du cache pour les infos enrichies du conducteur (nom + photo)
	driverInfoTTL = 15 * time.Minute

	// TTL du cache pour la note moyenne du conducteur
	driverRatingTTL = 5 * time.Minute

	// Préfixe des clés Redis pour le trips-service
	keyPrefix = "trip:"
)

// cachedVehicleInfo est la structure JSON stockée pour les infos véhicule.
type cachedVehicleInfo struct {
	Brand string `json:"brand"`
	Plate string `json:"plate"`
}

// cachedDriverInfo est la structure JSON stockée pour les infos enrichies du conducteur.
type cachedDriverInfo struct {
	Name            string `json:"name"`
	ProfileImageURL string `json:"profile_image_url"`
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

func (c *TripCache) driverInfoKey(driverID string) string {
	return keyPrefix + "driver-info:" + driverID
}

func (c *TripCache) driverRatingKey(driverID string) string {
	return keyPrefix + "driver-rating:" + driverID
}

// =============================================================================
// Driver Info (nom + photo, enrichissement pour la recherche)
// =============================================================================

// GetDriverInfo récupère les infos enrichies du conducteur depuis le cache.
func (c *TripCache) GetDriverInfo(ctx context.Context, driverID string) (string, string, bool, error) {
	key := c.driverInfoKey(driverID)

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", "", false, nil
		}
		return "", "", false, fmt.Errorf("cache get: %w", err)
	}

	var info cachedDriverInfo
	if err := json.Unmarshal(data, &info); err != nil {
		c.client.Del(ctx, key) //nolint:errcheck
		return "", "", false, nil
	}

	return info.Name, info.ProfileImageURL, true, nil
}

// SetDriverInfo stocke les infos enrichies du conducteur dans le cache.
func (c *TripCache) SetDriverInfo(ctx context.Context, driverID, name, profileImageURL string) error {
	key := c.driverInfoKey(driverID)

	data, err := json.Marshal(cachedDriverInfo{Name: name, ProfileImageURL: profileImageURL})
	if err != nil {
		return fmt.Errorf("cache marshal: %w", err)
	}

	return c.client.Set(ctx, key, data, driverInfoTTL).Err()
}

// =============================================================================
// Driver Rating (note moyenne, enrichissement pour la recherche)
// =============================================================================

// GetDriverRating récupère la note moyenne du conducteur depuis le cache.
func (c *TripCache) GetDriverRating(ctx context.Context, driverID string) (float64, bool, error) {
	key := c.driverRatingKey(driverID)

	val, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("cache get: %w", err)
	}

	rating, err := strconv.ParseFloat(val, 64)
	if err != nil {
		c.client.Del(ctx, key) //nolint:errcheck
		return 0, false, nil
	}

	return rating, true, nil
}

// SetDriverRating stocke la note moyenne du conducteur dans le cache.
func (c *TripCache) SetDriverRating(ctx context.Context, driverID string, average float64) error {
	key := c.driverRatingKey(driverID)
	return c.client.Set(ctx, key, fmt.Sprintf("%.1f", average), driverRatingTTL).Err()
}

// =============================================================================
// Search Results (résultats de recherche passager, TTL 60s)
// =============================================================================

// GetSearchResults récupère les résultats de recherche depuis le cache.
// Retourne (nil, 0, nil) si la clé n'existe pas (cache miss).
func (c *TripCache) GetSearchResults(ctx context.Context, cacheKey string) ([]*domain.TripPreview, int, error) {
	data, err := c.client.Get(ctx, cacheKey).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			c.logger.Debug("cache miss: search results", zap.String("key", cacheKey))
			return nil, 0, nil
		}
		c.logger.Error("cache get failed", zap.Error(err), zap.String("key", cacheKey))
		return nil, 0, fmt.Errorf("cache get: %w", err)
	}

	var result cachedSearchResult
	if err := json.Unmarshal(data, &result); err != nil {
		c.logger.Error("cache unmarshal failed", zap.Error(err), zap.String("key", cacheKey))
		c.client.Del(ctx, cacheKey) //nolint:errcheck
		return nil, 0, nil
	}

	c.logger.Debug("cache hit: search results", zap.String("key", cacheKey))
	return result.Previews, result.TotalCount, nil
}

// SetSearchResults stocke les résultats de recherche dans le cache.
func (c *TripCache) SetSearchResults(ctx context.Context, cacheKey string, previews []*domain.TripPreview, totalCount int) error {
	data, err := json.Marshal(cachedSearchResult{Previews: previews, TotalCount: totalCount})
	if err != nil {
		c.logger.Error("cache marshal failed", zap.Error(err), zap.String("key", cacheKey))
		return fmt.Errorf("cache marshal: %w", err)
	}

	if err := c.client.Set(ctx, cacheKey, data, searchResultsTTL).Err(); err != nil {
		c.logger.Error("cache set failed", zap.Error(err), zap.String("key", cacheKey))
		return fmt.Errorf("cache set: %w", err)
	}

	c.logger.Debug("cache set: search results", zap.String("key", cacheKey))
	return nil
}

// BuildSearchCacheKey génère une clé de cache normalisée à partir des paramètres de recherche.
// Arrondit lng/lat à 3 décimales (~111m) pour regrouper les requêtes proches.
func BuildSearchCacheKey(
	passengerLng, passengerLat *float64,
	distanceRangeMeters int,
	departureLocationName, arrivalLocationName string,
	tripStartDate, tripStartHour, tripArrivalHour *string,
	pageIndex int,
) string {
	parts := make([]string, 0, 10)

	// Normaliser les noms en minuscules pour le hash
	parts = append(parts, "dep="+strings.ToLower(departureLocationName))
	parts = append(parts, "arr="+strings.ToLower(arrivalLocationName))

	if passengerLng != nil && passengerLat != nil {
		// Arrondir à 3 décimales (~111m de précision)
		lng := math.Round(*passengerLng*1000) / 1000
		lat := math.Round(*passengerLat*1000) / 1000
		parts = append(parts, fmt.Sprintf("lng=%.3f", lng))
		parts = append(parts, fmt.Sprintf("lat=%.3f", lat))
		parts = append(parts, fmt.Sprintf("dist=%d", distanceRangeMeters))
	}

	if tripStartDate != nil && *tripStartDate != "" {
		parts = append(parts, "date="+*tripStartDate)
	}
	if tripStartHour != nil && *tripStartHour != "" {
		parts = append(parts, "sh="+*tripStartHour)
	}
	if tripArrivalHour != nil && *tripArrivalHour != "" {
		parts = append(parts, "ah="+*tripArrivalHour)
	}

	parts = append(parts, fmt.Sprintf("p=%d", pageIndex))

	sort.Strings(parts)
	raw := strings.Join(parts, "|")

	hash := sha256.Sum256([]byte(raw))
	return keyPrefix + "search:" + hex.EncodeToString(hash[:])
}
