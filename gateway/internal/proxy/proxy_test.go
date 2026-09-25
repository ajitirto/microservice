package proxy_test

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gateway/internal/proxy"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func echo(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s %s", path, r.URL.RequestURI())
	}
}

func mustNewProxy(t *testing.T, authURL, userURL, postURL, notifURL string, timeout time.Duration) *proxy.Proxy {
	t.Helper()
	p, err := proxy.New(authURL, userURL, postURL, notifURL, timeout, discardLogger())
	if err != nil {
		t.Fatalf("proxy.New: %v", err)
	}
	return p
}

func doRequest(t *testing.T, p *proxy.Proxy, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	p.ServeHTTP(rr, req)
	return rr
}

func TestRoutingForwardsToBackend(t *testing.T) {
	userSrv := httptest.NewServer(echo("U"))
	apiSrv := httptest.NewServer(echo("A"))
	defer userSrv.Close()
	defer apiSrv.Close()

	p := mustNewProxy(t, apiSrv.URL, userSrv.URL, apiSrv.URL, apiSrv.URL, 5*time.Second)

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "users with id", path: "/api/users/123", want: "U /users/123"},
		{name: "users exact", path: "/api/users", want: "U /users"},
		{name: "users trailing slash", path: "/api/users/", want: "U /users/"},
		{name: "auth", path: "/api/auth/login", want: "A /auth/login"},
		{name: "posts with query", path: "/api/posts?page=2&limit=10", want: "A /posts?page=2&limit=10"},
		{name: "notifications", path: "/api/notifications/7", want: "A /notifications/7"},
		{name: "post method", path: "/api/posts", want: "A /posts"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method := http.MethodGet
			if tt.name == "post method" {
				method = http.MethodPost
			}
			rr := doRequest(t, p, method, tt.path, nil)
			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d (body %q)", rr.Code, http.StatusOK, rr.Body.String())
			}
			if got := strings.TrimSpace(rr.Body.String()); got != tt.want {
				t.Errorf("body = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUnknownRouteReturns404(t *testing.T) {
	backend := httptest.NewServer(echo("A"))
	defer backend.Close()

	p := mustNewProxy(t, backend.URL, backend.URL, backend.URL, backend.URL, 5*time.Second)

	for _, path := range []string{"/api/unknown", "/api", "/not-an-api", "/"} {
		rr := doRequest(t, p, http.MethodGet, path, nil)
		if rr.Code != http.StatusNotFound {
			t.Errorf("path %s: status = %d, want 404 (body %q)", path, rr.Code, rr.Body.String())
		}
		var body map[string]string
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatalf("path %s: response is not JSON: %v", path, err)
		}
		if body["error"] != "Not Found" {
			t.Errorf("path %s: error field = %q, want %q", path, body["error"], "Not Found")
		}
	}
}

func TestRequestIDPropagatesToBackend(t *testing.T) {
	var gotRequestID string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequestID = r.Header.Get("X-Request-ID")
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	p := mustNewProxy(t, backend.URL, backend.URL, backend.URL, backend.URL, 5*time.Second)
	rr := doRequest(t, p, http.MethodGet, "/api/users/1", map[string]string{"X-Request-ID": "abc12345"})

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if gotRequestID != "abc12345" {
		t.Errorf("backend received X-Request-ID = %q, want %q", gotRequestID, "abc12345")
	}
}

func TestUnreachableBackendReturns502(t *testing.T) {
	closed := httptest.NewServer(echo("A"))
	closed.Close()

	p := mustNewProxy(t, closed.URL, closed.URL, closed.URL, closed.URL, 5*time.Second)
	rr := doRequest(t, p, http.MethodGet, "/api/users/1", nil)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502 (body %q)", rr.Code, rr.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if body["error"] != "Bad Gateway" {
		t.Errorf("error field = %q, want %q", body["error"], "Bad Gateway")
	}
}

func TestSlowBackendReturns504(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer slow.Close()

	p := mustNewProxy(t, slow.URL, slow.URL, slow.URL, slow.URL, 50*time.Millisecond)
	rr := doRequest(t, p, http.MethodGet, "/api/users/1", nil)

	if rr.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want 504 (body %q)", rr.Code, rr.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if body["error"] != "Gateway Timeout" {
		t.Errorf("error field = %q, want %q", body["error"], "Gateway Timeout")
	}
}
