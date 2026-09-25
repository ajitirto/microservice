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
		Port:     getEnv("USER_PORT", "8082"),
		LogLevel: getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
