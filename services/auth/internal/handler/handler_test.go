package handler_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"auth/internal/handler"
	"auth/internal/repository"
	"auth/internal/service"
)

func newTestMux() *http.ServeMux {
	repo := repository.NewInMemory()
	svc := service.New(repo.Users, repo.Refresh, "test-secret", 15*time.Minute, 24*time.Hour)
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

func decodeJSON(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v (body %q)", err, rr.Body.String())
	}
	return body
}

func loginViaAPI(t *testing.T, mux *http.ServeMux) string {
	t.Helper()
	rr := do(mux, http.MethodPost, "/auth/login", `{"email":"aji@example.com","password":"password123"}`, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200 (body %q)", rr.Code, rr.Body.String())
	}
	body := decodeJSON(t, rr)
	access, _ := body["access_token"].(string)
	if access == "" {
		t.Fatal("login response missing access_token")
	}
	return access
}

func TestRegister(t *testing.T) {
	mux := newTestMux()

	rr := do(mux, http.MethodPost, "/auth/register", `{"email":"siti@example.com","password":"secret123"}`, nil)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %q)", rr.Code, rr.Body.String())
	}
	body := decodeJSON(t, rr)
	for _, key := range []string{"user_id", "access_token", "refresh_token"} {
		if v, _ := body[key].(string); v == "" {
			t.Errorf("response missing %q: %v", key, body)
		}
	}
}

func TestRegisterDuplicate(t *testing.T) {
	mux := newTestMux()

	rr := do(mux, http.MethodPost, "/auth/register", `{"email":"aji@example.com","password":"secret123"}`, nil)
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (body %q)", rr.Code, rr.Body.String())
	}
}

func TestRegisterInvalidBody(t *testing.T) {
	mux := newTestMux()

	rr := do(mux, http.MethodPost, "/auth/register", `{not json`, nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestRegisterShortPassword(t *testing.T) {
	mux := newTestMux()

	rr := do(mux, http.MethodPost, "/auth/register", `{"email":"x@example.com","password":"short"}`, nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestLogin(t *testing.T) {
	mux := newTestMux()

	t.Run("valid credentials", func(t *testing.T) {
		rr := do(mux, http.MethodPost, "/auth/login", `{"email":"aji@example.com","password":"password123"}`, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %q)", rr.Code, rr.Body.String())
		}
		body := decodeJSON(t, rr)
		if body["token_type"] != "Bearer" {
			t.Errorf("token_type = %v, want Bearer", body["token_type"])
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		rr := do(mux, http.MethodPost, "/auth/login", `{"email":"aji@example.com","password":"wrong"}`, nil)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rr.Code)
		}
	})

	t.Run("unknown email", func(t *testing.T) {
		rr := do(mux, http.MethodPost, "/auth/login", `{"email":"ghost@example.com","password":"wrong-password"}`, nil)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rr.Code)
		}
	})
}

func TestVerify(t *testing.T) {
	mux := newTestMux()
	access := loginViaAPI(t, mux)

	t.Run("valid token", func(t *testing.T) {
		rr := do(mux, http.MethodGet, "/auth/verify", "", map[string]string{"Authorization": "Bearer " + access})
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %q)", rr.Code, rr.Body.String())
		}
		body := decodeJSON(t, rr)
		if body["user_id"] != "123" {
			t.Errorf("user_id = %v, want 123", body["user_id"])
		}
	})

	t.Run("no header", func(t *testing.T) {
		rr := do(mux, http.MethodGet, "/auth/verify", "", nil)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rr.Code)
		}
	})

	t.Run("wrong scheme", func(t *testing.T) {
		rr := do(mux, http.MethodGet, "/auth/verify", "", map[string]string{"Authorization": "Basic abc"})
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rr.Code)
		}
	})

	t.Run("tampered token", func(t *testing.T) {
		rr := do(mux, http.MethodGet, "/auth/verify", "", map[string]string{"Authorization": "Bearer " + access + "x"})
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rr.Code)
		}
	})
}

func TestRefreshAndLogout(t *testing.T) {
	mux := newTestMux()

	login := do(mux, http.MethodPost, "/auth/login", `{"email":"aji@example.com","password":"password123"}`, nil)
	body := decodeJSON(t, login)
	refreshToken, _ := body["refresh_token"].(string)
	if refreshToken == "" {
		t.Fatal("login response missing refresh_token")
	}

	t.Run("refresh valid", func(t *testing.T) {
		rr := do(mux, http.MethodPost, "/auth/refresh", `{"refresh_token":"`+refreshToken+`"}`, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %q)", rr.Code, rr.Body.String())
		}
		rotated := decodeJSON(t, rr)
		newRefresh, _ := rotated["refresh_token"].(string)
		if newRefresh == refreshToken {
			t.Error("refresh token should rotate")
		}
		refreshToken = newRefresh
	})

	t.Run("old refresh rejected", func(t *testing.T) {
		rr := do(mux, http.MethodPost, "/auth/refresh", `{"refresh_token":"og-old-token"}`, nil)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rr.Code)
		}
	})

	t.Run("logout", func(t *testing.T) {
		rr := do(mux, http.MethodPost, "/auth/logout", `{"refresh_token":"`+refreshToken+`"}`, nil)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", rr.Code)
		}
	})

	t.Run("refresh after logout rejected", func(t *testing.T) {
		rr := do(mux, http.MethodPost, "/auth/refresh", `{"refresh_token":"`+refreshToken+`"}`, nil)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rr.Code)
		}
	})
}

func TestHealth(t *testing.T) {
	mux := newTestMux()

	rr := do(mux, http.MethodGet, "/health", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := decodeJSON(t, rr)
	if body["status"] != "ok" {
		t.Errorf("status field = %v, want ok", body["status"])
	}
}
