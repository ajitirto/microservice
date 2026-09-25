package middleware

import (
	"net/http"

	"auth/internal/trace"
)

// Trace starts (or continues) a W3C trace context for every request so
// logs and downstream calls can be correlated across services by
// trace_id. The span is read by the logging middleware and by clients
// that forward the traceparent header.
func Trace(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parent, _ := trace.ParseTraceparent(r.Header.Get("traceparent"))
		r = r.WithContext(trace.WithSpan(r.Context(), trace.New(parent)))
		next.ServeHTTP(w, r)
	})
}
