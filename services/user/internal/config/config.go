package config

import (
	"os"
)

type Config struct {
	Port     string
	GRPCPort string
	LogLevel string
}

func Load() *Config {
	return &Config{
		Port:     getEnv("USER_PORT", "8082"),
		GRPCPort: getEnv("USER_GRPC_PORT", "9090"),
		LogLevel: getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
