// Package config loads the notification service configuration from
// environment variables with sensible local-development defaults.
package config

import (
	"os"
)

type Config struct {
	Port        string
	LogLevel    string
	RabbitMQURL string
}

func Load() *Config {
	return &Config{
		Port:        getEnv("NOTIFICATION_PORT", "8084"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		RabbitMQURL: getEnv("RABBITMQ_URL", ""),
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
