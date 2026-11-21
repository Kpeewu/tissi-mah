package config

import (
	"fmt"

	sharedconfig "github.com/Kpeewu/tissi-mah/pkg/config"
)

type Config struct {
	Server            ServerConfig
	Environment       EnvironmentConfig
	Database          DatabaseConfig
	Redis             RedisConfig
	NotificationRedis RedisConfig
	UserService       ServiceEndpoint
	BookingService    ServiceEndpoint
	TripsService      ServiceEndpoint
	SupportService      ServiceEndpoint
	ModerationService   ServiceEndpoint
	ModerationEnabled   bool
	ModerationFailClosed bool
	Encryption         EncryptionConfig
	Worker             WorkerConfig
	LogLevel           string
	InternalHMACSecret string
}

type ServerConfig struct {
	Host string
	Port string
}

type EnvironmentConfig struct {
	Mode string
}

type DatabaseConfig struct {
	URL      string
	MaxConns int32
	MinConns int32
}

type RedisConfig struct {
	URL string
}

type ServiceEndpoint struct {
	Host string
	Port string
}

func (s ServiceEndpoint) Addr() string {
	return fmt.Sprintf("%s:%s", s.Host, s.Port)
}

type EncryptionConfig struct {
	MasterKeyBase64 string // CHAT_ENCRYPTION_KEY : base64(32 random bytes)
}

type WorkerConfig struct {
	// Intervalle (en secondes) du worker qui subscribe Redis Streams pour
	// les events trip.completed et ferme les threads correspondants.
	TripCloserIntervalSeconds int
	// Durée de rétention des messages avant purge (jours).
	MessageRetentionDays int
}

func Load() (*Config, error) {
	values, err := sharedconfig.Load("")
	if err != nil {
		return nil, err
	}

	return &Config{
		Server: ServerConfig{
			Host: sharedconfig.GetStringOrDefault(values, "GRPC_ADDRESS", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "GRPC_PORT", "50065"),
		},
		Environment: EnvironmentConfig{
			Mode: sharedconfig.MustGetString(values, "ENVIRONMENT"),
		},
		Database: DatabaseConfig{
			URL:      sharedconfig.MustGetString(values, "DATABASE_URL"),
			MaxConns: int32(getIntOrDefault(values, "DB_MAX_CONNS", 10)),
			MinConns: int32(getIntOrDefault(values, "DB_MIN_CONNS", 2)),
		},
		Redis: RedisConfig{
			URL: sharedconfig.MustGetString(values, "REDIS_URL"),
		},
		// NotificationRedis = bus inter-services pour les events
		// (NEW_MESSAGE consommé par notification-service → push FCM).
		// Distinct du Redis local qui sert pour les locks et le TripCloserWorker.
		NotificationRedis: RedisConfig{
			URL: sharedconfig.MustGetString(values, "NOTIFICATION_REDIS_URL"),
		},
		UserService: ServiceEndpoint{
			Host: sharedconfig.GetStringOrDefault(values, "USER_SERVICE_HOST", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "USER_SERVICE_PORT", "50052"),
		},
		BookingService: ServiceEndpoint{
			Host: sharedconfig.GetStringOrDefault(values, "BOOKING_SERVICE_HOST", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "BOOKING_SERVICE_PORT", "50058"),
		},
		TripsService: ServiceEndpoint{
			Host: sharedconfig.GetStringOrDefault(values, "TRIPS_SERVICE_HOST", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "TRIPS_SERVICE_PORT", "50055"),
		},
		SupportService: ServiceEndpoint{
			Host: sharedconfig.GetStringOrDefault(values, "SUPPORT_SERVICE_HOST", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "SUPPORT_SERVICE_PORT", "50063"),
		},
		ModerationService: ServiceEndpoint{
			Host: sharedconfig.GetStringOrDefault(values, "MODERATION_SERVICE_HOST", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "MODERATION_SERVICE_PORT", "50066"),
		},
		ModerationEnabled:    getBoolOrDefault(values, "MODERATION_ENABLED", true),
		ModerationFailClosed: getBoolOrDefault(values, "MODERATION_FAIL_CLOSED", false),
		Encryption: EncryptionConfig{
			MasterKeyBase64: sharedconfig.MustGetString(values, "CHAT_ENCRYPTION_KEY"),
		},
		Worker: WorkerConfig{
			TripCloserIntervalSeconds: getIntOrDefault(values, "TRIP_CLOSER_INTERVAL_SECONDS", 30),
			MessageRetentionDays:      getIntOrDefault(values, "MESSAGE_RETENTION_DAYS", 180),
		},
		LogLevel:           sharedconfig.MustGetString(values, "LOG_LEVEL"),
		InternalHMACSecret: sharedconfig.GetStringOrDefault(values, "INTERNAL_HMAC_SECRET", ""),
	}, nil
}

func getBoolOrDefault(v interface{ GetBool(string) bool; IsSet(string) bool }, key string, def bool) bool {
	if !v.IsSet(key) {
		return def
	}
	return v.GetBool(key)
}

func getIntOrDefault(v interface{ GetInt(string) int; IsSet(string) bool }, key string, def int) int {
	if !v.IsSet(key) {
		return def
	}
	if val := v.GetInt(key); val != 0 {
		return val
	}
	return def
}
