package config

import (
	"fmt"

	sharedconfig "github.com/Kpeewu/tissi-mah/pkg/config"
)

type Config struct {
	Server             ServerConfig
	Environment        EnvironmentConfig
	MongoDB            MongoDBConfig
	Redis              RedisConfig
	AuthService        AuthServiceConfig
	FileService        FileServiceConfig
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

type MongoDBConfig struct {
	URL      string
	Database string
}

type RedisConfig struct {
	URL string
}

type AuthServiceConfig struct {
	Address string
	Port    string
}

type FileServiceConfig struct {
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
			Port: sharedconfig.GetStringOrDefault(values, "GRPC_PORT", "50052"),
		},
		Environment: EnvironmentConfig{
			Mode: sharedconfig.MustGetString(values, "ENVIRONMENT"),
		},
		MongoDB: MongoDBConfig{
			URL:      sharedconfig.MustGetString(values, "MONGODB_URL"),
			Database: sharedconfig.GetStringOrDefault(values, "MONGODB_DATABASE", "user_db"),
		},
		Redis: RedisConfig{
			URL: sharedconfig.MustGetString(values, "REDIS_URL"),
		},
		AuthService: AuthServiceConfig{
			Address: sharedconfig.GetStringOrDefault(values, "AUTH_SERVICE_HOST", "0.0.0.0"),
			Port:    sharedconfig.GetStringOrDefault(values, "AUTH_SERVICE_PORT", "50051"),
		},
		FileService: FileServiceConfig{
			Address: sharedconfig.GetStringOrDefault(values, "FILE_SERVICE_HOST", "0.0.0.0"),
			Port:    sharedconfig.GetStringOrDefault(values, "FILE_SERVICE_PORT", "50053"),
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
	if cfg.MongoDB.URL == "" {
		return fmt.Errorf("MONGODB_URL is required")
	}
	if cfg.Redis.URL == "" {
		return fmt.Errorf("REDIS_URL is required")
	}
	if cfg.Server.Port == "" {
		return fmt.Errorf("GRPC_PORT is required")
	}
	return nil
}
