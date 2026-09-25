// Package config loads the post service configuration from environment
// variables with sensible local-development defaults.
package config

import (
	"os"
)

type Config struct {
	Port     string
	LogLevel string
}

func Load() *Config {
	return &Config{
		Port:     getEnv("POST_PORT", "8083"),
		LogLevel: getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
