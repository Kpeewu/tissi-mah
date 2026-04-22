package config

import (
	"fmt"

	sharedconfig "github.com/Kpeewu/tissi-mah/pkg/config"
	"github.com/spf13/viper"
)

type Config struct {
	Server            ServerConfig
	Environment       EnvironmentConfig
	Database          DatabaseConfig
	Redis             RedisConfig
	NotificationRedis RedisConfig
	FedaPay           FedaPayConfig
	BookingService    ServiceEndpoint
	UserService       ServiceEndpoint
	SupportService    ServiceEndpoint
	Payout            PayoutConfig
	Refund            RefundConfig
	RefundWorker      RefundWorkerConfig
	Expiration        ExpirationConfig
	Worker            WorkerConfig
	LogLevel          string
}

type ServerConfig struct {
	Host string
	Port string
}

type EnvironmentConfig struct {
	Mode string
}

type DatabaseConfig struct {
	URL      string
	MaxConns int32
	MinConns int32
}

type RedisConfig struct {
	URL string
}

type FedaPayConfig struct {
	APIURL               string
	APIKey               string
	WebhookSecret        string
	MaxConcurrentRequests int
}

type WorkerConfig struct {
	WebhookMaxConcurrent int
	PayoutMaxConcurrent  int
	RefundMaxConcurrent  int
}

type RefundWorkerConfig struct {
	IntervalSeconds int
	MaxRetries      int
	BatchSize       int
}

type ServiceEndpoint struct {
	Host string
	Port string
}

func (s ServiceEndpoint) Addr() string {
	return fmt.Sprintf("%s:%s", s.Host, s.Port)
}

type PayoutConfig struct {
	IntervalSeconds        int
	ContestationDelayHours int
	PlatformFeePercent     int
}

type ExpirationConfig struct {
	IntervalSeconds       int
	PaymentTimeoutMinutes int
}

type RefundConfig struct {
	CancellationFullRefundHours    int
	CancellationGracePeriodMinutes int
	NoShowDriverDelayMinutes       int
	NoShowPassengerDelayMinutes    int
}

func Load() (*Config, error) {
	values, err := sharedconfig.Load("")
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Server: ServerConfig{
			Host: sharedconfig.GetStringOrDefault(values, "GRPC_ADDRESS", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "GRPC_PORT", "50059"),
		},
		Environment: EnvironmentConfig{
			Mode: sharedconfig.MustGetString(values, "ENVIRONMENT"),
		},
		Database: DatabaseConfig{
			URL:      sharedconfig.MustGetString(values, "DATABASE_URL"),
			MaxConns: int32(getIntOrDefault(values, "DB_MAX_CONNS", 20)),
			MinConns: int32(getIntOrDefault(values, "DB_MIN_CONNS", 5)),
		},
		Redis: RedisConfig{
			URL: sharedconfig.MustGetString(values, "REDIS_URL"),
		},
		NotificationRedis: RedisConfig{
			URL: sharedconfig.MustGetString(values, "NOTIFICATION_REDIS_URL"),
		},
		FedaPay: loadFedaPayConfig(values, sharedconfig.MustGetString(values, "ENVIRONMENT")),
		BookingService: ServiceEndpoint{
			Host: sharedconfig.GetStringOrDefault(values, "BOOKING_SERVICE_HOST", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "BOOKING_SERVICE_PORT", "50058"),
		},
		UserService: ServiceEndpoint{
			Host: sharedconfig.GetStringOrDefault(values, "USER_SERVICE_HOST", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "USER_SERVICE_PORT", "50052"),
		},
		SupportService: ServiceEndpoint{
			Host: sharedconfig.GetStringOrDefault(values, "SUPPORT_SERVICE_HOST", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "SUPPORT_SERVICE_PORT", "50063"),
		},
		Payout: PayoutConfig{
			IntervalSeconds:        getIntOrDefault(values, "PAYOUT_INTERVAL_SECONDS", 1800),
			ContestationDelayHours: getIntOrDefault(values, "CONTESTATION_DELAY_HOURS", 2),
			PlatformFeePercent:     getIntOrDefault(values, "PLATFORM_FEE_PERCENT", 10),
		},
		Expiration: ExpirationConfig{
			IntervalSeconds:       getIntOrDefault(values, "EXPIRATION_INTERVAL_SECONDS", 60),
			PaymentTimeoutMinutes: getIntOrDefault(values, "PAYMENT_TIMEOUT_MINUTES", 5),
		},
		Refund: RefundConfig{
			CancellationFullRefundHours:    getIntOrDefault(values, "CANCELLATION_FULL_REFUND_HOURS", 24),
			CancellationGracePeriodMinutes: getIntOrDefault(values, "CANCELLATION_GRACE_PERIOD_MINUTES", 30),
			NoShowDriverDelayMinutes:       getIntOrDefault(values, "NOSHOW_DRIVER_DELAY_MINUTES", 15),
			NoShowPassengerDelayMinutes:    getIntOrDefault(values, "NOSHOW_PASSENGER_DELAY_MINUTES", 15),
		},
		RefundWorker: RefundWorkerConfig{
			IntervalSeconds: getIntOrDefault(values, "REFUND_WORKER_INTERVAL_SECONDS", 60),
			MaxRetries:      getIntOrDefault(values, "REFUND_MAX_RETRIES", 3),
			BatchSize:       getIntOrDefault(values, "REFUND_BATCH_SIZE", 20),
		},
		Worker: WorkerConfig{
			WebhookMaxConcurrent: getIntOrDefault(values, "WEBHOOK_MAX_CONCURRENT", 10),
			PayoutMaxConcurrent:  getIntOrDefault(values, "PAYOUT_MAX_CONCURRENT", 5),
			RefundMaxConcurrent:  getIntOrDefault(values, "REFUND_MAX_CONCURRENT", 5),
		},
		LogLevel: sharedconfig.MustGetString(values, "LOG_LEVEL"),
	}

	return cfg, nil
}

func loadFedaPayConfig(v *viper.Viper, environment string) FedaPayConfig {
	maxConcurrent := getIntOrDefault(v, "FEDAPAY_MAX_CONCURRENT", 10)

	switch environment {
	case "prod", "staging", "development", "vps-dev":
		return FedaPayConfig{
			APIURL:                sharedconfig.MustGetString(v, "FEDAPAY_LIVE_API_URL"),
			APIKey:                sharedconfig.MustGetString(v, "FEDAPAY_LIVE_API_KEY"),
			WebhookSecret:         sharedconfig.MustGetString(v, "FEDAPAY_LIVE_WEBHOOK_SECRET"),
			MaxConcurrentRequests: maxConcurrent,
		}
	default: // local
		return FedaPayConfig{
			APIURL:                sharedconfig.MustGetString(v, "FEDAPAY_SANDBOX_API_URL"),
			APIKey:                sharedconfig.MustGetString(v, "FEDAPAY_SANDBOX_API_KEY"),
			WebhookSecret:         sharedconfig.MustGetString(v, "FEDAPAY_SANDBOX_WEBHOOK_SECRET"),
			MaxConcurrentRequests: maxConcurrent,
		}
	}
}

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
