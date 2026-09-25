package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gateway/internal/authresolver"
	"gateway/internal/config"
	"gateway/internal/ratelimit"
	"gateway/internal/server"

	"github.com/redis/go-redis/v9"
)

func buildLimiter(cfg *config.Config, logger *slog.Logger) ratelimit.Limiter {
	if cfg.RedisURL == "" {
		logger.Warn("REDIS_URL not set; rate limiting disabled")
		return ratelimit.NoopLimiter{}
	}

	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		logger.Warn("invalid REDIS_URL; rate limiting disabled", slog.Any("error", err))
		return ratelimit.NoopLimiter{}
	}
	opts.DialTimeout = 500 * time.Millisecond
	opts.ReadTimeout = 500 * time.Millisecond
	opts.WriteTimeout = 500 * time.Millisecond
	opts.PoolTimeout = 500 * time.Millisecond
	opts.MaxRetries = -1
	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		logger.Warn("redis unreachable; rate limiting disabled", slog.Any("error", err))
		_ = client.Close()
		return ratelimit.NoopLimiter{}
	}

	return ratelimit.NewRedisLimiter(client, cfg.RateLimitPerMinute, time.Minute)
}

func main() {
	cfg := config.Load()

	var level slog.Level
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	verifier, err := authresolver.New(authresolver.Config{
		Address:     cfg.AuthGRPCAddr,
		Timeout:     500 * time.Millisecond,
		MaxAttempts: 2,
		BaseBackoff: 50 * time.Millisecond,
		Logger:      logger,
	})
	if err != nil {
		logger.Error("failed to create auth resolver", slog.Any("error", err))
		os.Exit(1)
	}
	defer verifier.Close()

	limiter := buildLimiter(cfg, logger)

	h, err := server.New(cfg, logger, verifier, limiter)
	if err != nil {
		logger.Error("failed to create server", slog.Any("error", err))
		os.Exit(1)
	}

	srv := server.NewHTTPServer(cfg.Port, h)

	go func() {
		logger.Info("starting gateway server", slog.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()

	logger.Info("shutting down gateway server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server forced to shutdown", slog.Any("error", err))
		os.Exit(1)
	}

	logger.Info("server exited gracefully")
}
