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

	// Vérifier la connexion Redis
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Warn("redis not reachable, rate limiting will be fault-tolerant", zap.Error(err))
	} else {
		logger.Info("connected to redis for rate limiting")
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
		Logger:             logger,
	})
	if err != nil {
		return fmt.Errorf("gateway mux: %w", err)
	}

	// --- Middleware chain ---
	// Ordre : CORS → Rate Limit → JWT Firebase → grpc-gateway mux
	handler := buildHandler(cfg, gwMux, validator, redisClient, logger)

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
	)

	return srv.Serve(ctx)
}

// buildHandler construit la chaîne de middlewares HTTP
func buildHandler(
	cfg *config.Config,
	gwMux http.Handler,
	validator *firebaseValidator.JWTValidator,
	redisClient *redis.Client,
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

	// Rate limiting
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
		RedisClient: redisClient,
		Logger:      logger,
	})

	// JWT Firebase
	jwtMW := middleware.JWTFirebase(validator, func(path string) bool {
		return gateway.ProtectedRoutes[path]
	}, logger)

	// Chain : CORS → Rate Limit → JWT → handler
	return corsMW(rateLimitMW(jwtMW(rootMux)))
}
