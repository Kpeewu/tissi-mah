package config

import (
	"fmt"
	"strconv"

	sharedconfig "github.com/Kpeewu/tissi-mah/pkg/config"
	"github.com/spf13/viper"
)

func getIntOrDefault(v *viper.Viper, key string, def int) int {
	s := v.GetString(key)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

type Config struct {
	Server        ServerConfig
	Environment   EnvironmentConfig
	Database      DatabaseConfig
	Redis         RedisConfig
	EmailService  EmailServiceConfig
	JWT           JWTConfig
	OTP           OTPConfig
	RateLimit     RateLimitConfig
	PasswordReset PasswordResetConfig
	FrontendURL   string
	LogLevel      string
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

type RedisConfig struct {
	URL string
}

type EmailServiceConfig struct {
	Address string
	Port    string
}

type JWTConfig struct {
	Secret          string
	AccessTTLHours  int
	RefreshTTLHours int
}

type OTPConfig struct {
	TTLSeconds        int
	MaxAttempts       int
	ResendCooldownSec int
}

type RateLimitConfig struct {
	FailThreshold     int
	FailWindowSeconds int
}

type PasswordResetConfig struct {
	TTLSeconds int
}

func Load() (*Config, error) {
	values, err := sharedconfig.Load("")
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Server: ServerConfig{
			Host: sharedconfig.GetStringOrDefault(values, "GRPC_ADDRESS", "0.0.0.0"),
			Port: sharedconfig.GetStringOrDefault(values, "GRPC_PORT", "50063"),
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
		EmailService: EmailServiceConfig{
			Address: sharedconfig.GetStringOrDefault(values, "EMAIL_SERVICE_HOST", "0.0.0.0"),
			Port:    sharedconfig.GetStringOrDefault(values, "EMAIL_SERVICE_PORT", "50061"),
		},
		JWT: JWTConfig{
			Secret:          sharedconfig.MustGetString(values, "JWT_SECRET"),
			AccessTTLHours:  getIntOrDefault(values, "JWT_TTL_HOURS", 12),
			RefreshTTLHours: getIntOrDefault(values, "REFRESH_TOKEN_TTL_HOURS", 720),
		},
		OTP: OTPConfig{
			TTLSeconds:        getIntOrDefault(values, "OTP_TTL_SECONDS", 300),
			MaxAttempts:       getIntOrDefault(values, "OTP_MAX_ATTEMPTS", 3),
			ResendCooldownSec: getIntOrDefault(values, "OTP_RESEND_COOLDOWN_SECONDS", 300),
		},
		RateLimit: RateLimitConfig{
			FailThreshold:     getIntOrDefault(values, "LOGIN_FAIL_THRESHOLD", 5),
			FailWindowSeconds: getIntOrDefault(values, "LOGIN_FAIL_WINDOW_SECONDS", 86400),
		},
		PasswordReset: PasswordResetConfig{
			TTLSeconds: getIntOrDefault(values, "PASSWORD_RESET_TTL_SECONDS", 3600),
		},
		FrontendURL: sharedconfig.GetStringOrDefault(values, "SUPPORT_FRONTEND_URL", ""),
		LogLevel:    sharedconfig.MustGetString(values, "LOG_LEVEL"),
	}

	if err := validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func validate(c *Config) error {
	if c.Database.URL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.Redis.URL == "" {
		return fmt.Errorf("REDIS_URL is required")
	}
	if len(c.JWT.Secret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 bytes")
	}
	if c.Environment.Mode != "local" && c.FrontendURL == "" {
		return fmt.Errorf("SUPPORT_FRONTEND_URL is required outside local")
	}
	return nil
}
