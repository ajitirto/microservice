package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"auth/internal/authpb"
	"auth/internal/config"
	authgrpc "auth/internal/grpc"
	"auth/internal/repository"
	"auth/internal/server"
	"auth/internal/service"
)

func main() {
	cfg := config.Load()
	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	repo := repository.NewInMemory()
	svc := service.New(repo.Users, repo.Refresh, cfg.Secret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)

	srv, err := server.New(cfg, logger, repo)
	if err != nil {
		logger.Error("invalid server configuration", "error", err)
		os.Exit(1)
	}

	grpcListener, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		logger.Error("failed to listen for gRPC", "error", err, "port", cfg.GRPCPort)
		os.Exit(1)
	}
	grpcServer := grpc.NewServer()
	authpb.RegisterAuthServiceServer(grpcServer, authgrpc.NewServer(svc, logger))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 2)
	go func() {
		logger.Info("auth service listening", "addr", srv.Addr)
		errCh <- srv.ListenAndServe()
	}()
	go func() {
		logger.Info("auth gRPC service listening", "addr", grpcListener.Addr().String())
		if err := grpcServer.Serve(grpcListener); err != nil {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		logger.Info("shutdown signal received, draining connections")
		grpcServer.GracefulStop()
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
