// Package server wires the post service HTTP server and middleware chain.
package server

import (
	"log/slog"
	"net/http"
	"time"

	"post/internal/config"
	"post/internal/handler"
	"post/internal/metrics"
	"post/internal/middleware"
	"post/internal/repository"
	"post/internal/service"
)

func New(cfg *config.Config, logger *slog.Logger, repo repository.PostRepository, pub service.Publisher, users service.UserResolver) (*http.Server, error) {
	svc := service.New(repo, pub, users)
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
