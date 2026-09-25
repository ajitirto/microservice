package grpc_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	usergrpc "user/internal/grpc"
	"user/internal/repository"
	"user/internal/userpb"
)

func newServer() *usergrpc.Server {
	return usergrpc.NewServer(repository.NewInMemory(), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestGetUser(t *testing.T) {
	srv := newServer()
	ctx := context.Background()

	t.Run("existing user", func(t *testing.T) {
		resp, err := srv.GetUser(ctx, &userpb.GetUserRequest{UserId: "123"})
		if err != nil {
			t.Fatalf("GetUser: %v", err)
		}
		if resp.Id != "123" || resp.Name != "Aji" || resp.Email != "aji@example.com" {
			t.Errorf("resp = %+v, want seed user 123", resp)
		}
	})

	t.Run("unknown user maps to NotFound", func(t *testing.T) {
		_, err := srv.GetUser(ctx, &userpb.GetUserRequest{UserId: "nope"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("code = %v, want NotFound", status.Code(err))
		}
	})

	t.Run("empty id maps to InvalidArgument", func(t *testing.T) {
		_, err := srv.GetUser(ctx, &userpb.GetUserRequest{UserId: ""})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("code = %v, want InvalidArgument", status.Code(err))
		}
	})
}
