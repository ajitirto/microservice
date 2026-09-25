package userresolver_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync/atomic"
	"testing"
	"time"

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
	r, err := userresolver.New(userresolver.Config{
		Address:     lis.Addr().String(),
		Timeout:     200 * time.Millisecond,
		MaxAttempts: 3,
		BaseBackoff: 10 * time.Millisecond,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
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
