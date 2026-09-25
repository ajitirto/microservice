package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestID(t *testing.T) {
	t.Run("generates new ID if absent", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()

		handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(RequestIDHeader)
			if id == "" {
				t.Error("Request-ID not set in request header")
			}
		}))

		handler.ServeHTTP(rr, req)

		id := rr.Header().Get(RequestIDHeader)
		if id == "" {
			t.Error("Request-ID not set in response header")
		}
		if len(id) != 8 {
			t.Errorf("expected 8 hex chars, got %d", len(id))
		}
	})

	t.Run("reuses existing ID if present", func(t *testing.T) {
		existingID := "test-id"
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set(RequestIDHeader, existingID)
		rr := httptest.NewRecorder()

		handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(RequestIDHeader)
			if id != existingID {
				t.Errorf("expected %s, got %s", existingID, id)
			}
		}))

		handler.ServeHTTP(rr, req)

		id := rr.Header().Get(RequestIDHeader)
		if id != existingID {
			t.Errorf("expected %s, got %s", existingID, id)
		}
	})
}
