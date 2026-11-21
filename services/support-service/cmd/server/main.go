package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	pkgDatabase "github.com/Kpeewu/tissi-mah/pkg/database"
	pkgLogger "github.com/Kpeewu/tissi-mah/pkg/logger"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/config"
	grpcsrv "github.com/Kpeewu/tissi-mah/services/support-service/internal/grpc"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/otp"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/repository/implementations"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/service"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/token"
	"go.uber.org/zap"
)

func main() {
	bootstrapLogger := pkgLogger.NewDefault("support-service")
	defer bootstrapLogger.Sync() //nolint:errcheck

	if err := run(bootstrapLogger); err != nil {
		bootstrapLogger.Fatal("service terminated with error", zap.Error(err))
	}
}

func run(bootstrapLogger *zap.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	logger, err := pkgLogger.New(pkgLogger.Config{
		Level:       cfg.LogLevel,
		Environment: cfg.Environment.Mode,
		ServiceName: "support-service",
	})
	if err != nil {
		return fmt.Errorf("logger: %w", err)
	}
	defer logger.Sync() //nolint:errcheck

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pkgDatabase.NewPostgresPoolFromURL(ctx, cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	defer pool.Close()
	logger.Info("connected to postgres")

	redisClient, err := pkgDatabase.NewRedisClientFromURL(ctx, cfg.Redis.URL)
	if err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	defer redisClient.Close()
	logger.Info("connected to redis")

	emailClient, err := client.NewEmailClient(cfg.EmailService.Address, cfg.EmailService.Port, logger)
	if err != nil {
		return fmt.Errorf("email-service client: %w", err)
	}
	defer emailClient.Close() //nolint:errcheck

	readRepo := implementations.NewSupportUserReadRepository(pool, logger)
	writeRepo := implementations.NewSupportUserWriteRepository(pool, logger)

	otpStore := otp.NewStore(
		redisClient,
		time.Duration(cfg.OTP.TTLSeconds)*time.Second,
		time.Duration(cfg.OTP.ResendCooldownSec)*time.Second,
		time.Duration(cfg.RateLimit.FailWindowSeconds)*time.Second,
	)
	jwtSigner := token.NewJWTSigner(cfg.JWT.Secret, cfg.JWT.AccessTTLHours)
	refreshStore := token.NewRefreshStore(redisClient, cfg.JWT.RefreshTTLHours)
	resetStore := token.NewResetStore(redisClient, time.Duration(cfg.PasswordReset.TTLSeconds)*time.Second)

	supportSvc := service.NewSupportService(
		cfg, readRepo, writeRepo, otpStore, jwtSigner, refreshStore, resetStore, emailClient, logger,
	)

	srv, err := grpcsrv.NewSupportServer(cfg, supportSvc, logger)
	if err != nil {
		return fmt.Errorf("grpc server: %w", err)
	}

	logger.Info("support-service ready", zap.String("port", cfg.Server.Port))
	return srv.Serve(ctx)
}
