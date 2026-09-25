// Package middleware provides the post service HTTP middleware chain:
// recovery, request ID, and logging.
package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

const RequestIDHeader = "X-Request-ID"

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(RequestIDHeader)
		if requestID == "" {
			requestID = generateID()
		}

		r.Header.Set(RequestIDHeader, requestID)
		w.Header().Set(RequestIDHeader, requestID)

		next.ServeHTTP(w, r)
	})
}

func generateID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "00000000"
	}
	return hex.EncodeToString(b)
}
