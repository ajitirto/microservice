package authresolver_test

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

	"gateway/internal/authpb"
	"gateway/internal/authresolver"
)

type fakeAuthServer struct {
	authpb.UnimplementedAuthServiceServer
	unavailable atomic.Bool
	calls       atomic.Int64
}

func (s *fakeAuthServer) VerifyToken(ctx context.Context, req *authpb.VerifyTokenRequest) (*authpb.VerifyTokenResponse, error) {
	s.calls.Add(1)
	if s.unavailable.Load() {
		return nil, status.Error(codes.Unavailable, "auth service unavailable")
	}
	if req.GetToken() != "good" {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}
	return &authpb.VerifyTokenResponse{UserId: "123", Email: "aji@example.com"}, nil
}

func startFakeServer(t *testing.T, fake *fakeAuthServer) net.Listener {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	gs := grpc.NewServer()
	authpb.RegisterAuthServiceServer(gs, fake)
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)
	return lis
}

func mustResolver(t *testing.T, lis net.Listener) *authresolver.Resolver {
	t.Helper()
	r, err := authresolver.New(authresolver.Config{
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

func TestVerifyValidToken(t *testing.T) {
	fake := &fakeAuthServer{}
	lis := startFakeServer(t, fake)
	r := mustResolver(t, lis)

	userID, err := r.Verify(context.Background(), "good")
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if userID != "123" {
		t.Errorf("userID = %q, want 123", userID)
	}
}

func TestVerifyInvalidTokenNoRetry(t *testing.T) {
	fake := &fakeAuthServer{}
	lis := startFakeServer(t, fake)
	r := mustResolver(t, lis)

	_, err := r.Verify(context.Background(), "bad")
	if !errors.Is(err, authresolver.ErrInvalidToken) {
		t.Fatalf("err = %v, want ErrInvalidToken", err)
	}
	if fake.calls.Load() != 1 {
		t.Errorf("calls = %d, want 1 (no retry on Unauthenticated)", fake.calls.Load())
	}
}

func TestVerifyRetriesOnUnavailable(t *testing.T) {
	fake := &fakeAuthServer{}
	fake.unavailable.Store(true)
	lis := startFakeServer(t, fake)
	r := mustResolver(t, lis)

	_, err := r.Verify(context.Background(), "good")
	if !errors.Is(err, authresolver.ErrAuthUnavailable) {
		t.Fatalf("err = %v, want ErrAuthUnavailable", err)
	}
	if fake.calls.Load() != 3 {
		t.Errorf("calls = %d, want 3 (max attempts)", fake.calls.Load())
	}
}

func TestVerifyRecoversAfterTransientFailure(t *testing.T) {
	fake := &fakeAuthServer{}
	lis := startFakeServer(t, fake)
	r := mustResolver(t, lis)

	fake.unavailable.Store(true)
	if _, err := r.Verify(context.Background(), "good"); !errors.Is(err, authresolver.ErrAuthUnavailable) {
		t.Fatalf("err = %v, want ErrAuthUnavailable", err)
	}

	fake.unavailable.Store(false)
	userID, err := r.Verify(context.Background(), "good")
	if err != nil {
		t.Fatalf("Verify after recovery: %v", err)
	}
	if userID != "123" {
		t.Errorf("userID = %q, want 123", userID)
	}
}
