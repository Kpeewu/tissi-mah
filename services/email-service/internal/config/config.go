package config

import (
	"fmt"

	sharedconfig "github.com/Kpeewu/tissi-mah/pkg/config"
)

type Config struct {
	Server      ServerConfig
	Environment EnvironmentConfig
	SendGrid    SendGridConfig
	AWS         AWSConfig
	Email       EmailConfig
	LogLevel    string
}

type ServerConfig struct {
	Host string
	Port string
}

type EnvironmentConfig struct {
	Mode string
}

type SendGridConfig struct {
	APIKey string
}

type AWSConfig struct {
	Region string
}

type EmailConfig struct {
	From string
}

func Load() (*Config, error) {
	values, err := sharedconfig.Load("")
	if err != nil {
		return nil, err
	}

	config := &Config{
		Server: ServerConfig{
			Host: sharedconfig.GetStringOrDefault(values, "GRPC_ADDRESS", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "GRPC_PORT", "50061"),
		},
		Environment: EnvironmentConfig{
			Mode: sharedconfig.MustGetString(values, "ENVIRONMENT"),
		},
		SendGrid: SendGridConfig{
			APIKey: sharedconfig.GetStringOrDefault(values, "SENDGRID_API_KEY", ""),
		},
		AWS: AWSConfig{
			Region: sharedconfig.GetStringOrDefault(values, "AWS_REGION", "eu-west-1"),
		},
		Email: EmailConfig{
			From: sharedconfig.GetStringOrDefault(values, "EMAIL_FROM", "noreply@tissimah.com"),
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
	if cfg.Environment.Mode == "prod" && cfg.AWS.Region == "" {
		return fmt.Errorf("AWS_REGION is required in production")
	}
	if cfg.Environment.Mode != "prod" && cfg.SendGrid.APIKey == "" {
		return fmt.Errorf("SENDGRID_API_KEY is required in non-production environments")
	}
	return nil
}
