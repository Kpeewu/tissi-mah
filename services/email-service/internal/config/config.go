package config

import (
	"fmt"

	sharedconfig "github.com/Kpeewu/tissi-mah/pkg/config"
)

type Config struct {
	Server      ServerConfig
	Environment EnvironmentConfig
	SMTP        SMTPConfig
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

type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
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
		SMTP: SMTPConfig{
			Host:     sharedconfig.GetStringOrDefault(values, "SMTP_HOST", "ssl0.ovh.net"),
			Port:     sharedconfig.GetStringOrDefault(values, "SMTP_PORT", "465"),
			Username: sharedconfig.GetStringOrDefault(values, "SMTP_USERNAME", ""),
			Password: sharedconfig.GetStringOrDefault(values, "SMTP_PASSWORD", ""),
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
	if cfg.SMTP.Username == "" {
		return fmt.Errorf("SMTP_USERNAME is required")
	}
	if cfg.SMTP.Password == "" {
		return fmt.Errorf("SMTP_PASSWORD is required")
	}
	return nil
}
