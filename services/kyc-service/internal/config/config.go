package config

import (
	"fmt"

	sharedconfig "github.com/Kpeewu/tissi-mah/pkg/config"
)

type Config struct {
	Server             ServerConfig
	Environment        EnvironmentConfig
	FileService        FileServiceConfig
	UserService        UserServiceConfig
	SupportService     SupportServiceConfig
	VehicleService     VehicleServiceConfig
	NotificationRedis  RedisConfig
	LogLevel           string
	InternalHMACSecret string
}

type RedisConfig struct {
	URL string
}

type ServerConfig struct {
	Host string
	Port string
}

type EnvironmentConfig struct {
	Mode string
}

type FileServiceConfig struct {
	Address string
	Port    string
}

type UserServiceConfig struct {
	Address string
	Port    string
}

func (c UserServiceConfig) Addr() string {
	return fmt.Sprintf("%s:%s", c.Address, c.Port)
}

type SupportServiceConfig struct {
	Address string
	Port    string
}

func (c SupportServiceConfig) Addr() string {
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

	config := &Config{
		Server: ServerConfig{
			Host: sharedconfig.GetStringOrDefault(values, "GRPC_ADDRESS", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "GRPC_PORT", "50057"),
		},
		Environment: EnvironmentConfig{
			Mode: sharedconfig.MustGetString(values, "ENVIRONMENT"),
		},
		FileService: FileServiceConfig{
			Address: sharedconfig.GetStringOrDefault(values, "FILE_SERVICE_HOST", "0.0.0.0"),
			Port:    sharedconfig.GetStringOrDefault(values, "FILE_SERVICE_PORT", "50053"),
		},
		UserService: UserServiceConfig{
			Address: sharedconfig.GetStringOrDefault(values, "USER_SERVICE_HOST", "0.0.0.0"),
			Port:    sharedconfig.GetStringOrDefault(values, "USER_SERVICE_PORT", "50052"),
		},
		SupportService: SupportServiceConfig{
			Address: sharedconfig.GetStringOrDefault(values, "SUPPORT_SERVICE_HOST", "0.0.0.0"),
			Port:    sharedconfig.GetStringOrDefault(values, "SUPPORT_SERVICE_PORT", "50063"),
		},
		VehicleService: VehicleServiceConfig{
			Address: sharedconfig.GetStringOrDefault(values, "VEHICLE_SERVICE_HOST", "0.0.0.0"),
			Port:    sharedconfig.GetStringOrDefault(values, "VEHICLE_SERVICE_PORT", "50055"),
		},
		NotificationRedis: RedisConfig{
			URL: sharedconfig.MustGetString(values, "NOTIFICATION_REDIS_URL"),
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
	if cfg.Server.Port == "" {
		return fmt.Errorf("GRPC_PORT is required")
	}
	if cfg.NotificationRedis.URL == "" {
		return fmt.Errorf("NOTIFICATION_REDIS_URL is required")
	}
	return nil
}
