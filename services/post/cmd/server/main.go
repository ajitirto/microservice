package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"post/internal/config"
	"post/internal/publisher"
	"post/internal/repository"
	"post/internal/server"
	"post/internal/userresolver"
)

func main() {
	cfg := config.Load()
	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	repo := repository.NewInMemory()
	pub := publisher.NewHTTP(cfg.NotificationURL)

	users, err := userresolver.New(userresolver.Config{
		Address:     cfg.UserGRPCAddr,
		Timeout:     time.Second,
		MaxAttempts: 3,
		BaseBackoff: 50 * time.Millisecond,
		Logger:      logger,
	})
	if err != nil {
		logger.Error("failed to init user resolver", "error", err)
		os.Exit(1)
	}
	defer users.Close()

	if cfg.RedisURL != "" {
		cache, err := userresolver.NewRedisCache(cfg.RedisURL)
		if err != nil {
			logger.Warn("invalid REDIS_URL, running without cache", "error", err)
		} else {
			users.AttachCache(cache)
		}
	}

	srv, err := server.New(cfg, logger, repo, pub, users)
	if err != nil {
		logger.Error("invalid server configuration", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("post service listening", "addr", srv.Addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		logger.Info("shutdown signal received, draining connections")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
			os.Exit(1)
		}
		logger.Info("server stopped")
	}
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
