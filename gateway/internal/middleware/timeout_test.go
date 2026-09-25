package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTimeout(t *testing.T) {
	t.Run("responds with 504 on timeout", func(t *testing.T) {
		timeout := 10 * time.Millisecond
		handler := Timeout(timeout)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(50 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusGatewayTimeout {
			t.Errorf("expected 504, got %d", status)
		}

		if !strings.Contains(rr.Body.String(), "Gateway Timeout") {
			t.Errorf("expected error message, got %s", rr.Body.String())
		}
	})

	t.Run("succeeds within timeout", func(t *testing.T) {
		timeout := 100 * time.Millisecond
		handler := Timeout(timeout)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("expected 200, got %d", status)
		}
	})
}
