package config

import (
	"fmt"

	sharedconfig "github.com/Kpeewu/tissi-mah/pkg/config"
	"github.com/spf13/viper"
)

// Config contient toute la configuration du booking-service.
type Config struct {
	Server         ServerConfig
	Environment    EnvironmentConfig
	Database       DatabaseConfig
	Redis          RedisConfig
	TripService    ServiceEndpoint
	UserService    ServiceEndpoint
	PaymentService ServiceEndpoint
	ServiceFee     ServiceFeeConfig
	Reconciliation ReconciliationConfig
	Payment        PaymentConfig
	LogLevel       string
}

// ServerConfig contient la configuration du serveur gRPC.
type ServerConfig struct {
	Host string
	Port string
}

// EnvironmentConfig contient l'environnement d'exécution.
type EnvironmentConfig struct {
	Mode string
}

// DatabaseConfig contient l'URL de connexion PostgreSQL.
type DatabaseConfig struct {
	URL string
}

// RedisConfig contient l'URL de connexion Redis.
type RedisConfig struct {
	URL string
}

// ServiceEndpoint contient l'adresse d'un service distant.
type ServiceEndpoint struct {
	Host string
	Port string
}

// Addr retourne l'adresse host:port du service.
func (s ServiceEndpoint) Addr() string {
	return fmt.Sprintf("%s:%s", s.Host, s.Port)
}

// ServiceFeeConfig contient le pourcentage de frais de service.
type ServiceFeeConfig struct {
	Percent int
}

// ReconciliationConfig contient l'intervalle de réconciliation Redis/DB.
type ReconciliationConfig struct {
	IntervalSeconds int
}

// PaymentConfig contient la configuration liée au paiement.
type PaymentConfig struct {
	ContestationDelaySeconds     int
	ReleaseWorkerIntervalSeconds int
}

// Load charge la configuration depuis les variables d'environnement.
func Load() (*Config, error) {
	values, err := sharedconfig.Load("")
	if err != nil {
		return nil, err
	}

	config := &Config{
		Server: ServerConfig{
			Host: sharedconfig.GetStringOrDefault(values, "GRPC_ADDRESS", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "GRPC_PORT", "50058"),
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
		TripService: ServiceEndpoint{
			Host: sharedconfig.GetStringOrDefault(values, "TRIPS_SERVICE_HOST", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "TRIPS_SERVICE_PORT", "50056"),
		},
		UserService: ServiceEndpoint{
			Host: sharedconfig.GetStringOrDefault(values, "USER_SERVICE_HOST", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "USER_SERVICE_PORT", "50052"),
		},
		ServiceFee: ServiceFeeConfig{
			Percent: getIntOrDefault(values, "SERVICE_FEE_PERCENT", 10),
		},
		PaymentService: ServiceEndpoint{
			Host: sharedconfig.GetStringOrDefault(values, "PAYMENT_SERVICE_HOST", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "PAYMENT_SERVICE_PORT", "50059"),
		},
		Reconciliation: ReconciliationConfig{
			IntervalSeconds: getIntOrDefault(values, "RECONCILIATION_INTERVAL_SECONDS", 300),
		},
		Payment: PaymentConfig{
			ContestationDelaySeconds:     getIntOrDefault(values, "CONTESTATION_DELAY_SECONDS", 7200),
			ReleaseWorkerIntervalSeconds: getIntOrDefault(values, "RELEASE_WORKER_INTERVAL_SECONDS", 60),
		},
		LogLevel: sharedconfig.MustGetString(values, "LOG_LEVEL"),
	}

	return config, nil
}

// getIntOrDefault lit un entier depuis la config Viper ou retourne la valeur par défaut.
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
