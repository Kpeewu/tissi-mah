package config

import (
	"fmt"

	sharedconfig "github.com/Kpeewu/tissi-mah/pkg/config"
)

type Config struct {
	Server      ServerConfig
	Environment EnvironmentConfig
	Database    DatabaseConfig
	UserService ServiceConfig
	AuthService ServiceConfig
	LogLevel    string
	Moderation  ModerationConfig
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

type ServiceConfig struct {
	Address string
	Port    string
}

// ModerationConfig regroupe les seuils et clés d'API pour la modération.
type ModerationConfig struct {
	// Seuils de décision (texte)
	TextBlockedThreshold float64
	TextFlaggedThreshold float64
	// Clés API externes (optionnelles — graceful degradation si vides)
	PerspectiveAPIKey  string
	GoogleAppCreds     string // GOOGLE_APPLICATION_CREDENTIALS (JSON path ou contenu)
	// Comportement en cas d'indisponibilité du service externe
	FailClosed bool
}

func Load() (*Config, error) {
	values, err := sharedconfig.Load("")
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Server: ServerConfig{
			Host: sharedconfig.GetStringOrDefault(values, "GRPC_ADDRESS", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "GRPC_PORT", "50066"),
		},
		Environment: EnvironmentConfig{
			Mode: sharedconfig.MustGetString(values, "ENVIRONMENT"),
		},
		Database: DatabaseConfig{
			URL: sharedconfig.MustGetString(values, "DATABASE_URL"),
		},
		UserService: ServiceConfig{
			Address: sharedconfig.GetStringOrDefault(values, "USER_SERVICE_HOST", "0.0.0.0"),
			Port:    sharedconfig.GetStringOrDefault(values, "USER_SERVICE_PORT", "50052"),
		},
		AuthService: ServiceConfig{
			Address: sharedconfig.GetStringOrDefault(values, "AUTH_SERVICE_HOST", "0.0.0.0"),
			Port:    sharedconfig.GetStringOrDefault(values, "AUTH_SERVICE_PORT", "50051"),
		},
		LogLevel: sharedconfig.MustGetString(values, "LOG_LEVEL"),
		Moderation: ModerationConfig{
			TextBlockedThreshold: getFloat64OrDefault(values, "TEXT_BLOCKED_THRESHOLD", 0.90),
			TextFlaggedThreshold: getFloat64OrDefault(values, "TEXT_FLAGGED_THRESHOLD", 0.70),
			PerspectiveAPIKey:    sharedconfig.GetStringOrDefault(values, "PERSPECTIVE_API_KEY", ""),
			GoogleAppCreds:       sharedconfig.GetStringOrDefault(values, "GOOGLE_APPLICATION_CREDENTIALS", ""),
			FailClosed:           getBoolOrDefault(values, "MODERATION_FAIL_CLOSED", false),
		},
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
	return nil
}

func getFloat64OrDefault(values interface{ GetFloat64(string) float64; IsSet(string) bool }, key string, def float64) float64 {
	if !values.IsSet(key) {
		return def
	}
	v := values.GetFloat64(key)
	if v == 0 {
		return def
	}
	return v
}

func getBoolOrDefault(values interface{ GetBool(string) bool; IsSet(string) bool }, key string, def bool) bool {
	if !values.IsSet(key) {
		return def
	}
	return values.GetBool(key)
}
