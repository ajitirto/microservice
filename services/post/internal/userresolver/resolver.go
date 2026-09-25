// Package userresolver implements the gRPC client the post service
// uses to resolve user data from the user service. Calls are bounded by
// a timeout and retried with exponential backoff on transient failures.
package userresolver

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"post/internal/model"
	"post/internal/userpb"
)

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrUserUnavailable = errors.New("user service unavailable")
)

// Config pins the retry policy. MaxAttempts is the total number of
// attempts (including the first), so 3 means up to 2 retries.
type Config struct {
	Address     string
	Timeout     time.Duration
	MaxAttempts int
	BaseBackoff time.Duration
	Logger      *slog.Logger
}

type Resolver struct {
	client      userpb.UserServiceClient
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
		cfg.Timeout = time.Second
	}
	if cfg.MaxAttempts < 1 {
		cfg.MaxAttempts = 3
	}
	if cfg.BaseBackoff == 0 {
		cfg.BaseBackoff = 50 * time.Millisecond
	}

	conn, err := grpc.NewClient(cfg.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &Resolver{
		client:      userpb.NewUserServiceClient(conn),
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

func (r *Resolver) GetUser(ctx context.Context, userID string) (*model.UserInfo, error) {
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
