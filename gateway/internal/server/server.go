package server

import (
	"log/slog"
	"net/http"
	"time"

	"gateway/internal/config"
	"gateway/internal/handler"
	"gateway/internal/metrics"
	"gateway/internal/middleware"
	"gateway/internal/proxy"
	"gateway/internal/ratelimit"
)

func New(cfg *config.Config, logger *slog.Logger, verifier middleware.TokenVerifier, limiter ratelimit.Limiter) (http.Handler, error) {
	p, err := proxy.New(
		cfg.AuthServiceURL,
		cfg.UserServiceURL,
		cfg.PostServiceURL,
		cfg.NotificationServiceURL,
		cfg.Timeout,
		logger,
	)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", handler.Health)
	mux.Handle("/metrics", metrics.Handler())
	mux.Handle("/api", p)
	mux.Handle("/api/", p)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		middleware.WriteJSON(w, http.StatusNotFound, map[string]string{
			"error":      http.StatusText(http.StatusNotFound),
			"request_id": r.Header.Get(middleware.RequestIDHeader),
		})
	})

	// Middleware chain: metrics -> recovery -> requestID -> trace -> logging -> timeout -> rateLimit -> auth -> router
	var h http.Handler = mux
	h = middleware.Auth(verifier)(h)
	h = middleware.RateLimit(limiter, logger)(h)
	h = middleware.Timeout(cfg.Timeout)(h)
	h = middleware.Logging(logger)(h)
	h = middleware.Trace(h)
	h = middleware.RequestID(h)
	h = middleware.Recovery(logger)(h)
	h = metrics.Middleware(h)

	return h, nil
}

func NewHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              ":" + addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}
