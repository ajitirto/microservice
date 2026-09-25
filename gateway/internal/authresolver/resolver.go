// Package authresolver implements the gRPC client the gateway uses to
// verify access tokens against the auth service. The gateway never
// holds the signing secret; verification is delegated over gRPC with a
// timeout and retries on transient failures.
package authresolver

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"gateway/internal/authpb"
)

var (
	ErrInvalidToken    = errors.New("invalid token")
	ErrAuthUnavailable = errors.New("auth service unavailable")
)

type Config struct {
	Address     string
	Timeout     time.Duration
	MaxAttempts int
	BaseBackoff time.Duration
	Logger      *slog.Logger
}

type Resolver struct {
	client      authpb.AuthServiceClient
	conn        *grpc.ClientConn
	timeout     time.Duration
	maxAttempts int
	baseBackoff time.Duration
	logger      *slog.Logger
}

func New(cfg Config) (*Resolver, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 500 * time.Millisecond
	}
	if cfg.MaxAttempts < 1 {
		cfg.MaxAttempts = 2
	}
	if cfg.BaseBackoff == 0 {
		cfg.BaseBackoff = 50 * time.Millisecond
	}

	conn, err := grpc.NewClient(cfg.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &Resolver{
		client:      authpb.NewAuthServiceClient(conn),
		conn:        conn,
		timeout:     cfg.Timeout,
		maxAttempts: cfg.MaxAttempts,
		baseBackoff: cfg.BaseBackoff,
		logger:      cfg.Logger,
	}, nil
}

func (r *Resolver) Close() error {
	return r.conn.Close()
}

func (r *Resolver) Verify(ctx context.Context, token string) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= r.maxAttempts; attempt++ {
		if attempt > 1 {
			r.logger.Warn("token verify retry",
				"attempt", attempt,
				"error", lastErr,
			)
		}

		callCtx, cancel := context.WithTimeout(ctx, r.timeout)
		resp, err := r.client.VerifyToken(callCtx, &authpb.VerifyTokenRequest{Token: token})
		cancel()

		if err == nil {
			return resp.GetUserId(), nil
		}

		switch status.Code(err) {
		case codes.Unauthenticated, codes.InvalidArgument:
			return "", ErrInvalidToken
		}

		lastErr = err
		if attempt < r.maxAttempts {
			select {
			case <-ctx.Done():
				return "", ErrAuthUnavailable
			case <-time.After(r.baseBackoff * time.Duration(1<<(attempt-1))):
			}
		}
	}
	return "", ErrAuthUnavailable
}
