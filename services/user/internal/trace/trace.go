// Package trace implements minimal W3C trace context propagation. A
// Span identifies one unit of work: the TraceID ties every hop of a
// request together, while SpanID and ParentID describe the current hop
// and the one above it. Only the traceparent header is used (no
// tracestate), which is enough for correlating logs across the gateway
// and downstream services without an external trace backend.
package trace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
)

// Span is one hop in a distributed trace.
type Span struct {
	TraceID  string
	SpanID   string
	ParentID string
}

// New returns a child of parent, or a new root span when parent is nil.
func New(parent *Span) *Span {
	s := &Span{TraceID: randomHex(16), SpanID: randomHex(8)}
	if parent != nil {
		s.TraceID = parent.TraceID
		s.ParentID = parent.SpanID
	}
	return s
}

// ParseTraceparent decodes a W3C traceparent header value in the form
// "00-<trace-id>-<span-id>-<flags>". It returns ok=false for any
// malformed value so callers fall back to starting a new trace.
func ParseTraceparent(v string) (*Span, bool) {
	parts := strings.Split(v, "-")
	if len(parts) != 4 || parts[0] != "00" {
		return nil, false
	}
	if len(parts[1]) != 32 || len(parts[2]) != 16 {
		return nil, false
	}
	if _, err := hex.DecodeString(parts[1]); err != nil {
		return nil, false
	}
	if _, err := hex.DecodeString(parts[2]); err != nil {
		return nil, false
	}
	return &Span{TraceID: parts[1], SpanID: parts[2]}, true
}

// Traceparent renders the span as a W3C traceparent header value.
func (s *Span) Traceparent() string {
	return "00-" + s.TraceID + "-" + s.SpanID + "-01"
}

type ctxKey struct{}

// WithSpan stores the span in ctx for downstream middleware and
// clients to read.
func WithSpan(ctx context.Context, s *Span) context.Context {
	return context.WithValue(ctx, ctxKey{}, s)
}

// SpanFrom returns the current span, or nil when the context carries none.
func SpanFrom(ctx context.Context) *Span {
	s, _ := ctx.Value(ctxKey{}).(*Span)
	return s
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// getrandom failure is fatal enough; zero-fill keeps
		// correlation working instead of panicking.
		return strings.Repeat("0", 2*n)
	}
	return hex.EncodeToString(b)
}
