// geolocation-service — routing OSRM + geocoding Nominatim + cache Redis.
// Service stateless appelé par le front pendant la création d'un trajet.
package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	pkgDatabase "github.com/Kpeewu/tissi-mah/pkg/database"
	pkgLogger "github.com/Kpeewu/tissi-mah/pkg/logger"
	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/cache"
	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/client/nominatim"
	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/client/osrm"
	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/config"
	grpcServer "github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/grpc"
	"github.com/Kpeewu/tissi-mah/services/geolocation-service/internal/service"
	"go.uber.org/zap"
)

func main() {
	bootstrapLogger := pkgLogger.NewDefault("geolocation-service")
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
		ServiceName: "geolocation-service",
	})
	if err != nil {
		return fmt.Errorf("logger: %w", err)
	}
	defer logger.Sync() //nolint:errcheck

	// --- Context avec arrêt gracieux ---
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// --- OSRM client (timeout + semaphore + circuit breaker actifs dès le départ) ---
	osrmClient := osrm.New(osrm.DefaultConfig(cfg.OSRM.URL), logger)
	logger.Info("osrm client ready", zap.String("base_url", cfg.OSRM.URL))

	// --- Nominatim client (optionnel — si NOMINATIM_URL vide, le client est nil
	// et le service renvoie ErrorGeocodingUnavailable sur Geocode/ReverseGeocode) ---
	nominatimClient := nominatim.New(nominatim.DefaultConfig(cfg.Nominatim.URL), logger)
	if nominatimClient != nil {
		logger.Info("nominatim client ready", zap.String("base_url", cfg.Nominatim.URL))
	} else {
		logger.Warn("nominatim disabled (NOMINATIM_URL empty), geocoding endpoints will return ErrorGeocodingUnavailable")
	}

	// --- Redis cache (graceful degradation si indisponible) ---
	redisClient, err := pkgDatabase.NewRedisClientFromURL(ctx, cfg.Redis.URL)
	var geoCache *cache.Cache
	if err != nil {
		logger.Warn("redis not reachable, cache disabled", zap.Error(err))
		geoCache = cache.New(nil, logger)
	} else {
		logger.Info("connected to redis for caching")
		geoCache = cache.New(redisClient, logger)
		defer redisClient.Close() //nolint:errcheck
	}

	// --- Service ---
	geoService := service.NewGeolocationService(osrmClient, nominatimClient, geoCache, cfg.Geocode.DefaultCountries, logger)

	// --- gRPC server ---
	srv, err := grpcServer.NewGeolocationServer(cfg, geoService, logger)
	if err != nil {
		return fmt.Errorf("grpc server: %w", err)
	}

	logger.Info("geolocation-service ready",
		zap.String("port", cfg.Server.Port),
		zap.String("osrm", cfg.OSRM.URL),
		zap.String("nominatim", cfg.Nominatim.URL),
	)

	return srv.Serve(ctx)
}
