// Package logger provides shared logging utilities for all services.
package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config holds logger configuration.
type Config struct {
	Level       string // debug, info, warn, error
	Environment string // local, development, staging, production
	ServiceName string
}

// New creates a new logger instance.
func New(cfg Config) (*zap.Logger, error) {
	var zapConfig zap.Config

	if cfg.Environment == "production" || cfg.Environment == "staging" {
		zapConfig = zap.NewProductionConfig()
	} else {
		zapConfig = zap.NewDevelopmentConfig()
		zapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	// Set log level
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		level = zapcore.InfoLevel
	}
	zapConfig.Level = zap.NewAtomicLevelAt(level)

	logger, err := zapConfig.Build(
		zap.Fields(
			zap.String("service", cfg.ServiceName),
			zap.String("environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, err
	}

	return logger, nil
}

// NewDefault creates a default logger for quick setup.
func NewDefault(serviceName string) *zap.Logger {
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = "local"
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	logger, err := New(Config{
		Level:       logLevel,
		Environment: env,
		ServiceName: serviceName,
	})
	if err != nil {
		// Fallback to basic logger
		return zap.NewExample()
	}

	return logger
}
