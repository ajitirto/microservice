package trace

import "testing"

func TestParseTraceparentValid(t *testing.T) {
	span, ok := ParseTraceparent("00-0123456789abcdef0123456789abcdef-0123456789abcdef-01")
	if !ok {
		t.Fatal("expected valid traceparent to parse")
	}
	if span.TraceID != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("unexpected trace id: %s", span.TraceID)
	}
	if span.SpanID != "0123456789abcdef" {
		t.Fatalf("unexpected span id: %s", span.SpanID)
	}
}

func TestParseTraceparentMalformed(t *testing.T) {
	bad := []string{
		"",
		"00-0123456789abcdef0123456789abcdef",
		"01-0123456789abcdef0123456789abcdef-0123456789abcdef-01",
		"00-0123456789abcdef0123456789abcdef-0123456789abcde-01",
		"00-zz23456789abcdef0123456789abcdef-0123456789abcdef-01",
	}
	for _, v := range bad {
		if _, ok := ParseTraceparent(v); ok {
			t.Fatalf("expected %q to be rejected", v)
		}
	}
}

func TestNewChildKeepsTraceID(t *testing.T) {
	parent := New(nil)
	child := New(parent)
	if child.TraceID != parent.TraceID {
		t.Fatalf("child trace %s != parent trace %s", child.TraceID, parent.TraceID)
	}
	if child.ParentID != parent.SpanID {
		t.Fatalf("child parent %s != parent span %s", child.ParentID, parent.SpanID)
	}
}

func TestTraceparentRoundtrip(t *testing.T) {
	span := New(nil)
	parsed, ok := ParseTraceparent(span.Traceparent())
	if !ok {
		t.Fatal("roundtrip failed to parse")
	}
	if parsed.TraceID != span.TraceID || parsed.SpanID != span.SpanID {
		t.Fatalf("roundtrip mismatch: %+v vs %+v", parsed, span)
	}
}
