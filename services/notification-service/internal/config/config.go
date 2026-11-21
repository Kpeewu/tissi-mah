package config

import (
	"fmt"

	sharedconfig "github.com/Kpeewu/tissi-mah/pkg/config"
)

type Config struct {
	Server       ServerConfig
	Environment  EnvironmentConfig
	Database     DatabaseConfig
	Redis        RedisConfig
	UserService  ServiceEndpoint
	PushService        ServiceEndpoint
	EmailService       ServiceEndpoint
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

type ServiceEndpoint struct {
	Host string
	Port string
}

func (s ServiceEndpoint) Address() string {
	return fmt.Sprintf("%s:%s", s.Host, s.Port)
}

func Load() (*Config, error) {
	values, err := sharedconfig.Load("")
	if err != nil {
		return nil, err
	}

	config := &Config{
		Server: ServerConfig{
			Host: sharedconfig.GetStringOrDefault(values, "GRPC_ADDRESS", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "GRPC_PORT", "50060"),
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
		UserService: ServiceEndpoint{
			Host: sharedconfig.GetStringOrDefault(values, "USER_SERVICE_HOST", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "USER_SERVICE_PORT", "50052"),
		},
		PushService: ServiceEndpoint{
			Host: sharedconfig.GetStringOrDefault(values, "PUSH_SERVICE_HOST", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "PUSH_SERVICE_PORT", "50062"),
		},
		EmailService: ServiceEndpoint{
			Host: sharedconfig.GetStringOrDefault(values, "EMAIL_SERVICE_HOST", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "EMAIL_SERVICE_PORT", "50061"),
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
	return nil
}
