package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					requestID := r.Header.Get(RequestIDHeader)
					logger.Error("panic recovered",
						slog.Any("error", err),
						slog.String("stack", string(debug.Stack())),
						slog.String("request_id", requestID),
					)
					WriteJSON(w, http.StatusInternalServerError, map[string]string{
						"error":      http.StatusText(http.StatusInternalServerError),
						"request_id": requestID,
					})
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
