package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RateLimitTierConfig contient les limites pour un tier donné
type RateLimitTierConfig struct {
	Second int // 0 = pas de limite par seconde
	Minute int
	Hour   int
}

// RateLimitConfig contient la configuration complète du rate limiting
type RateLimitConfig struct {
	Tiers        map[string]RateLimitTierConfig
	GetTier      func(path string) string // retourne le tier pour un path donné
	RedisClient  *redis.Client
	Logger       *zap.Logger
}

// RateLimit retourne un middleware HTTP qui applique le rate limiting Redis distribué.
// Pattern : sliding window avec INCR + EXPIRE par IP et par fenêtre temporelle.
// Fault-tolerant : si Redis est indisponible, la requête passe.
func RateLimit(cfg RateLimitConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tier := cfg.GetTier(r.URL.Path)
			tierCfg, ok := cfg.Tiers[tier]
			if !ok {
				// Pas de config pour ce tier, laisser passer
				next.ServeHTTP(w, r)
				return
			}

			ip := extractIP(r)
			ctx := r.Context()

			// Vérifier chaque fenêtre temporelle (seconde, minute, heure)
			if tierCfg.Second > 0 {
				if blocked := checkWindow(ctx, cfg.RedisClient, cfg.Logger, w, tier, ip, "second", tierCfg.Second, time.Second); blocked {
					return
				}
			}

			if tierCfg.Minute > 0 {
				if blocked := checkWindow(ctx, cfg.RedisClient, cfg.Logger, w, tier, ip, "minute", tierCfg.Minute, time.Minute); blocked {
					return
				}
			}

			if tierCfg.Hour > 0 {
				if blocked := checkWindow(ctx, cfg.RedisClient, cfg.Logger, w, tier, ip, "hour", tierCfg.Hour, time.Hour); blocked {
					return
				}
			}

			// Incrémenter les compteurs (fire-and-forget, fault-tolerant)
			incrementCounters(ctx, cfg.RedisClient, cfg.Logger, tier, ip, tierCfg)

			// Set les headers de rate limiting (fenêtre minute comme principal)
			if tierCfg.Minute > 0 {
				remaining := getRemainingQuota(ctx, cfg.RedisClient, cfg.Logger, tier, ip, "minute", tierCfg.Minute, time.Minute)
				w.Header().Set("X-RateLimit-Limit-Minute", strconv.Itoa(tierCfg.Minute))
				w.Header().Set("X-RateLimit-Remaining-Minute", strconv.Itoa(remaining))
			}
			if tierCfg.Hour > 0 {
				remaining := getRemainingQuota(ctx, cfg.RedisClient, cfg.Logger, tier, ip, "hour", tierCfg.Hour, time.Hour)
				w.Header().Set("X-RateLimit-Limit-Hour", strconv.Itoa(tierCfg.Hour))
				w.Header().Set("X-RateLimit-Remaining-Hour", strconv.Itoa(remaining))
			}

			next.ServeHTTP(w, r)
		})
	}
}

// checkWindow vérifie si la limite est atteinte pour une fenêtre donnée.
// Retourne true si la requête est bloquée (429).
func checkWindow(ctx context.Context, rdb *redis.Client, logger *zap.Logger, w http.ResponseWriter, tier, ip, window string, limit int, duration time.Duration) bool {
	key := rateLimitKey(tier, ip, window, duration)
	count, err := rdb.Get(ctx, key).Int()
	if err != nil && err != redis.Nil {
		// Redis indisponible — fault-tolerant, laisser passer
		logger.Warn("rate limit redis error, allowing request", zap.Error(err))
		return false
	}

	if count >= limit {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", strconv.FormatInt(int64(duration.Seconds()), 10))
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"message":"API rate limit exceeded"}`)) //nolint:errcheck
		return true
	}

	return false
}

// incrementCounters incrémente les compteurs Redis pour toutes les fenêtres actives
func incrementCounters(ctx context.Context, rdb *redis.Client, logger *zap.Logger, tier, ip string, cfg RateLimitTierConfig) {
	windows := []struct {
		name     string
		duration time.Duration
		limit    int
	}{
		{"second", time.Second, cfg.Second},
		{"minute", time.Minute, cfg.Minute},
		{"hour", time.Hour, cfg.Hour},
	}

	for _, w := range windows {
		if w.limit <= 0 {
			continue
		}
		key := rateLimitKey(tier, ip, w.name, w.duration)
		pipe := rdb.Pipeline()
		pipe.Incr(ctx, key)
		pipe.Expire(ctx, key, w.duration)
		if _, err := pipe.Exec(ctx); err != nil {
			logger.Warn("rate limit increment failed", zap.String("key", key), zap.Error(err))
		}
	}
}

// getRemainingQuota retourne le nombre de requêtes restantes pour une fenêtre
func getRemainingQuota(ctx context.Context, rdb *redis.Client, logger *zap.Logger, tier, ip, window string, limit int, duration time.Duration) int {
	key := rateLimitKey(tier, ip, window, duration)
	count, err := rdb.Get(ctx, key).Int()
	if err != nil {
		if err != redis.Nil {
			logger.Warn("rate limit get remaining failed", zap.Error(err))
		}
		return limit
	}
	remaining := limit - count
	if remaining < 0 {
		return 0
	}
	return remaining
}

// rateLimitKey génère la clé Redis pour un compteur de rate limiting
func rateLimitKey(tier, ip, window string, duration time.Duration) string {
	// Fenêtre temporelle alignée (ex: minute courante)
	now := time.Now().Unix()
	windowStart := now - (now % int64(duration.Seconds()))
	return fmt.Sprintf("ratelimit:%s:%s:%s:%d", tier, ip, window, windowStart)
}

// extractIP extrait l'adresse IP du client depuis la requête
func extractIP(r *http.Request) string {
	// X-Forwarded-For (derrière un proxy/load balancer)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Prendre la première IP (client original)
		if idx := len(xff); idx > 0 {
			for i, c := range xff {
				if c == ',' {
					return xff[:i]
				}
			}
			return xff
		}
	}

	// X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// RemoteAddr (fallback)
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
