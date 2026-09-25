package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"gateway/internal/authresolver"
)

const UserIDHeader = "X-User-ID"

// TokenVerifier resolves the user id behind an access token without
// exposing how tokens are validated.
type TokenVerifier interface {
	Verify(ctx context.Context, token string) (userID string, err error)
}

func Auth(verifier TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isPublic(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || token == "" {
				unauthorized(w, r, "missing bearer token")
				return
			}

			userID, err := verifier.Verify(r.Context(), token)
			if err != nil {
				if errors.Is(err, authresolver.ErrAuthUnavailable) {
					WriteJSON(w, http.StatusServiceUnavailable, errorBody(r, "auth service unavailable"))
					return
				}
				unauthorized(w, r, "invalid or expired token")
				return
			}

			r.Header.Set(UserIDHeader, userID)
			next.ServeHTTP(w, r)
		})
	}
}

func unauthorized(w http.ResponseWriter, r *http.Request, message string) {
	WriteJSON(w, http.StatusUnauthorized, errorBody(r, message))
}

func errorBody(r *http.Request, message string) map[string]string {
	return map[string]string{
		"error":      message,
		"request_id": r.Header.Get(RequestIDHeader),
	}
}

func isPublic(path string) bool {
	return path == "/health" || strings.HasPrefix(path, "/api/auth") || !strings.HasPrefix(path, "/api")
}
