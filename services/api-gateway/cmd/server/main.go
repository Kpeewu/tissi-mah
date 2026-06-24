package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"

	pkgLogger "github.com/Kpeewu/tissi-mah/pkg/logger"
	"github.com/Kpeewu/tissi-mah/services/api-gateway/internal/config"
	"github.com/Kpeewu/tissi-mah/services/api-gateway/internal/gateway"
	"github.com/Kpeewu/tissi-mah/services/api-gateway/internal/middleware"
	"github.com/Kpeewu/tissi-mah/services/api-gateway/internal/server"
	firebaseValidator "github.com/Kpeewu/tissi-mah/services/api-gateway/pkg/firebase"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const serviceVersion = "1.0.0"

func main() {
	bootstrapLogger := pkgLogger.NewDefault("api-gateway")
	defer bootstrapLogger.Sync() //nolint:errcheck

	if err := run(bootstrapLogger); err != nil {
		bootstrapLogger.Fatal("service terminated with error", zap.Error(err))
	}
}

func run(bootstrapLogger *zap.Logger) error {
	// --- Config ---
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	// --- Logger ---
	logger, err := pkgLogger.New(pkgLogger.Config{
		Level:       cfg.LogLevel,
		Environment: cfg.Environment.Mode,
		ServiceName: "api-gateway",
	})
	if err != nil {
		return fmt.Errorf("logger: %w", err)
	}
	defer logger.Sync() //nolint:errcheck

	// --- Context avec arrêt gracieux ---
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// --- Redis (rate limiting) ---
	redisOpts, err := redis.ParseURL(cfg.Redis.URL)
	if err != nil {
		return fmt.Errorf("redis url parse: %w", err)
	}
	redisClient := redis.NewClient(redisOpts)
	defer redisClient.Close()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Warn("redis not reachable, rate limiting will be fault-tolerant", zap.Error(err))
	} else {
		logger.Info("connected to redis for rate limiting")
	}

	// --- Redis suspension (optionnel, pointe vers le Redis de auth-service) ---
	var suspensionRedis *redis.Client
	if cfg.SuspensionRedis.URL != "" {
		suspOpts, err := redis.ParseURL(cfg.SuspensionRedis.URL)
		if err != nil {
			return fmt.Errorf("suspension redis url parse: %w", err)
		}
		suspensionRedis = redis.NewClient(suspOpts)
		defer suspensionRedis.Close()
		if err := suspensionRedis.Ping(ctx).Err(); err != nil {
			logger.Warn("suspension redis not reachable, account suspension checks disabled", zap.Error(err))
			suspensionRedis = nil
		} else {
			logger.Info("connected to suspension redis")
		}
	}

	// --- Firebase JWT validator ---
	validator, err := firebaseValidator.NewJWTValidator(ctx, cfg.Firebase.ProjectID)
	if err != nil {
		return fmt.Errorf("firebase: %w", err)
	}
	logger.Info("firebase jwt validator initialized")

	// --- grpc-gateway mux ---
	gwMux, err := gateway.NewGatewayMux(ctx, gateway.MuxConfig{
		AuthServiceAddr:    cfg.AuthService.Address(),
		UserServiceAddr:    cfg.UserService.Address(),
		RatingServiceAddr:  cfg.RatingService.Address(),
		FileServiceAddr:    cfg.FileService.Address(),
		VehicleServiceAddr: cfg.VehicleService.Address(),
		TripsServiceAddr:   cfg.TripsService.Address(),
		KYCServiceAddr:     cfg.KYCService.Address(),
		BookingServiceAddr: cfg.BookingService.Address(),
		PaymentServiceAddr:      cfg.PaymentService.Address(),
		NotificationServiceAddr: cfg.NotificationService.Address(),
		SupportServiceAddr:      cfg.SupportService.Address(),
		GeolocationServiceAddr:  cfg.GeolocationService.Address(),
		ChatServiceAddr:         cfg.ChatService.Address(),
		InternalHMACSecret:      cfg.Security.InternalHMACSecret,
		Logger:                  logger,
	})
	if err != nil {
		return fmt.Errorf("gateway mux: %w", err)
	}

	// --- Middleware chain ---
	// Ordre : CORS → Rate Limit → JWT Firebase → grpc-gateway mux
	handler := buildHandler(cfg, gwMux, validator, redisClient, suspensionRedis, logger)

	// --- HTTP server ---
	srv := server.New(server.Config{
		Host: cfg.Server.Host,
		Port: cfg.Server.Port,
	}, handler, logger)

	logger.Info("api-gateway ready",
		zap.String("port", cfg.Server.Port),
		zap.String("auth-service", cfg.AuthService.Address()),
		zap.String("user-service", cfg.UserService.Address()),
		zap.String("rating-service", cfg.RatingService.Address()),
		zap.String("file-service", cfg.FileService.Address()),
		zap.String("vehicle-service", cfg.VehicleService.Address()),
		zap.String("trips-service", cfg.TripsService.Address()),
		zap.String("kyc-service", cfg.KYCService.Address()),
		zap.String("booking-service", cfg.BookingService.Address()),
		zap.String("payment-service", cfg.PaymentService.Address()),
	)

	return srv.Serve(ctx)
}

