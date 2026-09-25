// Package config loads the post service configuration from environment
// variables with sensible local-development defaults.
package config

import (
	"os"
)

type Config struct {
	Port            string
	LogLevel        string
	NotificationURL string
	UserGRPCAddr    string
	RedisURL        string
}

func Load() *Config {
	return &Config{
		Port:            getEnv("POST_PORT", "8083"),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		NotificationURL: getEnv("NOTIFICATION_SERVICE_URL", "http://notification:8084"),
		UserGRPCAddr:    getEnv("USER_GRPC_ADDR", "user:9090"),
		RedisURL:        getEnv("REDIS_URL", ""),
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
