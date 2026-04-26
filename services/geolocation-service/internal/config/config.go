package config

import (
	"fmt"

	sharedconfig "github.com/Kpeewu/tissi-mah/pkg/config"
)

// Config rassemble la configuration runtime du geolocation-service.
type Config struct {
	Server      ServerConfig
	Environment EnvironmentConfig
	OSRM        OSRMConfig
	Nominatim   NominatimConfig
	Redis       RedisConfig
	Geocode     GeocodeConfig
	LogLevel    string
}

type ServerConfig struct {
	Host string
	Port string
}

type EnvironmentConfig struct {
	Mode string
}

type OSRMConfig struct {
	URL string // ex: http://osrm-backend:5000
}

type NominatimConfig struct {
	URL string // ex: http://nominatim-backend:7070 en K8s (port Service), http://localhost:7070 en local
}

type RedisConfig struct {
	URL string
}

type GeocodeConfig struct {
	// DefaultCountries : ISO codes séparés par virgules (ex: "tg,gh,bj,bf").
	// Utilisé comme filtre par défaut dans Geocode.
	DefaultCountries string
}

func Load() (*Config, error) {
	values, err := sharedconfig.Load("")
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Server: ServerConfig{
			Host: sharedconfig.GetStringOrDefault(values, "GRPC_ADDRESS", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "GRPC_PORT", "50064"),
		},
		Environment: EnvironmentConfig{
			Mode: sharedconfig.MustGetString(values, "ENVIRONMENT"),
		},
		OSRM: OSRMConfig{
			URL: sharedconfig.MustGetString(values, "OSRM_URL"),
		},
		Nominatim: NominatimConfig{
			URL: sharedconfig.GetStringOrDefault(values, "NOMINATIM_URL", ""),
		},
		Redis: RedisConfig{
			URL: sharedconfig.MustGetString(values, "REDIS_URL"),
		},
		Geocode: GeocodeConfig{
			DefaultCountries: sharedconfig.GetStringOrDefault(values, "GEOCODE_DEFAULT_COUNTRIES", "tg,gh,bj,bf"),
		},
		LogLevel: sharedconfig.MustGetString(values, "LOG_LEVEL"),
	}

	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func validate(cfg *Config) error {
	if cfg.OSRM.URL == "" {
		return fmt.Errorf("OSRM_URL is required")
	}
	if cfg.Redis.URL == "" {
		return fmt.Errorf("REDIS_URL is required")
	}
	if cfg.Server.Port == "" {
		return fmt.Errorf("GRPC_PORT is required")
	}
	return nil
}
