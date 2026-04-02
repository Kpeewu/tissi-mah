package config

import (
	"fmt"

	sharedconfig "github.com/Kpeewu/tissi-mah/pkg/config"
)

type Config struct {
	Server      ServerConfig
	Environment EnvironmentConfig
	Firebase    FirebaseConfig
	LogLevel    string
}

type ServerConfig struct {
	Host string
	Port string
}

type EnvironmentConfig struct {
	Mode string
}

type FirebaseConfig struct {
	CredentialsPath string
}

func Load() (*Config, error) {
	values, err := sharedconfig.Load("")
	if err != nil {
		return nil, err
	}

	config := &Config{
		Server: ServerConfig{
			Host: sharedconfig.GetStringOrDefault(values, "GRPC_ADDRESS", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "GRPC_PORT", "50062"),
		},
		Environment: EnvironmentConfig{
			Mode: sharedconfig.MustGetString(values, "ENVIRONMENT"),
		},
		Firebase: FirebaseConfig{
			CredentialsPath: sharedconfig.MustGetString(values, "FIREBASE_CREDENTIALS_PATH"),
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
	if cfg.Firebase.CredentialsPath == "" {
		return fmt.Errorf("FIREBASE_CREDENTIALS_PATH is required")
	}
	return nil
}
