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

	"github.com/sony/gobreaker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"gateway/internal/authpb"
)

const (
	// breakerMaxFailures opens the circuit after this many consecutive
	// failures; breakerCooldown is how long it stays open before a
	// half-open probe is allowed through.
	breakerMaxFailures = 5
	breakerCooldown    = 10 * time.Second
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

	breaker *gobreaker.CircuitBreaker
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
		breaker: gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:        "auth-verifier",
			MaxRequests: 1,
			Timeout:     breakerCooldown,
			ReadyToTrip: func(counts gobreaker.Counts) bool {
				return counts.ConsecutiveFailures >= breakerMaxFailures
			},
			OnStateChange: func(name string, from, to gobreaker.State) {
				cfg.Logger.Warn("auth resolver circuit state change",
					"from", from.String(), "to", to.String())
			},
		}),
	}, nil
}

func (r *Resolver) Close() error {
	return r.conn.Close()
}

// Verify routes through the circuit breaker so a failing auth service
// stops being hammered once the circuit opens; open-circuit calls fail
// fast with ErrAuthUnavailable. Only transport-level failures trip the
// breaker; an invalid-token verdict is a valid business outcome.
func (r *Resolver) Verify(ctx context.Context, token string) (string, error) {
	v, err := r.breaker.Execute(func() (any, error) {
		userID, err := r.verifyWithRetries(ctx, token)
		if err != nil && !errors.Is(err, ErrInvalidToken) {
			return verifyResult{}, err
		}
		return verifyResult{userID: userID, err: err}, nil
	})
	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) {
			r.logger.Warn("auth resolver circuit open; failing fast")
			return "", ErrAuthUnavailable
		}
		return "", err
	}
	res := v.(verifyResult)
	return res.userID, res.err
}

type verifyResult struct {
	userID string
	err    error
}

func (r *Resolver) verifyWithRetries(ctx context.Context, token string) (string, error) {
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
