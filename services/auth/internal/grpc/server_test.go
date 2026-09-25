package grpc_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"auth/internal/authpb"
	authgrpc "auth/internal/grpc"
	"auth/internal/model"
	"auth/internal/repository"
	"auth/internal/service"
)

func newFixture(t *testing.T) (*authgrpc.Server, *service.Service) {
	t.Helper()
	repo := repository.NewInMemory()
	svc := service.New(repo.Users, repo.Refresh, "test-secret", 15*time.Minute, 24*time.Hour)
	return authgrpc.NewServer(svc, slog.New(slog.NewTextHandler(io.Discard, nil))), svc
}

func validToken(t *testing.T, svc *service.Service) string {
	t.Helper()
	pair, err := svc.Login(context.Background(), model.LoginInput{Email: "aji@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	return pair.AccessToken
}

func TestVerifyToken(t *testing.T) {
	srv, svc := newFixture(t)
	ctx := context.Background()

	t.Run("valid token", func(t *testing.T) {
		resp, err := srv.VerifyToken(ctx, &authpb.VerifyTokenRequest{Token: validToken(t, svc)})
		if err != nil {
			t.Fatalf("VerifyToken: %v", err)
		}
		if resp.UserId != "123" || resp.Email != "aji@example.com" {
			t.Errorf("resp = %+v, want seed user 123", resp)
		}
	})

	t.Run("invalid token maps to Unauthenticated", func(t *testing.T) {
		_, err := srv.VerifyToken(ctx, &authpb.VerifyTokenRequest{Token: "garbage"})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("code = %v, want Unauthenticated", status.Code(err))
		}
	})

	t.Run("empty token maps to Unauthenticated", func(t *testing.T) {
		_, err := srv.VerifyToken(ctx, &authpb.VerifyTokenRequest{Token: ""})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("code = %v, want Unauthenticated", status.Code(err))
		}
	})
}
