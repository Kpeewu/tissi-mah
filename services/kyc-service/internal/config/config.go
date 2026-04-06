package config

import (
	"fmt"

	sharedconfig "github.com/Kpeewu/tissi-mah/pkg/config"
)

type Config struct {
	Server            ServerConfig
	Environment       EnvironmentConfig
	FileService       FileServiceConfig
	Persona           PersonaConfig
	NotificationRedis RedisConfig
	LogLevel          string
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

type PersonaConfig struct {
	APIKey        string
	TemplateID    string
	WebhookSecret string
}

func Load() (*Config, error) {
	values, err := sharedconfig.Load("")
	if err != nil {
		return nil, err
	}

	config := &Config{
		Server: ServerConfig{
			Host: sharedconfig.GetStringOrDefault(values, "GRPC_ADDRESS", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "GRPC_PORT", "50055"),
		},
		Environment: EnvironmentConfig{
			Mode: sharedconfig.MustGetString(values, "ENVIRONMENT"),
		},
		FileService: FileServiceConfig{
			Address: sharedconfig.GetStringOrDefault(values, "FILE_SERVICE_HOST", "0.0.0.0"),
			Port:    sharedconfig.GetStringOrDefault(values, "FILE_SERVICE_PORT", "50053"),
		},
		Persona: PersonaConfig{
			APIKey:        sharedconfig.MustGetString(values, "PERSONA_API_KEY"),
			TemplateID:    sharedconfig.GetStringOrDefault(values, "PERSONA_TEMPLATE_ID", "itmpl_default"),
			WebhookSecret: sharedconfig.MustGetString(values, "PERSONA_WEBHOOK_SECRET"),
		},
		NotificationRedis: RedisConfig{
			URL: sharedconfig.MustGetString(values, "NOTIFICATION_REDIS_URL"),
		},
		LogLevel: sharedconfig.MustGetString(values, "LOG_LEVEL"),
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
	if cfg.Persona.APIKey == "" {
		return fmt.Errorf("PERSONA_API_KEY is required")
	}
	if cfg.Persona.WebhookSecret == "" {
		return fmt.Errorf("PERSONA_WEBHOOK_SECRET is required")
	}
	if cfg.NotificationRedis.URL == "" {
		return fmt.Errorf("NOTIFICATION_REDIS_URL is required")
	}
	return nil
}
