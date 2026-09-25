package userresolver_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"post/internal/userpb"
	"post/internal/userresolver"
)

type fakeUserServer struct {
	userpb.UnimplementedUserServiceServer
	unavailable atomic.Bool
	calls       atomic.Int64
}

func (s *fakeUserServer) GetUser(ctx context.Context, req *userpb.GetUserRequest) (*userpb.GetUserResponse, error) {
	s.calls.Add(1)
	if s.unavailable.Load() {
		return nil, status.Error(codes.Unavailable, "user service unavailable")
	}
	if req.GetUserId() != "123" {
		return nil, status.Error(codes.NotFound, "user not found")
	}
	return &userpb.GetUserResponse{Id: "123", Name: "Aji", Email: "aji@example.com"}, nil
}

func startFakeServer(t *testing.T, fake *fakeUserServer) net.Listener {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	gs := grpc.NewServer()
	userpb.RegisterUserServiceServer(gs, fake)
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)
	return lis
}

func mustResolver(t *testing.T, lis net.Listener) *userresolver.Resolver {
	t.Helper()
	return mustResolverWithCache(t, lis, nil)
}

func mustResolverWithCache(t *testing.T, lis net.Listener, cache userresolver.Cache) *userresolver.Resolver {
	t.Helper()
	r, err := userresolver.New(userresolver.Config{
		Address:     lis.Addr().String(),
		Timeout:     200 * time.Millisecond,
		MaxAttempts: 3,
		BaseBackoff: 10 * time.Millisecond,
		Cache:       cache,
		CacheTTL:    time.Minute,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}

type fakeCache struct {
	mu    sync.Mutex
	store map[string]fakeEntry
}

type fakeEntry struct {
	value string
	exp   time.Time
}

func (c *fakeCache) Get(ctx context.Context, key string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.store[key]
	if !ok || time.Now().After(e.exp) {
		return "", redis.Nil
	}
	return e.value, nil
}

func (c *fakeCache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = fakeEntry{value: value, exp: time.Now().Add(ttl)}
	return nil
}

func TestResolverCacheHitSkipsRPC(t *testing.T) {
	fake := &fakeUserServer{}
	lis := startFakeServer(t, fake)
	cache := &fakeCache{store: map[string]fakeEntry{"user:123": {value: `{"ID":"123","Name":"Aji","Email":"aji@example.com"}`, exp: time.Now().Add(time.Minute)}}}
	r := mustResolverWithCache(t, lis, cache)

	u, err := r.GetUser(context.Background(), "123")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if u.Name != "Aji" {
		t.Errorf("name = %q, want Aji", u.Name)
	}
	if fake.calls.Load() != 0 {
		t.Errorf("rpc calls = %d, want 0 (served from cache)", fake.calls.Load())
	}
}

func TestResolverStoresAfterFetch(t *testing.T) {
	fake := &fakeUserServer{}
	lis := startFakeServer(t, fake)
	cache := &fakeCache{store: map[string]fakeEntry{}}
	r := mustResolverWithCache(t, lis, cache)

	if _, err := r.GetUser(context.Background(), "123"); err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if fake.calls.Load() != 1 {
		t.Fatalf("rpc calls = %d, want 1", fake.calls.Load())
	}
	if _, ok := cache.store["user:123"]; !ok {
		t.Error("resolved user should be cached")
	}

	if _, err := r.GetUser(context.Background(), "123"); err != nil {
		t.Fatalf("GetUser (cached): %v", err)
	}
	if fake.calls.Load() != 1 {
		t.Errorf("rpc calls = %d, want still 1 (second call cached)", fake.calls.Load())
	}
}

func TestResolverCacheTimeoutDoesNotDelayFetch(t *testing.T) {
	fake := &fakeUserServer{}
	lis := startFakeServer(t, fake)
	r := mustResolverWithCache(t, lis, blockingCache{})

	start := time.Now()
	u, err := r.GetUser(context.Background(), "123")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if u.Name != "Aji" {
		t.Errorf("name = %q, want Aji", u.Name)
	}
	if elapsed > 600*time.Millisecond {
		t.Errorf("elapsed = %v, want < 600ms (cache op must time out)", elapsed)
	}
}

type blockingCache struct{}

func (blockingCache) Get(ctx context.Context, key string) (string, error) {
	<-ctx.Done()
	return "", ctx.Err()
}

func (blockingCache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	<-ctx.Done()
	return ctx.Err()
}

func TestResolverCacheCooldownSkipsFailingCache(t *testing.T) {
	fake := &fakeUserServer{}
	lis := startFakeServer(t, fake)
	cache := &failingCache{}
	r := mustResolverWithCache(t, lis, cache)

	for i := 0; i < 50; i++ {
		if _, err := r.GetUser(context.Background(), "123"); err != nil {
			t.Fatalf("GetUser %d: %v", i, err)
		}
	}
	if cache.gets > 4 {
		t.Errorf("cache gets = %d, want <= 4 (cooldown must bound retries)", cache.gets)
	}
}

type failingCache struct {
	gets int
}

func (c *failingCache) Get(ctx context.Context, key string) (string, error) {
	c.gets++
	return "", errors.New("connection refused")
}

func (c *failingCache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return errors.New("connection refused")
}

func TestResolverExpiredCacheRefetches(t *testing.T) {
	fake := &fakeUserServer{}
	lis := startFakeServer(t, fake)
	cache := &fakeCache{store: map[string]fakeEntry{"user:123": {value: `{"ID":"123","Name":"Aji","Email":"aji@example.com"}`, exp: time.Now().Add(-time.Second)}}}
	r := mustResolverWithCache(t, lis, cache)

	if _, err := r.GetUser(context.Background(), "123"); err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if fake.calls.Load() != 1 {
		t.Errorf("rpc calls = %d, want 1 (expired entry refetched)", fake.calls.Load())
	}
}

func TestResolverGetUser(t *testing.T) {
	fake := &fakeUserServer{}
	lis := startFakeServer(t, fake)
	r := mustResolver(t, lis)

	u, err := r.GetUser(context.Background(), "123")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if u.ID != "123" || u.Name != "Aji" || u.Email != "aji@example.com" {
		t.Errorf("user = %+v, want seed user", u)
	}
}

func TestResolverUnknownUserNoRetry(t *testing.T) {
	fake := &fakeUserServer{}
	lis := startFakeServer(t, fake)
	r := mustResolver(t, lis)

	_, err := r.GetUser(context.Background(), "xyz")
	if !errors.Is(err, userresolver.ErrUserNotFound) {
		t.Fatalf("err = %v, want ErrUserNotFound", err)
	}
	if fake.calls.Load() != 1 {
		t.Errorf("calls = %d, want 1 (no retry on NotFound)", fake.calls.Load())
	}
}

func TestResolverRetriesOnUnavailable(t *testing.T) {
	fake := &fakeUserServer{}
	fake.unavailable.Store(true)
	lis := startFakeServer(t, fake)
	r := mustResolver(t, lis)

	_, err := r.GetUser(context.Background(), "123")
	if !errors.Is(err, userresolver.ErrUserUnavailable) {
		t.Fatalf("err = %v, want ErrUserUnavailable", err)
	}
	if fake.calls.Load() != 3 {
		t.Errorf("calls = %d, want 3 (max attempts)", fake.calls.Load())
	}
}

func TestResolverRecoversAfterTransientFailure(t *testing.T) {
	fake := &fakeUserServer{}
	lis := startFakeServer(t, fake)
	r := mustResolver(t, lis)

	fake.unavailable.Store(true)
	if _, err := r.GetUser(context.Background(), "123"); !errors.Is(err, userresolver.ErrUserUnavailable) {
		t.Fatalf("err = %v, want ErrUserUnavailable", err)
	}

	fake.unavailable.Store(false)
	u, err := r.GetUser(context.Background(), "123")
	if err != nil {
		t.Fatalf("GetUser after recovery: %v", err)
	}
	if u.Name != "Aji" {
		t.Errorf("name = %q, want Aji", u.Name)
	}
}
