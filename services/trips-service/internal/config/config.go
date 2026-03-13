package config

import (
	"fmt"

	sharedconfig "github.com/Kpeewu/tissi-mah/pkg/config"
)

type Config struct {
	Server         ServerConfig
	Environment    EnvironmentConfig
	Database       DatabaseConfig
	Redis          RedisConfig
	UserService    UserServiceConfig
	VehicleService VehicleServiceConfig
	LogLevel       string
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

func (c UserServiceConfig) Addr() string {
	return fmt.Sprintf("%s:%s", c.Address, c.Port)
}

type VehicleServiceConfig struct {
	Address string
	Port    string
}

func (c VehicleServiceConfig) Addr() string {
	return fmt.Sprintf("%s:%s", c.Address, c.Port)
}

func Load() (*Config, error) {
	values, err := sharedconfig.Load("")
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Server: ServerConfig{
			Host: sharedconfig.GetStringOrDefault(values, "GRPC_ADDRESS", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "GRPC_PORT", "50056"),
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
		UserService: UserServiceConfig{
			Address: sharedconfig.GetStringOrDefault(values, "USER_SERVICE_HOST", "0.0.0.0"),
			Port:    sharedconfig.GetStringOrDefault(values, "USER_SERVICE_PORT", "50052"),
		},
		VehicleService: VehicleServiceConfig{
			Address: sharedconfig.GetStringOrDefault(values, "VEHICLE_SERVICE_HOST", "0.0.0.0"),
			Port:    sharedconfig.GetStringOrDefault(values, "VEHICLE_SERVICE_PORT", "50055"),
		},
		LogLevel: sharedconfig.MustGetString(values, "LOG_LEVEL"),
	}

	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func validate(cfg *Config) error {
	if cfg.Database.URL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.Redis.URL == "" {
		return fmt.Errorf("REDIS_URL is required")
	}
	if cfg.Server.Port == "" {
		return fmt.Errorf("GRPC_PORT is required")
	}
	return nil
}
