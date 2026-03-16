package config

import (
	"fmt"

	sharedconfig "github.com/Kpeewu/tissi-mah/pkg/config"
	"github.com/spf13/viper"
)

type Config struct {
	Server      ServerConfig
	Environment EnvironmentConfig
	Redis       RedisConfig
	Firebase    FirebaseConfig
	AuthService    ServiceEndpoint
	UserService    ServiceEndpoint
	RatingService  ServiceEndpoint
	FileService    ServiceEndpoint
	VehicleService ServiceEndpoint
	TripsService   ServiceEndpoint
	KYCService     ServiceEndpoint
	CORS          CORSConfig
	RateLimit   RateLimitConfig
	LogLevel    string
}

type ServerConfig struct {
	Host string
	Port string
}

type EnvironmentConfig struct {
	Mode string
}

type RedisConfig struct {
	URL string
}

type FirebaseConfig struct {
	ProjectID string
}

type ServiceEndpoint struct {
	Host string
	Port string
}

// Address retourne l'adresse host:port du service
func (s ServiceEndpoint) Address() string {
	return fmt.Sprintf("%s:%s", s.Host, s.Port)
}

type CORSConfig struct {
	AllowedOrigins string
}

// RateLimitConfig contient les limites par tier (requests par fenêtre)
type RateLimitConfig struct {
	GlobalMinute        int
	GlobalHour          int
	AuthSecond          int
	AuthMinute          int
	AuthHour            int
	CreateAccountMinute int
	CreateAccountHour   int
	SensitiveMinute     int
	SensitiveHour       int
}

func Load() (*Config, error) {
	values, err := sharedconfig.Load("")
	if err != nil {
		return nil, err
	}

	config := &Config{
		Server: ServerConfig{
			Host: sharedconfig.GetStringOrDefault(values, "HTTP_ADDRESS", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "HTTP_PORT", "8080"),
		},
		Environment: EnvironmentConfig{
			Mode: sharedconfig.MustGetString(values, "ENVIRONMENT"),
		},
		Redis: RedisConfig{
			URL: sharedconfig.MustGetString(values, "REDIS_URL"),
		},
		Firebase: FirebaseConfig{
			ProjectID: sharedconfig.MustGetString(values, "FIREBASE_PROJECT_ID"),
		},
		AuthService: ServiceEndpoint{
			Host: sharedconfig.GetStringOrDefault(values, "AUTH_SERVICE_HOST", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "AUTH_SERVICE_PORT", "50051"),
		},
		UserService: ServiceEndpoint{
			Host: sharedconfig.GetStringOrDefault(values, "USER_SERVICE_HOST", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "USER_SERVICE_PORT", "50052"),
		},
		RatingService: ServiceEndpoint{
			Host: sharedconfig.GetStringOrDefault(values, "RATING_SERVICE_HOST", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "RATING_SERVICE_PORT", "50054"),
		},
		FileService: ServiceEndpoint{
			Host: sharedconfig.GetStringOrDefault(values, "FILE_SERVICE_HOST", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "FILE_SERVICE_PORT", "50053"),
		},
		VehicleService: ServiceEndpoint{
			Host: sharedconfig.GetStringOrDefault(values, "VEHICLE_SERVICE_HOST", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "VEHICLE_SERVICE_PORT", "50055"),
		},
		TripsService: ServiceEndpoint{
			Host: sharedconfig.GetStringOrDefault(values, "TRIPS_SERVICE_HOST", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "TRIPS_SERVICE_PORT", "50056"),
		},
		KYCService: ServiceEndpoint{
			Host: sharedconfig.GetStringOrDefault(values, "KYC_SERVICE_HOST", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "KYC_SERVICE_PORT", "50057"),
		},
		CORS: CORSConfig{
			AllowedOrigins: sharedconfig.GetStringOrDefault(values, "CORS_ALLOWED_ORIGINS", "*"),
		},
		RateLimit: RateLimitConfig{
			GlobalMinute:        getIntOrDefault(values, "RATE_LIMIT_GLOBAL_MINUTE", 60),
			GlobalHour:          getIntOrDefault(values, "RATE_LIMIT_GLOBAL_HOUR", 1000),
			AuthSecond:          getIntOrDefault(values, "RATE_LIMIT_AUTH_SECOND", 1),
			AuthMinute:          getIntOrDefault(values, "RATE_LIMIT_AUTH_MINUTE", 10),
			AuthHour:            getIntOrDefault(values, "RATE_LIMIT_AUTH_HOUR", 100),
			CreateAccountMinute: getIntOrDefault(values, "RATE_LIMIT_CREATE_MINUTE", 3),
			CreateAccountHour:   getIntOrDefault(values, "RATE_LIMIT_CREATE_HOUR", 10),
			SensitiveMinute:     getIntOrDefault(values, "RATE_LIMIT_SENSITIVE_MINUTE", 1),
			SensitiveHour:       getIntOrDefault(values, "RATE_LIMIT_SENSITIVE_HOUR", 5),
		},
		LogLevel: sharedconfig.MustGetString(values, "LOG_LEVEL"),
	}

	if err := validate(config); err != nil {
		return nil, err
	}

	return config, nil
}

func validate(cfg *Config) error {
	if cfg.Redis.URL == "" {
		return fmt.Errorf("REDIS_URL is required")
	}
	if cfg.Firebase.ProjectID == "" {
		return fmt.Errorf("FIREBASE_PROJECT_ID is required")
	}
	if cfg.Server.Port == "" {
		return fmt.Errorf("HTTP_PORT is required")
	}
	return nil
}

// getIntOrDefault lit un entier depuis la config Viper ou retourne la valeur par défaut
func getIntOrDefault(v *viper.Viper, key string, defaultVal int) int {
	if !v.IsSet(key) {
		return defaultVal
	}
	val := v.GetInt(key)
	if val == 0 {
		return defaultVal
	}
	return val
}
