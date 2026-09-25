package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port                   string
	Timeout                time.Duration
	AuthServiceURL         string
	UserServiceURL         string
	PostServiceURL         string
	NotificationServiceURL string
	AuthGRPCAddr           string
	RedisURL               string
	RateLimitPerMinute     uint64
	LogLevel               string
}

func Load() *Config {
	return &Config{
		Port:                   getEnv("GATEWAY_PORT", "8080"),
		Timeout:                getDurationEnv("GATEWAY_TIMEOUT", "10s"),
		AuthServiceURL:         getEnv("AUTH_SERVICE_URL", "http://auth:8081"),
		UserServiceURL:         getEnv("USER_SERVICE_URL", "http://user:8082"),
		PostServiceURL:         getEnv("POST_SERVICE_URL", "http://post:8083"),
		NotificationServiceURL: getEnv("NOTIFICATION_SERVICE_URL", "http://notification:8084"),
		AuthGRPCAddr:           getEnv("AUTH_GRPC_ADDR", "auth:9091"),
		RedisURL:               getEnv("REDIS_URL", ""),
		RateLimitPerMinute:     getUintEnv("GATEWAY_RATE_LIMIT", 60),
		LogLevel:               getEnv("LOG_LEVEL", "info"),
	}
}

func getUintEnv(key string, defaultValue uint64) uint64 {
	value := getEnv(key, "")
	if value == "" {
		return defaultValue
	}
	n, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return defaultValue
	}
	return n
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getDurationEnv(key, defaultValue string) time.Duration {
	value := getEnv(key, defaultValue)
	d, err := time.ParseDuration(value)
	if err != nil {
		d, _ = time.ParseDuration(defaultValue)
	}
	return d
}
