// Package config loads the auth service configuration from environment
// variables with sensible local-development defaults.
package config

import (
	"os"
	"time"
)

type Config struct {
	Port            string
	GRPCPort        string
	Secret          string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	DatabaseURL     string
	LogLevel        string
}

func Load() *Config {
	return &Config{
		Port:            getEnv("AUTH_PORT", "8081"),
		GRPCPort:        getEnv("AUTH_GRPC_PORT", "9091"),
		Secret:          getEnv("AUTH_SECRET", "dev-secret-change-me"),
		AccessTokenTTL:  getDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL: getDuration("REFRESH_TOKEN_TTL", 24*time.Hour),
		DatabaseURL:     getEnv("DATABASE_URL", ""),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getDuration(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}
