// Package config provides shared configuration utilities for all services.
package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Load loads configuration from environment variables and optional config file.
// The prefix is used for environment variables (e.g., "AUTH" -> "AUTH_DATABASE_URL").
func Load(prefix string, configPaths ...string) (*viper.Viper, error) {
	v := viper.New()

	// Environment variables
	v.SetEnvPrefix(prefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	// Config file (optional)
	if len(configPaths) > 0 {
		for _, path := range configPaths {
			v.AddConfigPath(path)
		}
		v.SetConfigName("config")
		v.SetConfigType("yaml")

		if err := v.ReadInConfig(); err != nil {
			// Config file is optional, only log if it's not a "not found" error
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return nil, fmt.Errorf("failed to read config file: %w", err)
			}
		}
	}

	return v, nil
}

// MustGetString gets a string value or panics if not set.
func MustGetString(v *viper.Viper, key string) string {
	value := v.GetString(key)
	if value == "" {
		panic(fmt.Sprintf("required config key %q is not set", key))
	}
	return value
}

// GetStringOrDefault gets a string value or returns the default.
func GetStringOrDefault(v *viper.Viper, key, defaultValue string) string {
	value := v.GetString(key)
	if value == "" {
		return defaultValue
	}
	return value
}
