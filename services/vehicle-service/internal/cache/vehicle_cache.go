package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/domain"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// TTL du cache pour les infos d'un véhicule
	vehicleInfoTTL = 5 * time.Minute

	// Préfixe des clés Redis pour le vehicle-service
	keyPrefix = "vehicle:"
)

// VehicleCache gère le cache Redis pour les données des véhicules.
type VehicleCache struct {
	client *redis.Client
	logger *zap.Logger
}

// NewVehicleCache crée une instance du cache véhicule.
func NewVehicleCache(client *redis.Client, logger *zap.Logger) *VehicleCache {
	return &VehicleCache{client: client, logger: logger}
}

// GetVehicle récupère les infos d'un véhicule depuis le cache.
// Retourne nil, nil si la clé n'existe pas (cache miss).
// Seules les infos véhicule sont cachées — jamais les documents, qui sont gérés
// par file-service et peuvent changer indépendamment (évite toute staleness).
func (c *VehicleCache) GetVehicle(ctx context.Context, vehicleID string) (*domain.Vehicle, error) {
	key := c.infoKey(vehicleID)

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			c.logger.Debug("cache miss: vehicle", zap.String("vehicleID", vehicleID))
			return nil, nil
		}
		c.logger.Error("cache get failed", zap.Error(err), zap.String("key", key))
		return nil, fmt.Errorf("cache get: %w", err)
	}

	var vehicle domain.Vehicle
	if err := json.Unmarshal(data, &vehicle); err != nil {
		c.logger.Error("cache unmarshal failed", zap.Error(err), zap.String("vehicleID", vehicleID))
		// Clé corrompue — on l'efface et on traite comme un miss
		c.client.Del(ctx, key) //nolint:errcheck
		return nil, nil
	}

	c.logger.Debug("cache hit: vehicle", zap.String("vehicleID", vehicleID))
	return &vehicle, nil
}

// SetVehicle stocke les infos d'un véhicule dans le cache.
func (c *VehicleCache) SetVehicle(ctx context.Context, vehicleID string, vehicle *domain.Vehicle) error {
	key := c.infoKey(vehicleID)

	data, err := json.Marshal(vehicle)
	if err != nil {
		c.logger.Error("cache marshal failed", zap.Error(err), zap.String("vehicleID", vehicleID))
		return fmt.Errorf("cache marshal: %w", err)
	}

	if err := c.client.Set(ctx, key, data, vehicleInfoTTL).Err(); err != nil {
		c.logger.Error("cache set failed", zap.Error(err), zap.String("key", key))
		return fmt.Errorf("cache set: %w", err)
	}

	c.logger.Debug("cache set: vehicle", zap.String("vehicleID", vehicleID))
	return nil
}

// InvalidateVehicle supprime les entrées de cache liées à un véhicule.
// Appelé lors d'une mise à jour, suppression ou changement de statut de vérification.
func (c *VehicleCache) InvalidateVehicle(ctx context.Context, vehicleID string) {
	key := c.infoKey(vehicleID)
	if err := c.client.Del(ctx, key).Err(); err != nil {
		c.logger.Warn("cache invalidation failed", zap.Error(err), zap.String("vehicleID", vehicleID))
		return
	}
	c.logger.Debug("cache invalidated: vehicle", zap.String("vehicleID", vehicleID))
}

// infoKey retourne la clé Redis pour les infos d'un véhicule.
func (c *VehicleCache) infoKey(vehicleID string) string {
	return keyPrefix + "info:" + vehicleID
}
