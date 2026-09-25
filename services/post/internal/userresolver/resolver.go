// Package userresolver implements the gRPC client the post service
// uses to resolve user data from the user service. Calls are bounded by
// a timeout and retried with exponential backoff on transient failures.
// A cache can be attached to avoid repeated lookups of the same user.
package userresolver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sony/gobreaker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"post/internal/model"
	"post/internal/userpb"
)

const (
	defaultCacheTTL = 60 * time.Second
	cacheOpTimeout  = 250 * time.Millisecond
	cacheCooldown   = 2 * time.Second

	// breakerMaxFailures opens the circuit after this many consecutive
	// failures; breakerCooldown is how long it stays open before a
	// half-open probe is allowed through.
	breakerMaxFailures = 5
	breakerCooldown    = 10 * time.Second
)

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrUserUnavailable = errors.New("user service unavailable")
)

// Cache stores resolved users keyed by user id. Get returns the stored
// value; redis.Nil signals a miss.
type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
}

type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(redisURL string) (*RedisCache, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	opts.DialTimeout = 500 * time.Millisecond
	opts.ReadTimeout = 500 * time.Millisecond
	opts.WriteTimeout = 500 * time.Millisecond
	opts.PoolTimeout = 500 * time.Millisecond
	opts.MaxRetries = -1
	return &RedisCache{client: redis.NewClient(opts)}, nil
}

func (c *RedisCache) Close() error {
	return c.client.Close()
}

func (c *RedisCache) Get(ctx context.Context, key string) (string, error) {
	return c.client.Get(ctx, key).Result()
}

