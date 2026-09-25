// Package server wires the user service HTTP server and middleware chain.
package server

import (
	"log/slog"
	"net/http"
	"time"

	"user/internal/config"
	"user/internal/handler"
	"user/internal/middleware"
	"user/internal/repository"
	"user/internal/service"
)

func New(cfg *config.Config, logger *slog.Logger, repo repository.UserRepository) (*http.Server, error) {
	svc := service.New(repo)
	h := handler.New(svc, logger)

	mux := http.NewServeMux()
	h.SetRoutes(mux)

	// Middleware chain: recovery -> requestID -> logging -> mux
	var root http.Handler = mux
	root = middleware.Logging(logger)(root)
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
