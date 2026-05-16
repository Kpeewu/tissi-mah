package config

import (
	"fmt"

	sharedconfig "github.com/Kpeewu/tissi-mah/pkg/config"
	"github.com/spf13/viper"
)

type Config struct {
	Server          ServerConfig
	Environment     EnvironmentConfig
	Redis           RedisConfig
	SuspensionRedis RedisConfig
	Firebase       FirebaseConfig
	AuthService    ServiceEndpoint
	UserService    ServiceEndpoint
	RatingService  ServiceEndpoint
	FileService    ServiceEndpoint
	VehicleService ServiceEndpoint
	TripsService   ServiceEndpoint
	KYCService     ServiceEndpoint
	BookingService ServiceEndpoint
	PaymentService      ServiceEndpoint
	NotificationService ServiceEndpoint
	SupportService      ServiceEndpoint
	GeolocationService  ServiceEndpoint
	ChatService         ServiceEndpoint
	SupportJWTSecret    string
	CORS                CORSConfig
	RateLimit      RateLimitConfig
	AppID          AppIDConfig
	Security       SecurityConfig
	LogLevel       string
}

// AppIDConfig contient les whitelists CSV d'App-IDs par zone cliente.
// CSV pour rotation sans downtime : "ancien-id,nouveau-id" pendant la fenêtre.
type AppIDConfig struct {
	MobileAppIDs  string // MOBILE_APP_IDS — ex: "uuid-dev-1,uuid-dev-2"
	SupportAppIDs string // SUPPORT_APP_IDS
}

// SecurityConfig regroupe les toggles de sécurité par environnement.
type SecurityConfig struct {
	// EnableHSTS active le header Strict-Transport-Security.
	// À true uniquement si TLS est terminé en amont (staging, prod).
	EnableHSTS bool
	// BodySizeMaxBytes limite la taille du body HTTP (défaut 1 Mo).
	BodySizeMaxBytes int64
	// RateLimitFailClosed : si true, bloquer en cas de Redis down au lieu de laisser passer.
	// Activé en staging/prod pour garantir la protection même en incident.
	RateLimitFailClosed bool
	// InternalHMACSecret sert à signer/vérifier x-firebase-uid entre l'api-gateway
	// et les services internes (propagé via metadata gRPC).
	InternalHMACSecret string
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
		SuspensionRedis: RedisConfig{
			URL: sharedconfig.GetStringOrDefault(values, "SUSPENSION_REDIS_URL", ""),
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
		BookingService: ServiceEndpoint{
			Host: sharedconfig.GetStringOrDefault(values, "BOOKING_SERVICE_HOST", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "BOOKING_SERVICE_PORT", "50058"),
		},
		PaymentService: ServiceEndpoint{
			Host: sharedconfig.GetStringOrDefault(values, "PAYMENT_SERVICE_HOST", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "PAYMENT_SERVICE_PORT", "50059"),
		},
		NotificationService: ServiceEndpoint{
			Host: sharedconfig.GetStringOrDefault(values, "NOTIF_SERVICE_HOST", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "NOTIF_SERVICE_PORT", "50060"),
		},
		SupportService: ServiceEndpoint{
			Host: sharedconfig.GetStringOrDefault(values, "SUPPORT_SERVICE_HOST", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "SUPPORT_SERVICE_PORT", "50063"),
		},
		GeolocationService: ServiceEndpoint{
			Host: sharedconfig.GetStringOrDefault(values, "GEOLOCATION_SERVICE_HOST", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "GEOLOCATION_SERVICE_PORT", "50064"),
		},
		ChatService: ServiceEndpoint{
			Host: sharedconfig.GetStringOrDefault(values, "CHAT_SERVICE_HOST", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "CHAT_SERVICE_PORT", "50065"),
		},
		SupportJWTSecret: sharedconfig.GetStringOrDefault(values, "SUPPORT_JWT_SECRET", ""),
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
		AppID: AppIDConfig{
			MobileAppIDs:  sharedconfig.MustGetString(values, "MOBILE_APP_IDS"),
			SupportAppIDs: sharedconfig.MustGetString(values, "SUPPORT_APP_IDS"),
		},
		Security: SecurityConfig{
			EnableHSTS:          getBoolOrDefault(values, "ENABLE_HSTS", false),
			BodySizeMaxBytes:    getInt64OrDefault(values, "BODY_SIZE_MAX_BYTES", 10<<20),
			RateLimitFailClosed: getBoolOrDefault(values, "RATELIMIT_FAIL_CLOSED", false),
			InternalHMACSecret:  sharedconfig.GetStringOrDefault(values, "INTERNAL_HMAC_SECRET", ""),
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
	// Un SUPPORT_JWT_SECRET vide accepterait n'importe quel JWT HS256 en back-office.
	if cfg.Environment.Mode != "local" && cfg.SupportJWTSecret == "" {
		return fmt.Errorf("SUPPORT_JWT_SECRET is required in %s environment", cfg.Environment.Mode)
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

func getInt64OrDefault(v *viper.Viper, key string, defaultVal int64) int64 {
	if !v.IsSet(key) {
		return defaultVal
	}
	val := v.GetInt64(key)
	if val == 0 {
		return defaultVal
	}
	return val
}

func getBoolOrDefault(v *viper.Viper, key string, defaultVal bool) bool {
	if !v.IsSet(key) {
		return defaultVal
	}
	return v.GetBool(key)
}
