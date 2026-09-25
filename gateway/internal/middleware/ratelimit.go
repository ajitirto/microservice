package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"strings"

	"gateway/internal/ratelimit"
)

// RateLimit rejects requests that exceed the limiter's per-client
// allowance. Redis errors fail open so an outage never blocks traffic.
func RateLimit(limiter ratelimit.Limiter, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" || !strings.HasPrefix(r.URL.Path, "/api") {
				next.ServeHTTP(w, r)
				return
			}

			ok, err := limiter.Allow(r.Context(), "rl:"+clientIP(r))
			if err != nil {
				logger.Warn("rate limiter unavailable; allowing request", "error", err)
				next.ServeHTTP(w, r)
				return
			}
			if !ok {
				WriteJSON(w, http.StatusTooManyRequests, errorBody(r, "too many requests"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
