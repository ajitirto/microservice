package consumer

import (
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestShouldRetry(t *testing.T) {
	cases := []struct {
		count, max int
		want       bool
	}{
		{0, 3, true},
		{1, 3, true},
		{2, 3, true},
		{3, 3, false},
		{4, 3, false},
	}
	for _, c := range cases {
		if got := ShouldRetry(c.count, c.max); got != c.want {
			t.Errorf("ShouldRetry(%d, %d) = %v, want %v", c.count, c.max, got, c.want)
		}
	}
}

func TestRetryCountFromHeaders(t *testing.T) {
	cases := []struct {
		name    string
		headers amqp.Table
		want    int
	}{
		{"missing", nil, 0},
		{"int32", amqp.Table{"x-retry-count": int32(2)}, 2},
		{"int64", amqp.Table{"x-retry-count": int64(3)}, 3},
		{"int", amqp.Table{"x-retry-count": 4}, 4},
		{"float64", amqp.Table{"x-retry-count": float64(5)}, 5},
		{"other type ignored", amqp.Table{"x-retry-count": "nope"}, 0},
	}
	for _, c := range cases {
		if got := retryCount(c.headers); got != c.want {
			t.Errorf("%s: retryCount = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestRepublishIncrementsRetryHeader(t *testing.T) {
	headers := amqp.Table{"x-retry-count": int32(1)}
	want := retryCount(headers) + 1
	if want != 2 {
		t.Errorf("incremented retry count = %d, want 2", want)
	}
}