func (c *RedisCache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

// Config pins the retry policy. MaxAttempts is the total number of
// attempts (including the first), so 3 means up to 2 retries.
type Config struct {
	Address     string
	Timeout     time.Duration
	MaxAttempts int
	BaseBackoff time.Duration
	Cache       Cache
	CacheTTL    time.Duration
	Logger      *slog.Logger
}

type Resolver struct {
	client      userpb.UserServiceClient
	conn        *grpc.ClientConn
	timeout     time.Duration
	maxAttempts int
	baseBackoff time.Duration
	cache       Cache
	cacheTTL    time.Duration
	logger      *slog.Logger

	breaker           *gobreaker.CircuitBreaker
	disableCacheUntil atomic.Int64
}

func New(cfg Config) (*Resolver, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = time.Second
	}
	if cfg.MaxAttempts < 1 {
		cfg.MaxAttempts = 3
	}
	if cfg.BaseBackoff == 0 {
		cfg.BaseBackoff = 50 * time.Millisecond
	}
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = defaultCacheTTL
	}

	conn, err := grpc.NewClient(cfg.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`))
	if err != nil {
		return nil, err
	}
	return &Resolver{
		client:      userpb.NewUserServiceClient(conn),
		conn:        conn,
		timeout:     cfg.Timeout,
		maxAttempts: cfg.MaxAttempts,
		baseBackoff: cfg.BaseBackoff,
		cache:       cfg.Cache,
		cacheTTL:    cfg.CacheTTL,
		logger:      cfg.Logger,
		breaker: gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:        "user-resolver",
			MaxRequests: 1,
			Timeout:     breakerCooldown,
			ReadyToTrip: func(counts gobreaker.Counts) bool {
				return counts.ConsecutiveFailures >= breakerMaxFailures
			},
			OnStateChange: func(name string, from, to gobreaker.State) {
				cfg.Logger.Warn("user resolver circuit state change",
					"from", from.String(), "to", to.String())
			},
		}),
	}, nil
}

func (r *Resolver) Close() error {
	return r.conn.Close()
}

// AttachCache wires a cache into the resolver; safe before first use.
func (r *Resolver) AttachCache(cache Cache) {
	r.cache = cache
}

func (r *Resolver) GetUser(ctx context.Context, userID string) (*model.UserInfo, error) {
	if info, ok := r.fromCache(ctx, userID); ok {
		return info, nil
	}

	info, err := r.fetch(ctx, userID)
	if err != nil {
		return nil, err
	}

	r.store(ctx, userID, info)
	return info, nil
}

// fetch routes through the circuit breaker so a failing user service
// stops being hammered once the circuit opens; open-circuit calls fail
// fast with ErrUserUnavailable. Only transport-level failures trip the
// breaker; a NotFound response is a valid business outcome.
func (r *Resolver) fetch(ctx context.Context, userID string) (*model.UserInfo, error) {
	v, err := r.breaker.Execute(func() (any, error) {
		info, err := r.fetchWithRetries(ctx, userID)
		if err != nil && !errors.Is(err, ErrUserNotFound) {
			return userLookupResult{}, err
		}
		return userLookupResult{info: info, err: err}, nil
	})
	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) {
			r.logger.Warn("user resolver circuit open; failing fast", "user_id", userID)
			return nil, ErrUserUnavailable
		}
		return nil, err
	}
	res := v.(userLookupResult)
	return res.info, res.err
}

type userLookupResult struct {
	info *model.UserInfo
	err  error
}

func (r *Resolver) fetchWithRetries(ctx context.Context, userID string) (*model.UserInfo, error) {
	var lastErr error
	for attempt := 1; attempt <= r.maxAttempts; attempt++ {
		if attempt > 1 {
			r.logger.Warn("user lookup retry",
				"user_id", userID,
				"attempt", attempt,
				"error", lastErr,
			)
		}

		callCtx, cancel := context.WithTimeout(ctx, r.timeout)
		resp, err := r.client.GetUser(callCtx, &userpb.GetUserRequest{UserId: userID})
		cancel()

		if err == nil {
			return &model.UserInfo{
				ID:    resp.GetId(),
				Name:  resp.GetName(),
				Email: resp.GetEmail(),
			}, nil
		}

		switch status.Code(err) {
		case codes.NotFound:
			return nil, ErrUserNotFound
		case codes.InvalidArgument:
			return nil, ErrUserNotFound
		}

		lastErr = err
		if attempt < r.maxAttempts {
			select {
			case <-ctx.Done():
				return nil, ErrUserUnavailable
			case <-time.After(r.baseBackoff * time.Duration(1<<(attempt-1))):
			}
		}
	}
	return nil, ErrUserUnavailable
}

func (r *Resolver) fromCache(ctx context.Context, userID string) (*model.UserInfo, bool) {
	if r.cache == nil {
		return nil, false
	}
	now := time.Now()
	if now.UnixNano() < r.disableCacheUntil.Load() {
		return nil, false
	}
	cacheCtx, cancel := context.WithTimeout(ctx, cacheOpTimeout)
	defer cancel()
	raw, err := boundedGet(r.cache, cacheCtx, keyFor(userID))
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			r.disableCache(now)
		}
		return nil, false
	}
	var info model.UserInfo
	if err := json.Unmarshal([]byte(raw), &info); err != nil {
		return nil, false
	}
	return &info, true
}

func (r *Resolver) store(ctx context.Context, userID string, info *model.UserInfo) {
	if r.cache == nil {
		return
	}
	if time.Now().UnixNano() < r.disableCacheUntil.Load() {
		return
	}
	raw, err := json.Marshal(info)
	if err != nil {
		return
	}
	cacheCtx, cancel := context.WithTimeout(ctx, cacheOpTimeout)
	defer cancel()
	if err := boundedSet(r.cache, cacheCtx, keyFor(userID), string(raw), r.cacheTTL); err != nil {
		r.disableCache(time.Now())
		r.logger.Warn("failed to cache user", "user_id", userID, "error", err)
	}
}

// boundedGet and boundedSet cap cache operations with a select so a
// hung DNS lookup can never stall the request past cacheOpTimeout.
func boundedGet(c Cache, ctx context.Context, key string) (string, error) {
	if c == nil {
		return "", redis.Nil
	}
	ch := make(chan struct {
		v   string
		err error
	}, 1)
	go func() {
		v, err := c.Get(ctx, key)
		ch <- struct {
			v   string
			err error
		}{v, err}
	}()
	select {
	case r := <-ch:
		return r.v, r.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func boundedSet(c Cache, ctx context.Context, key, value string, ttl time.Duration) error {
	if c == nil {
		return nil
	}
	ch := make(chan error, 1)
	go func() { ch <- c.Set(ctx, key, value, ttl) }()
	select {
	case err := <-ch:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Resolver) disableCache(now time.Time) {
	r.disableCacheUntil.Store(now.Add(cacheCooldown).UnixNano())
}

func keyFor(userID string) string {
	return "user:" + userID
}
