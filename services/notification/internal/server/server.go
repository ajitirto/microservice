// Package server wires the notification service HTTP server and
// middleware chain.
package server

import (
	"log/slog"
	"net/http"
	"time"

	"notification/internal/config"
	"notification/internal/handler"
	"notification/internal/metrics"
	"notification/internal/middleware"
	"notification/internal/repository"
	"notification/internal/service"
)

func New(cfg *config.Config, logger *slog.Logger, repo repository.NotificationRepository) (*http.Server, error) {
	svc := service.New(repo)
	h := handler.New(svc, logger)

	mux := http.NewServeMux()
	h.SetRoutes(mux)
	mux.Handle("/metrics", metrics.Handler())

	// Middleware chain: recovery -> requestID -> trace -> logging -> metrics -> mux
	var root http.Handler = mux
	root = metrics.Middleware(root)
	root = middleware.Logging(logger)(root)
	root = middleware.Trace(root)
	root = middleware.RequestID(root)
	root = middleware.Recovery(logger)(root)

	return NewHTTPServer(cfg.Port, root), nil
}

func NewHTTPServer(port string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}
