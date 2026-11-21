package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Kpeewu/tissi-mah/services/rating-service/internal/domain"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// TTL du cache pour les notes d'un utilisateur
	userRatingsTTL = 5 * time.Minute

	// TTL du cache pour la moyenne des notes d'un utilisateur
	userAverageTTL = 5 * time.Minute

	// Préfixe des clés Redis pour le rating-service
	keyPrefix = "rating:"
)

// UserAverage représente la moyenne et le total des notes en cache.
type UserAverage struct {
	Average      float64 `json:"average"`
	TotalRatings int32   `json:"total_ratings"`
}

// RatingCache gère le cache Redis pour les données des notes.
type RatingCache struct {
	client *redis.Client
	logger *zap.Logger
}

// NewRatingCache crée une instance du cache rating.
func NewRatingCache(client *redis.Client, logger *zap.Logger) *RatingCache {
	return &RatingCache{client: client, logger: logger}
}

// GetUserRatings récupère les notes d'un utilisateur depuis le cache.
// Retourne nil, nil si la clé n'existe pas (cache miss).
func (c *RatingCache) GetUserRatings(ctx context.Context, userRatedID string) ([]*domain.Rating, error) {
	key := c.userRatingsKey(userRatedID)

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			c.logger.Debug("cache miss: user ratings", zap.String("userRatedID", userRatedID))
			return nil, nil
		}
		c.logger.Error("cache get failed", zap.Error(err), zap.String("key", key))
		return nil, fmt.Errorf("cache get: %w", err)
	}

	var ratings []*domain.Rating
	if err := json.Unmarshal(data, &ratings); err != nil {
		c.logger.Error("cache unmarshal failed", zap.Error(err), zap.String("userRatedID", userRatedID))
		c.client.Del(ctx, key) //nolint:errcheck
		return nil, nil
	}

	c.logger.Debug("cache hit: user ratings", zap.String("userRatedID", userRatedID))
	return ratings, nil
}

// SetUserRatings stocke les notes d'un utilisateur dans le cache.
func (c *RatingCache) SetUserRatings(ctx context.Context, userRatedID string, ratings []*domain.Rating) error {
	key := c.userRatingsKey(userRatedID)

	data, err := json.Marshal(ratings)
	if err != nil {
		c.logger.Error("cache marshal failed", zap.Error(err), zap.String("userRatedID", userRatedID))
		return fmt.Errorf("cache marshal: %w", err)
	}

	if err := c.client.Set(ctx, key, data, userRatingsTTL).Err(); err != nil {
		c.logger.Error("cache set failed", zap.Error(err), zap.String("key", key))
		return fmt.Errorf("cache set: %w", err)
	}

	c.logger.Debug("cache set: user ratings", zap.String("userRatedID", userRatedID))
	return nil
}

// GetUserAverage récupère la moyenne des notes depuis le cache.
// Retourne nil, nil si la clé n'existe pas (cache miss).
func (c *RatingCache) GetUserAverage(ctx context.Context, userRatedID string) (*UserAverage, error) {
	key := c.userAverageKey(userRatedID)

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			c.logger.Debug("cache miss: user average", zap.String("userRatedID", userRatedID))
			return nil, nil
		}
		c.logger.Error("cache get failed", zap.Error(err), zap.String("key", key))
		return nil, fmt.Errorf("cache get: %w", err)
	}

	var avg UserAverage
	if err := json.Unmarshal(data, &avg); err != nil {
		c.logger.Error("cache unmarshal failed", zap.Error(err), zap.String("userRatedID", userRatedID))
		c.client.Del(ctx, key) //nolint:errcheck
		return nil, nil
	}

	c.logger.Debug("cache hit: user average", zap.String("userRatedID", userRatedID))
	return &avg, nil
}

// SetUserAverage stocke la moyenne des notes dans le cache.
func (c *RatingCache) SetUserAverage(ctx context.Context, userRatedID string, average float64, totalRatings int32) error {
	key := c.userAverageKey(userRatedID)

	data, err := json.Marshal(UserAverage{Average: average, TotalRatings: totalRatings})
	if err != nil {
		c.logger.Error("cache marshal failed", zap.Error(err), zap.String("userRatedID", userRatedID))
		return fmt.Errorf("cache marshal: %w", err)
	}

	if err := c.client.Set(ctx, key, data, userAverageTTL).Err(); err != nil {
		c.logger.Error("cache set failed", zap.Error(err), zap.String("key", key))
		return fmt.Errorf("cache set: %w", err)
	}

	c.logger.Debug("cache set: user average", zap.String("userRatedID", userRatedID))
	return nil
}

// InvalidateUser supprime les entrées de cache liées à un utilisateur noté.
// Appelé lors d'un nouveau rating ou d'une mise à jour.
func (c *RatingCache) InvalidateUser(ctx context.Context, userRatedID string) {
	keys := []string{
		c.userRatingsKey(userRatedID),
		c.userAverageKey(userRatedID),
	}
	if err := c.client.Del(ctx, keys...).Err(); err != nil {
		c.logger.Warn("cache invalidation failed", zap.Error(err), zap.String("userRatedID", userRatedID))
		return
	}
	c.logger.Debug("cache invalidated: user", zap.String("userRatedID", userRatedID))
}

func (c *RatingCache) userRatingsKey(userRatedID string) string {
	return keyPrefix + "user:" + userRatedID
}

func (c *RatingCache) userAverageKey(userRatedID string) string {
	return keyPrefix + "average:" + userRatedID
}
