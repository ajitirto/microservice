// Package grpc exposes the auth service internal API over gRPC. The
// gateway uses VerifyToken to validate access tokens without ever
// holding the signing secret.
package grpc

import (
	"context"
	"errors"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"auth/internal/authpb"
	"auth/internal/service"
)

type Server struct {
	authpb.UnimplementedAuthServiceServer
	svc    *service.Service
	logger *slog.Logger
}

func NewServer(svc *service.Service, logger *slog.Logger) *Server {
	return &Server{svc: svc, logger: logger}
}

func (s *Server) VerifyToken(ctx context.Context, req *authpb.VerifyTokenRequest) (*authpb.VerifyTokenResponse, error) {
	if req.GetToken() == "" {
		return nil, status.Error(codes.Unauthenticated, "token required")
	}

	result, err := s.svc.Verify(ctx, req.GetToken())
	if err != nil {
		if errors.Is(err, service.ErrInvalidToken) {
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}
		s.logger.Error("failed to verify token", "error", err)
		return nil, status.Error(codes.Internal, "failed to verify token")
	}

	return &authpb.VerifyTokenResponse{
		UserId: result.UserID,
		Email:  result.Email,
	}, nil
}
