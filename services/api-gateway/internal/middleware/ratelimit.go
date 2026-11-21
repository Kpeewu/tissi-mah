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
	Tiers       map[string]RateLimitTierConfig
	GetTier     func(path string) string // retourne le tier pour un path donné
	// GetUID extrait le Firebase UID depuis la requête (header x-firebase-uid
	// positionné par le middleware JWT Firebase, qui précède le rate limiter
	// dans la chaîne). Si non nil et que l'UID est disponible, la clé Redis
	// est basée sur l'UID au lieu de l'IP, ce qui évite que 2 utilisateurs
	// derrière le même proxy partagent le même compteur.
	GetUID      func(r *http.Request) string
	RedisClient *redis.Client
	Logger      *zap.Logger
	// FailClosed : si true et que Redis est indisponible, bloquer les requêtes
	// avec 503 au lieu de les laisser passer (fail-open). À activer en prod/staging.
	FailClosed  bool
}

// RateLimit retourne un middleware HTTP qui applique le rate limiting Redis distribué.
// Pattern : sliding window avec INCR + EXPIRE par sujet (UID si authentifié, IP sinon)
// et par fenêtre temporelle.
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
			uid := ""
			if cfg.GetUID != nil {
				uid = cfg.GetUID(r)
			}
			ctx := r.Context()

			// Vérifier chaque fenêtre temporelle (seconde, minute, heure)
			if tierCfg.Second > 0 {
				if blocked := checkWindow(ctx, cfg.RedisClient, cfg.Logger, cfg.FailClosed, w, tier, ip, uid, "second", tierCfg.Second, time.Second); blocked {
					return
				}
			}

			if tierCfg.Minute > 0 {
				if blocked := checkWindow(ctx, cfg.RedisClient, cfg.Logger, cfg.FailClosed, w, tier, ip, uid, "minute", tierCfg.Minute, time.Minute); blocked {
					return
				}
			}

			if tierCfg.Hour > 0 {
				if blocked := checkWindow(ctx, cfg.RedisClient, cfg.Logger, cfg.FailClosed, w, tier, ip, uid, "hour", tierCfg.Hour, time.Hour); blocked {
					return
				}
			}

			// Incrémenter les compteurs (fire-and-forget, fault-tolerant)
			incrementCounters(ctx, cfg.RedisClient, cfg.Logger, tier, ip, uid, tierCfg)

			// Set les headers de rate limiting (fenêtre minute comme principal)
			if tierCfg.Minute > 0 {
				remaining := getRemainingQuota(ctx, cfg.RedisClient, cfg.Logger, tier, ip, uid, "minute", tierCfg.Minute, time.Minute)
				w.Header().Set("X-RateLimit-Limit-Minute", strconv.Itoa(tierCfg.Minute))
				w.Header().Set("X-RateLimit-Remaining-Minute", strconv.Itoa(remaining))
			}
			if tierCfg.Hour > 0 {
				remaining := getRemainingQuota(ctx, cfg.RedisClient, cfg.Logger, tier, ip, uid, "hour", tierCfg.Hour, time.Hour)
				w.Header().Set("X-RateLimit-Limit-Hour", strconv.Itoa(tierCfg.Hour))
				w.Header().Set("X-RateLimit-Remaining-Hour", strconv.Itoa(remaining))
			}

			next.ServeHTTP(w, r)
		})
	}
}

// checkWindow vérifie si la limite est atteinte pour une fenêtre donnée.
// Retourne true si la requête est bloquée.
// Si failClosed est true et Redis est indisponible, bloque avec 503.
// Sinon (fail-open), laisse passer en loggant un warning.
func checkWindow(ctx context.Context, rdb *redis.Client, logger *zap.Logger, failClosed bool, w http.ResponseWriter, tier, ip, uid, window string, limit int, duration time.Duration) bool {
	key := rateLimitKey(tier, ip, uid, window, duration)
	count, err := rdb.Get(ctx, key).Int()
	if err != nil && err != redis.Nil {
		if failClosed {
			logger.Warn("rate limit redis unavailable, blocking (fail-closed)", zap.Error(err))
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "5")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"message":"service temporarily unavailable"}`)) //nolint:errcheck
			return true
		}
		logger.Warn("rate limit redis error, allowing request (fail-open)", zap.Error(err))
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
func incrementCounters(ctx context.Context, rdb *redis.Client, logger *zap.Logger, tier, ip, uid string, cfg RateLimitTierConfig) {
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
		key := rateLimitKey(tier, ip, uid, w.name, w.duration)
		pipe := rdb.Pipeline()
		pipe.Incr(ctx, key)
		pipe.Expire(ctx, key, w.duration)
		if _, err := pipe.Exec(ctx); err != nil {
			logger.Warn("rate limit increment failed", zap.String("key", key), zap.Error(err))
		}
	}
}

// getRemainingQuota retourne le nombre de requêtes restantes pour une fenêtre
func getRemainingQuota(ctx context.Context, rdb *redis.Client, logger *zap.Logger, tier, ip, uid, window string, limit int, duration time.Duration) int {
	key := rateLimitKey(tier, ip, uid, window, duration)
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

// rateLimitKey génère la clé Redis pour un compteur de rate limiting.
// Si un Firebase UID est disponible (requête authentifiée), il sert de sujet
// principal pour éviter que 2 utilisateurs derrière le même proxy partagent
// le même compteur. Sinon, l'IP est utilisée.
func rateLimitKey(tier, ip, uid, window string, duration time.Duration) string {
	subject := ip
	if uid != "" {
		subject = "uid:" + uid
	}
	now := time.Now().Unix()
	windowStart := now - (now % int64(duration.Seconds()))
	return fmt.Sprintf("ratelimit:%s:%s:%s:%d", tier, subject, window, windowStart)
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