// buildHandler construit la chaîne de middlewares HTTP.
//
// Ordre d'exécution :
//  1. PanicRecovery   — intercepte les panics, renvoie 500 JSON
//  2. RequestID       — génère/propage X-Request-ID
//  3. SecurityHeaders — HSTS, X-Frame-Options, etc.
//  4. BodySizeLimit   — limite à 1 Mo par défaut
//  5. CORS            — gestion des preflight
//  6. JWT Firebase    — valide Bearer token Firebase (routes protégées mobile)
//  7. JWT Support     — valide Bearer token support (routes back-office)
//  8. RateLimit       — sliding window par UID (si authentifié) ou par IP
//  9. AppID           — vérifie X-App-ID contre whitelist mobile / support
// 10. mux (grpc-gateway)
func buildHandler(
	cfg *config.Config,
	gwMux http.Handler,
	validator *firebaseValidator.JWTValidator,
	redisClient *redis.Client,
	suspensionRedis *redis.Client,
	logger *zap.Logger,
) http.Handler {
	// Mux principal avec health check direct (non proxié vers les services)
	rootMux := http.NewServeMux()
	rootMux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"status":    "healthy",
			"version":   serviceVersion,
			"timestamp": time.Now().Unix(),
		})
	})
	rootMux.Handle("/", gwMux)

	// CORS
	corsOrigins := strings.Split(cfg.CORS.AllowedOrigins, ",")
	for i := range corsOrigins {
		corsOrigins[i] = strings.TrimSpace(corsOrigins[i])
	}
	corsMW := middleware.CORS(middleware.CORSConfig{
		AllowedOrigins: corsOrigins,
		Environment:    cfg.Environment.Mode,
	})

	// JWT Firebase — maintenant AVANT le rate limit pour que l'UID soit
	// disponible comme clé de rate limiting.
	// isDual : routes acceptant Firebase OU Support JWT (ex: getDocument).
	jwtMW := middleware.JWTFirebase(
		validator,
		func(path string) bool { return gateway.ProtectedRoutes[path] },
		func(path string) bool { return gateway.DualProtectedRoutes[path] },
		suspensionRedis,
		logger,
	)

	// JWT Support (back-office admin / agents) — canal d'auth séparé de Firebase.
	// Couvre SupportProtectedRoutes + DualProtectedRoutes.
	jwtSupportMW := middleware.JWTSupport(cfg.SupportJWTSecret, func(path string) bool {
		return gateway.SupportProtectedRoutes[path] || gateway.DualProtectedRoutes[path]
	}, logger)

	// Rate limiting — clé hybride UID (si JWT précédent a setté x-firebase-uid)
	// ou IP (routes publiques / anonymes).
	rateLimitMW := middleware.RateLimit(middleware.RateLimitConfig{
		Tiers: map[string]middleware.RateLimitTierConfig{
			"global": {
				Minute: cfg.RateLimit.GlobalMinute,
				Hour:   cfg.RateLimit.GlobalHour,
			},
			"auth": {
				Second: cfg.RateLimit.AuthSecond,
				Minute: cfg.RateLimit.AuthMinute,
				Hour:   cfg.RateLimit.AuthHour,
			},
			"create": {
				Minute: cfg.RateLimit.CreateAccountMinute,
				Hour:   cfg.RateLimit.CreateAccountHour,
			},
			"sensitive": {
				Minute: cfg.RateLimit.SensitiveMinute,
				Hour:   cfg.RateLimit.SensitiveHour,
			},
		},
		GetTier: func(path string) string {
			if tier, ok := gateway.RouteRateLimitConfig[path]; ok {
				return string(tier)
			}
			return string(gateway.TierGlobal)
		},
		// GetUID lit x-firebase-uid (mobile) ou x-support-uid (back-office).
		// Sans UID (routes publiques), on tombe sur la clé IP.
		GetUID: func(r *http.Request) string {
			if uid := r.Header.Get("x-firebase-uid"); uid != "" {
				return uid
			}
			return r.Header.Get("x-support-uid")
		},
		RedisClient: redisClient,
		FailClosed:  cfg.Security.RateLimitFailClosed,
		Logger:      logger,
	})

	// AppID — vérifie X-App-ID sur les routes protégées Firebase et Support.
	mobileIDs := middleware.ParseAppIDs(cfg.AppID.MobileAppIDs)
	supportIDs := middleware.ParseAppIDs(cfg.AppID.SupportAppIDs)
	appIDMW := middleware.AppID(
		mobileIDs,
		supportIDs,
		func(path string) bool { return gateway.ProtectedRoutes[path] || gateway.DualProtectedRoutes[path] },
		func(path string) bool { return gateway.SupportProtectedRoutes[path] || gateway.DualProtectedRoutes[path] },
		logger,
	)

	// Stack complet :
	// panic → reqID → secHeaders → bodySize → cors → jwt → jwtSupport → rateLimit → appID → mux
	return middleware.PanicRecovery(logger)(
		middleware.RequestID()(
			middleware.SecurityHeaders(cfg.Security.EnableHSTS)(
				middleware.BodySizeLimit(cfg.Security.BodySizeMaxBytes)(
					corsMW(
						jwtMW(jwtSupportMW(
							rateLimitMW(
								appIDMW(rootMux)))))))))
}
