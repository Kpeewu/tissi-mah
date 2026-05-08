package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
	"github.com/jackc/pgx/v5/pgxpool"

	pkgDatabase "github.com/Kpeewu/tissi-mah/pkg/database"
	pkgLogger "github.com/Kpeewu/tissi-mah/pkg/logger"
	"github.com/Kpeewu/tissi-mah/services/chat-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/chat-service/internal/config"
	"github.com/Kpeewu/tissi-mah/services/chat-service/internal/crypto"
	chatGRPC "github.com/Kpeewu/tissi-mah/services/chat-service/internal/grpc"
	"github.com/Kpeewu/tissi-mah/services/chat-service/internal/repository/implementations"
	"github.com/Kpeewu/tissi-mah/services/chat-service/internal/service"
)

func main() {
	bootstrapLogger := pkgLogger.NewDefault("chat-service")
	defer bootstrapLogger.Sync() //nolint:errcheck

	if err := run(bootstrapLogger); err != nil {
		bootstrapLogger.Fatal("chat-service terminated", zap.Error(err))
	}
}

func run(bootstrapLogger *zap.Logger) error {
	// Config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	// Logger
	logger, err := pkgLogger.New(pkgLogger.Config{
		Level:       cfg.LogLevel,
		Environment: cfg.Environment.Mode,
		ServiceName: "chat-service",
	})
	if err != nil {
		return fmt.Errorf("logger: %w", err)
	}
	defer logger.Sync() //nolint:errcheck

	// Context
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Database
	poolConfig, err := pgxpool.ParseConfig(cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("postgres parse config: %w", err)
	}
	poolConfig.MaxConns = cfg.Database.MaxConns
	poolConfig.MinConns = cfg.Database.MinConns
	db, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	defer db.Close()
	if err = db.Ping(ctx); err != nil {
		return fmt.Errorf("postgres ping: %w", err)
	}

	// Redis local (TripCloserWorker subscribe trip.completed, locks)
	redisClient, err := pkgDatabase.NewRedisClientFromURL(ctx, cfg.Redis.URL)
	if err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	defer redisClient.Close()

	// Redis notification (publication NEW_MESSAGE → push FCM via
	// notification-service). Distinct du Redis local pour isoler la file
	// d'événements inter-services.
	notifRedis, err := pkgDatabase.NewRedisClientFromURL(ctx, cfg.NotificationRedis.URL)
	if err != nil {
		return fmt.Errorf("notification redis: %w", err)
	}
	defer notifRedis.Close()

	// Encrypteur
	encryptor, err := crypto.NewMessageEncryptor(cfg.Encryption.MasterKeyBase64)
	if err != nil {
		return fmt.Errorf("encryption: %w", err)
	}

	// Clients inter-services
	userClient, err := client.NewUserClient(cfg.UserService.Addr(), logger)
	if err != nil {
		return fmt.Errorf("user client: %w", err)
	}
	defer userClient.Close()

	bookingClient, err := client.NewBookingClient(cfg.BookingService.Addr(), logger)
	if err != nil {
		return fmt.Errorf("booking client: %w", err)
	}
	defer bookingClient.Close()

	tripClient, err := client.NewTripClient(cfg.TripsService.Addr(), logger)
	if err != nil {
		return fmt.Errorf("trip client: %w", err)
	}
	defer tripClient.Close()

	// Repositories
	threadRepo := implementations.NewChatThreadRepository(db, logger)
	messageRepo := implementations.NewChatMessageRepository(db, logger)

	// Service
	chatService := service.NewChatService(
		threadRepo, messageRepo,
		userClient, bookingClient, tripClient,
		encryptor, notifRedis, logger,
	)

	// Workers background
	go service.StartTripCloserWorker(ctx, chatService, redisClient, cfg.Worker.TripCloserIntervalSeconds, logger)
	go service.StartRetentionWorker(ctx, messageRepo, cfg.Worker.MessageRetentionDays, logger)

	// Serveur gRPC (TLS self-signed via grpcutil.NewServer)
	srv, err := chatGRPC.NewChatServer(cfg, chatService, encryptor, logger)
	if err != nil {
		return fmt.Errorf("grpc server: %w", err)
	}

	logger.Info("chat-service ready", zap.String("port", cfg.Server.Port))

	return srv.Serve(ctx)
}
