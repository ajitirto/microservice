package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gateway/internal/authresolver"
	"gateway/internal/middleware"
)

type fakeVerifier struct {
	userID string
	err    error
	calls  int
}

func (f *fakeVerifier) Verify(ctx context.Context, token string) (string, error) {
	f.calls++
	if f.err != nil {
		return "", f.err
	}
	return f.userID, nil
}

func echoUserID(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(r.Header.Get(middleware.UserIDHeader)))
}

func authedMux(verifier middleware.TokenVerifier) *http.ServeMux {
	h := middleware.Auth(verifier)(http.HandlerFunc(echoUserID))
	mux := http.NewServeMux()
	mux.Handle("/api/posts", h)
	mux.Handle("/api/auth/login", h)
	mux.Handle("/health", h)
	mux.Handle("/", h)
	return mux
}

func TestAuthPublicPathsSkipVerification(t *testing.T) {
	verifier := &fakeVerifier{userID: "123"}
	mux := authedMux(verifier)

	for _, path := range []string{"/health", "/api/auth/login", "/"} {
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		if rr.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200 (public path)", path, rr.Code)
		}
	}
	if verifier.calls != 0 {
		t.Errorf("verifier called %d times, want 0 for public paths", verifier.calls)
	}
}

func TestAuthRejectsMissingBearer(t *testing.T) {
	verifier := &fakeVerifier{userID: "123"}
	mux := authedMux(verifier)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/posts", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	if verifier.calls != 0 {
		t.Errorf("verifier called %d times, want 0", verifier.calls)
	}
}

func TestAuthRejectsInvalidToken(t *testing.T) {
	verifier := &fakeVerifier{err: authresolver.ErrInvalidToken}
	mux := authedMux(verifier)

	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	req.Header.Set("Authorization", "Bearer garbage")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestAuthUnavailable(t *testing.T) {
	verifier := &fakeVerifier{err: authresolver.ErrAuthUnavailable}
	mux := authedMux(verifier)

	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	req.Header.Set("Authorization", "Bearer x")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rr.Code)
	}
}

func TestAuthInjectsUserIDFromToken(t *testing.T) {
	verifier := &fakeVerifier{userID: "123"}
	mux := authedMux(verifier)

	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	req.Header.Set("Authorization", "Bearer good")
	req.Header.Set(middleware.UserIDHeader, "spoofed")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if got := strings.TrimSpace(rr.Body.String()); got != "123" {
		t.Errorf("X-User-ID = %q, want 123 (client spoof overwritten)", got)
	}
}
