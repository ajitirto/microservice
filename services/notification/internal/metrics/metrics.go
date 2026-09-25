// Package metrics exposes shared Prometheus HTTP instrumentation:
// request counts by method/route/status plus a duration histogram.
// Routes are normalized so dynamic ids never blow up label cardinality.
package metrics

import (
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests handled.",
		},
		[]string{"method", "route", "status"},
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route"},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration)
}

// Middleware records one observation per request, skipping requests to
// the metrics endpoint itself.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" || r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		route := normalizeRoute(r.URL.Path)
		httpRequestsTotal.WithLabelValues(r.Method, route, strconv.Itoa(rw.status)).Inc()
		httpRequestDuration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
	})
}

// Handler serves Prometheus metrics for scraping.
func Handler() http.Handler {
	return promhttp.Handler()
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

var (
	idSegment     = regexp.MustCompile(`/[0-9a-f]{16}(/|$)`)
	digitsSegment = regexp.MustCompile(`/\d+(/|$)`)
)

func normalizeRoute(path string) string {
	path = idSegment.ReplaceAllString(path, "/:id$1")
	return digitsSegment.ReplaceAllString(path, "/:id$1")
}
