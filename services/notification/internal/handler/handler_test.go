package handler_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"notification/internal/handler"
	"notification/internal/repository"
	"notification/internal/service"
)

func newTestMux() *http.ServeMux {
	svc := service.New(repository.NewInMemory())
	h := handler.New(svc, slog.New(slog.NewTextHandler(io.Discard, nil)))

	mux := http.NewServeMux()
	h.SetRoutes(mux)
	return mux
}

func do(mux *http.ServeMux, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func decodeBody(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return body
}

func TestIngest(t *testing.T) {
	mux := newTestMux()

	t.Run("valid event", func(t *testing.T) {
		rr := do(mux, http.MethodPost, "/events/post_created", `{"user_id":"123","post_id":"p9","title":"New"}`, nil)
		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201", rr.Code)
		}
		body := decodeBody(t, rr)
		if body["type"] != "post_created" || body["user_id"] != "123" {
			t.Errorf("body = %v, want type post_created user 123", body)
		}
		if id, ok := body["id"].(string); !ok || id == "" {
			t.Error("body should contain generated id")
		}
	})

	t.Run("whitespace event type", func(t *testing.T) {
		rr := do(mux, http.MethodPost, "/events/%20", `{"user_id":"123"}`, nil)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("missing user id", func(t *testing.T) {
		rr := do(mux, http.MethodPost, "/events/post_created", `{"title":"x"}`, nil)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})
}

func TestListNotifications(t *testing.T) {
	mux := newTestMux()

	t.Run("missing header", func(t *testing.T) {
		rr := do(mux, http.MethodGet, "/notifications", "", nil)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rr.Code)
		}
	})

	t.Run("owner list", func(t *testing.T) {
		rr := do(mux, http.MethodGet, "/notifications", "", map[string]string{"X-User-ID": "123"})
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		body := decodeBody(t, rr)
		items, ok := body["notifications"].([]any)
		if !ok {
			t.Fatalf("body = %v, want notifications array", body)
		}
		if len(items) != 2 {
			t.Errorf("got %d notifications, want 2", len(items))
		}
	})
}

func TestGetNotificationByID(t *testing.T) {
	mux := newTestMux()

	t.Run("owner", func(t *testing.T) {
		rr := do(mux, http.MethodGet, "/notifications/n1", "", map[string]string{"X-User-ID": "123"})
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
	})

	t.Run("unknown", func(t *testing.T) {
		rr := do(mux, http.MethodGet, "/notifications/nope", "", map[string]string{"X-User-ID": "123"})
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
	})

	t.Run("non-owner", func(t *testing.T) {
		rr := do(mux, http.MethodGet, "/notifications/n1", "", map[string]string{"X-User-ID": "999"})
		if rr.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", rr.Code)
		}
	})
}

func TestMarkNotificationRead(t *testing.T) {
	mux := newTestMux()

	t.Run("owner marks read", func(t *testing.T) {
		rr := do(mux, http.MethodPost, "/notifications/n1/read", "", map[string]string{"X-User-ID": "123"})
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		body := decodeBody(t, rr)
		if body["read"] != true {
			t.Errorf("body = %v, want read true", body)
		}
	})

	t.Run("missing header", func(t *testing.T) {
		rr := do(mux, http.MethodPost, "/notifications/n1/read", "", nil)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rr.Code)
		}
	})

	t.Run("non-owner", func(t *testing.T) {
		rr := do(mux, http.MethodPost, "/notifications/n1/read", "", map[string]string{"X-User-ID": "999"})
		if rr.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", rr.Code)
		}
	})
}

func TestHealth(t *testing.T) {
	mux := newTestMux()

	rr := do(mux, http.MethodGet, "/health", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != `{"status":"ok"}` {
		t.Errorf("body = %q, want ok", rr.Body.String())
	}
}
