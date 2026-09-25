package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestClient(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func TestRedisLimiterAllowsUpToLimit(t *testing.T) {
	client := newTestClient(t)
	l := NewRedisLimiter(client, 2, time.Minute)

	for i := 0; i < 2; i++ {
		ok, err := l.Allow(context.Background(), "client")
		if err != nil {
			t.Fatalf("Allow %d: %v", i, err)
		}
		if !ok {
			t.Errorf("Allow %d: want true", i)
		}
	}
	ok, err := l.Allow(context.Background(), "client")
	if err != nil {
		t.Fatalf("Allow: %v", err)
	}
	if ok {
		t.Error("Allow: want false after limit")
	}
}

func TestRedisLimiterKeysAreIsolated(t *testing.T) {
	client := newTestClient(t)
	l := NewRedisLimiter(client, 1, time.Minute)

	if ok, _ := l.Allow(context.Background(), "a"); !ok {
		t.Error("a first: want true")
	}
	if ok, _ := l.Allow(context.Background(), "b"); !ok {
		t.Error("b first: want true (isolated key)")
	}
	if ok, _ := l.Allow(context.Background(), "a"); ok {
		t.Error("a second: want false (own counter)")
	}
}

func TestRedisLimiterWindowExpires(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	l := NewRedisLimiter(client, 1, time.Minute)

	if ok, _ := l.Allow(context.Background(), "k"); !ok {
		t.Fatal("first: want true")
	}
	mr.FastForward(61 * time.Second)
	ok, err := l.Allow(context.Background(), "k")
	if err != nil {
		t.Fatalf("Allow: %v", err)
	}
	if !ok {
		t.Error("after window: want true (counter reset)")
	}
}

func TestNoopLimiterAlwaysAllows(t *testing.T) {
	ok, err := NoopLimiter{}.Allow(context.Background(), "k")
	if err != nil || !ok {
		t.Errorf("Noop: ok=%v err=%v, want true nil", ok, err)
	}
}
