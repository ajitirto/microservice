// Package grpc exposes the user service internal API over gRPC. Other
// services (post, auth, ...) resolve user data through this contract
// instead of querying the user database directly.
package grpc

import (
	"context"
	"errors"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"user/internal/repository"
	"user/internal/userpb"
)

type Server struct {
	userpb.UnimplementedUserServiceServer
	repo   repository.UserRepository
	logger *slog.Logger
}

func NewServer(repo repository.UserRepository, logger *slog.Logger) *Server {
	return &Server{repo: repo, logger: logger}
}

func (s *Server) GetUser(ctx context.Context, req *userpb.GetUserRequest) (*userpb.GetUserResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id required")
	}

	u, err := s.repo.GetByID(ctx, req.GetUserId())
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		s.logger.Error("failed to get user", "error", err, "user_id", req.GetUserId())
		return nil, status.Error(codes.Internal, "failed to get user")
	}

	return &userpb.GetUserResponse{
		Id:    u.ID,
		Name:  u.Name,
		Email: u.Email,
	}, nil
}
