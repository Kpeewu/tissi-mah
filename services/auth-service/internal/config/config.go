package config

import (
	"fmt"

	sharedconfig "github.com/Kpeewu/tissi-mah/pkg/config"
)

type Config struct {
	Server             ServerConfig
	Environment        EnvironmentConfig
	Database           DatabaseConfig
	Redis              RedisConfig
	NotificationRedis  RedisConfig
	UserService        UserServiceConfig
	TripsService       ServiceConfig
	BookingService     ServiceConfig
	PaymentService     ServiceConfig
	ChatService        ServiceConfig
	FileService        ServiceConfig
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
	URL string
}

type RedisConfig struct {
	URL string
}

type UserServiceConfig struct {
	Address string
	Port    string
}

type ServiceConfig struct {
	Address string
	Port    string
}

func Load() (*Config, error) {

	values, err := sharedconfig.Load("")
	if err != nil {
		return nil, err
	}

	config := &Config{
		Server: ServerConfig{
			Host: sharedconfig.GetStringOrDefault(values, "GRPC_ADDRESS", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "GRPC_PORT", "50051"),
		},
		Environment: EnvironmentConfig{
			Mode: sharedconfig.MustGetString(values, "ENVIRONMENT"),
		},
		Database: DatabaseConfig{
			URL: sharedconfig.MustGetString(values, "DATABASE_URL"),
		},
		Redis: RedisConfig{
			URL: sharedconfig.MustGetString(values, "REDIS_URL"),
		},
		NotificationRedis: RedisConfig{
			URL: sharedconfig.MustGetString(values, "NOTIFICATION_REDIS_URL"),
		},
		UserService: UserServiceConfig{
			Address: sharedconfig.GetStringOrDefault(values, "USER_SERVICE_HOST", "0.0.0.0"),
			Port:    sharedconfig.GetStringOrDefault(values, "USER_SERVICE_PORT", "50052"),
		},
		TripsService: ServiceConfig{
			Address: sharedconfig.GetStringOrDefault(values, "TRIPS_SERVICE_HOST", "0.0.0.0"),
			Port:    sharedconfig.GetStringOrDefault(values, "TRIPS_SERVICE_PORT", "50053"),
		},
		BookingService: ServiceConfig{
			Address: sharedconfig.GetStringOrDefault(values, "BOOKING_SERVICE_HOST", "0.0.0.0"),
			Port:    sharedconfig.GetStringOrDefault(values, "BOOKING_SERVICE_PORT", "50055"),
		},
		PaymentService: ServiceConfig{
			Address: sharedconfig.GetStringOrDefault(values, "PAYMENT_SERVICE_HOST", "0.0.0.0"),
			Port:    sharedconfig.GetStringOrDefault(values, "PAYMENT_SERVICE_PORT", "50056"),
		},
		ChatService: ServiceConfig{
			Address: sharedconfig.GetStringOrDefault(values, "CHAT_SERVICE_HOST", "0.0.0.0"),
			Port:    sharedconfig.GetStringOrDefault(values, "CHAT_SERVICE_PORT", "50057"),
		},
		FileService: ServiceConfig{
			Address: sharedconfig.GetStringOrDefault(values, "FILE_SERVICE_HOST", "0.0.0.0"),
			Port:    sharedconfig.GetStringOrDefault(values, "FILE_SERVICE_PORT", "50058"),
		},
		LogLevel:           sharedconfig.MustGetString(values, "LOG_LEVEL"),
		InternalHMACSecret: sharedconfig.GetStringOrDefault(values, "INTERNAL_HMAC_SECRET", ""),
	}

	if err := validate(config); err != nil {
		return nil, err
	}

	return config, nil
}

func validate(cfg *Config) error {
	if cfg.Database.URL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.Redis.URL == "" {
		return fmt.Errorf("REDIS_URL is required")
	}
	if cfg.NotificationRedis.URL == "" {
		return fmt.Errorf("NOTIFICATION_REDIS_URL is required")
	}
	if cfg.Server.Port == "" {
		return fmt.Errorf("GRPC_PORT is required")
	}
	return nil
}
