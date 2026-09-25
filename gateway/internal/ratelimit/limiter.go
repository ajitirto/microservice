// Package ratelimit implements a Redis-backed fixed window limiter
// for the gateway. Limits are enforced atomically with a single Lua
// script so concurrent requests cannot race past the counter.
package ratelimit

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

var incrementScript = redis.NewScript(`
local count = redis.call('INCR', KEYS[1])
if count == 1 then
  redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return count
`)

// Limiter reports whether the key may proceed.
type Limiter interface {
	Allow(ctx context.Context, key string) (bool, error)
}

// RedisLimiter allows up to limit hits per fixed window.
type RedisLimiter struct {
	client *redis.Client
	limit  uint64
	window time.Duration
}

func NewRedisLimiter(client *redis.Client, limit uint64, window time.Duration) *RedisLimiter {
	return &RedisLimiter{client: client, limit: limit, window: window}
}

func (l *RedisLimiter) Allow(ctx context.Context, key string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()
	ch := make(chan struct {
		n   int64
		err error
	}, 1)
	go func() {
		n, err := incrementScript.Run(ctx, l.client, []string{key}, int(l.window.Seconds())).Int64()
		ch <- struct {
			n   int64
			err error
		}{n, err}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			return false, r.err
		}
		return r.n <= int64(l.limit), nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

// NoopLimiter always allows; used when Redis is unavailable.
type NoopLimiter struct{}

func (NoopLimiter) Allow(context.Context, string) (bool, error) { return true, nil }
