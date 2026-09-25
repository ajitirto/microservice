package handler_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"user/internal/handler"
	"user/internal/model"
	"user/internal/repository"
	"user/internal/service"
)

func newTestMux() (*http.ServeMux, *repository.InMemoryUserRepository) {
	repo := repository.NewInMemory()
	svc := service.New(repo)
	h := handler.New(svc, slog.New(slog.NewTextHandler(io.Discard, nil)))

	mux := http.NewServeMux()
	h.SetRoutes(mux)
	return mux, repo
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

func decodeUser(t *testing.T, rr *httptest.ResponseRecorder) model.User {
	t.Helper()
	var u model.User
	if err := json.Unmarshal(rr.Body.Bytes(), &u); err != nil {
		t.Fatalf("response is not a user JSON: %v (body %q)", err, rr.Body.String())
	}
	return u
}

func decodeErr(t *testing.T, rr *httptest.ResponseRecorder) map[string]string {
	t.Helper()
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v (body %q)", err, rr.Body.String())
	}
	return body
}

func TestGetUserByID(t *testing.T) {
	mux, _ := newTestMux()

	tests := []struct {
		name     string
		path     string
		wantCode int
	}{
		{name: "existing", path: "/users/123", wantCode: http.StatusOK},
		{name: "missing", path: "/users/999", wantCode: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := do(mux, http.MethodGet, tt.path, "", nil)
			if rr.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d (body %q)", rr.Code, tt.wantCode, rr.Body.String())
			}
			if tt.wantCode == http.StatusOK {
				u := decodeUser(t, rr)
				if u.ID != "123" || u.Name != "Aji" {
					t.Errorf("got %+v, want seeded user", u)
				}
			} else {
				body := decodeErr(t, rr)
				if body["error"] != "user not found" {
					t.Errorf("error = %q, want %q", body["error"], "user not found")
				}
			}
		})
	}
}

func TestGetMe(t *testing.T) {
	mux, _ := newTestMux()

	t.Run("with header", func(t *testing.T) {
		rr := do(mux, http.MethodGet, "/users/me", "", map[string]string{"X-User-ID": "123"})
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %q)", rr.Code, rr.Body.String())
		}
		if u := decodeUser(t, rr); u.ID != "123" {
			t.Errorf("id = %q, want 123", u.ID)
		}
	})

	t.Run("without header", func(t *testing.T) {
		rr := do(mux, http.MethodGet, "/users/me", "", nil)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rr.Code)
		}
		if body := decodeErr(t, rr); body["error"] != "missing X-User-ID header" {
			t.Errorf("error = %q, want %q", body["error"], "missing X-User-ID header")
		}
	})
}

func TestUpdateUser(t *testing.T) {
	mux, _ := newTestMux()

	t.Run("valid update", func(t *testing.T) {
		rr := do(mux, http.MethodPut, "/users/123", `{"name":"Budi"}`, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %q)", rr.Code, rr.Body.String())
		}
		if u := decodeUser(t, rr); u.Name != "Budi" {
			t.Errorf("name = %q, want %q", u.Name, "Budi")
		}
	})

	t.Run("malformed json", func(t *testing.T) {
		rr := do(mux, http.MethodPut, "/users/123", `{not json`, nil)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("no fields", func(t *testing.T) {
		rr := do(mux, http.MethodPut, "/users/123", `{}`, nil)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("invalid email", func(t *testing.T) {
		rr := do(mux, http.MethodPut, "/users/123", `{"email":"nope"}`, nil)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("unknown user", func(t *testing.T) {
		rr := do(mux, http.MethodPut, "/users/999", `{"name":"X"}`, nil)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
	})
}

func TestDeleteUser(t *testing.T) {
	mux, _ := newTestMux()

	t.Run("existing", func(t *testing.T) {
		rr := do(mux, http.MethodDelete, "/users/123", "", nil)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", rr.Code)
		}
	})

	t.Run("already deleted", func(t *testing.T) {
		rr := do(mux, http.MethodDelete, "/users/123", "", nil)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
	})
}

func TestHealth(t *testing.T) {
	mux, _ := newTestMux()

	rr := do(mux, http.MethodGet, "/health", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := decodeErr(t, rr)
	if body["status"] != "ok" {
		t.Errorf("status field = %q, want %q", body["status"], "ok")
	}
}
