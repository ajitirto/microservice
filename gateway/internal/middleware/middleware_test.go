package middleware_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"gateway/internal/middleware"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

var requestIDPattern = regexp.MustCompile(`^[0-9a-f]{8}$`)

func TestRequestIDGeneratedWhenAbsent(t *testing.T) {
	var requestID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID = r.Header.Get(middleware.RequestIDHeader)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users/1", nil)
	rr := httptest.NewRecorder()
	middleware.RequestID(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if !requestIDPattern.MatchString(requestID) {
		t.Errorf("generated request ID = %q, want %v", requestID, requestIDPattern)
	}
	if got := rr.Header().Get(middleware.RequestIDHeader); got != requestID {
		t.Errorf("response X-Request-ID = %q, want %q", got, requestID)
	}
}

func TestRequestIDReusedWhenPresent(t *testing.T) {
	var requestID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID = r.Header.Get(middleware.RequestIDHeader)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users/1", nil)
	req.Header.Set(middleware.RequestIDHeader, "client-supplied-id")
	rr := httptest.NewRecorder()
	middleware.RequestID(next).ServeHTTP(rr, req)

	if requestID != "client-supplied-id" {
		t.Errorf("request ID = %q, want %q", requestID, "client-supplied-id")
	}
	if got := rr.Header().Get(middleware.RequestIDHeader); got != "client-supplied-id" {
		t.Errorf("response X-Request-ID = %q, want %q", got, "client-supplied-id")
	}
}

func TestRecoveryReturns500AndKeepsServing(t *testing.T) {
	next := middleware.Recovery(discardLogger())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/users/1", nil)
	rr := httptest.NewRecorder()
	next.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if body["error"] != "Internal Server Error" {
		t.Errorf("error field = %q, want %q", body["error"], "Internal Server Error")
	}

	ok := middleware.Recovery(discardLogger())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rr2 := httptest.NewRecorder()
	ok.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/api/users/1", nil))
	if rr2.Code != http.StatusOK {
		t.Errorf("subsequent request status = %d, want 200", rr2.Code)
	}
}

func TestTimeoutReturns504(t *testing.T) {
	slow := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})
	next := middleware.Timeout(50 * time.Millisecond)(slow)

	req := httptest.NewRequest(http.MethodGet, "/api/users/1", nil)
	rr := httptest.NewRecorder()
	next.ServeHTTP(rr, req)

	if rr.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want 504", rr.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if body["error"] != "Gateway Timeout" {
		t.Errorf("error field = %q, want %q", body["error"], "Gateway Timeout")
	}
}

func TestTimeoutLetsFastRequestsThrough(t *testing.T) {
	fast := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	next := middleware.Timeout(500 * time.Millisecond)(fast)

	rr := httptest.NewRecorder()
	next.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/users/1", nil))

	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rr.Code)
	}
}

func TestFullChain(t *testing.T) {
	chain := http.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(middleware.RequestIDHeader) == "" {
			t.Error("request ID missing inside chain")
		}
		w.WriteHeader(http.StatusOK)
	}))
	chain = middleware.Timeout(500 * time.Millisecond)(chain)
	chain = middleware.Logging(discardLogger())(chain)
	chain = middleware.RequestID(chain)
	chain = middleware.Recovery(discardLogger())(chain)

	req := httptest.NewRequest(http.MethodGet, "/api/users/1", nil)
	rr := httptest.NewRecorder()
	chain.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if got := rr.Header().Get(middleware.RequestIDHeader); !requestIDPattern.MatchString(got) {
		t.Errorf("response X-Request-ID = %q, want 8 hex chars", got)
	}
}

func TestLoggingRecordsStatus(t *testing.T) {
	var logOutput strings.Builder
	logger := slog.New(slog.NewTextHandler(&logOutput, nil))

	next := middleware.Logging(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusTeapot)
	}))

	rr := httptest.NewRecorder()
	next.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/users/1", nil))

	if rr.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want 418", rr.Code)
	}
	if !strings.Contains(logOutput.String(), "status=418") {
		t.Errorf("log output = %q, want status=418", logOutput.String())
	}
}
