package config

import (
	"fmt"

	sharedconfig "github.com/Kpeewu/tissi-mah/pkg/config"
)

type Config struct {
	Server               ServerConfig
	Environment          EnvironmentConfig
	Database             DatabaseConfig
	Redis                RedisConfig
	S3                   S3Config
	UserService          UserServiceConfig
	VehicleService       ServiceConfig
	ModerationService    ServiceConfig
	ModerationEnabled    bool
	ModerationFailClosed bool
	LogLevel             string
}

type ServiceConfig struct {
	Address string
	Port    string
}

func (s ServiceConfig) Addr() string {
	return fmt.Sprintf("%s:%s", s.Address, s.Port)
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

type S3Config struct {
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
	Endpoint  string
	// PublicEndpoint est utilisé exclusivement pour construire les URL présignées
	// envoyées aux clients. Laissez vide si Endpoint est déjà le domaine public.
	// Ex: S3_ENDPOINT=http://minio:9000 (interne), S3_PUBLIC_ENDPOINT=https://storage.tissimah.kpeewu.dev
	PublicEndpoint string
	ForcePathStyle bool
}

type UserServiceConfig struct {
	Address string
	Port    string
}

func (c UserServiceConfig) Addr() string {
	return fmt.Sprintf("%s:%s", c.Address, c.Port)
}

func Load() (*Config, error) {

	values, err := sharedconfig.Load("")
	if err != nil {
		return nil, err
	}

	config := &Config{
		Server: ServerConfig{
			Host: sharedconfig.GetStringOrDefault(values, "GRPC_ADDRESS", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "GRPC_PORT", "50053"),
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
		S3: S3Config{
			Region:         sharedconfig.GetStringOrDefault(values, "S3_REGION", "us-east-1"),
			Bucket:         sharedconfig.MustGetString(values, "S3_BUCKET"),
			AccessKey:      sharedconfig.MustGetString(values, "S3_ACCESS_KEY"),
			SecretKey:      sharedconfig.MustGetString(values, "S3_SECRET_KEY"),
			Endpoint:       sharedconfig.GetStringOrDefault(values, "S3_ENDPOINT", ""),
			PublicEndpoint: sharedconfig.GetStringOrDefault(values, "S3_PUBLIC_ENDPOINT", ""),
			ForcePathStyle: sharedconfig.GetStringOrDefault(values, "S3_FORCE_PATH_STYLE", "true") == "true",
		},
		UserService: UserServiceConfig{
			Address: sharedconfig.GetStringOrDefault(values, "USER_SERVICE_HOST", "0.0.0.0"),
			Port:    sharedconfig.GetStringOrDefault(values, "USER_SERVICE_PORT", "50052"),
		},
		VehicleService: ServiceConfig{
			Address: sharedconfig.GetStringOrDefault(values, "VEHICLE_SERVICE_HOST", "0.0.0.0"),
			Port:    sharedconfig.GetStringOrDefault(values, "VEHICLE_SERVICE_PORT", "50055"),
		},
		ModerationService: ServiceConfig{
			Address: sharedconfig.GetStringOrDefault(values, "MODERATION_SERVICE_HOST", "0.0.0.0"),
			Port:    sharedconfig.GetStringOrDefault(values, "MODERATION_SERVICE_PORT", "50066"),
		},
		ModerationEnabled:    getBoolOrDefault(values, "MODERATION_ENABLED", true),
		ModerationFailClosed: getBoolOrDefault(values, "MODERATION_FAIL_CLOSED", false),
		LogLevel:             sharedconfig.MustGetString(values, "LOG_LEVEL"),
	}

	if err := validate(config); err != nil {
		return nil, err
	}

	return config, nil
}

func getBoolOrDefault(v interface {
	GetBool(string) bool
	IsSet(string) bool
}, key string, def bool) bool {
	if !v.IsSet(key) {
		return def
	}
	return v.GetBool(key)
}

func validate(cfg *Config) error {
	if cfg.Database.URL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.Redis.URL == "" {
		return fmt.Errorf("REDIS_URL is required")
	}
	if cfg.S3.Bucket == "" {
		return fmt.Errorf("S3_BUCKET is required")
	}
	if cfg.S3.AccessKey == "" {
		return fmt.Errorf("S3_ACCESS_KEY is required")
	}
	if cfg.S3.SecretKey == "" {
		return fmt.Errorf("S3_SECRET_KEY is required")
	}
	return nil
}
