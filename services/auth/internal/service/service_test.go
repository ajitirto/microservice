package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"auth/internal/model"
	"auth/internal/repository"
)

func newService() *Service {
	repo := repository.NewInMemory()
	return New(repo.Users, repo.Refresh, "test-secret", 15*time.Minute, 24*time.Hour)
}

func mustSign(t *testing.T, s *Service, uid string) string {
	t.Helper()
	token, err := s.signToken(uid, tokenTypeAccess, time.Minute)
	if err != nil {
		t.Fatalf("signToken: %v", err)
	}
	return token
}

func TestRegisterAndLogin(t *testing.T) {
	svc := newService()
	ctx := context.Background()

	pair, err := svc.Register(ctx, model.RegisterInput{Email: "budi@example.com", Password: "secret123"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if pair.UserID == "" || pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatalf("token pair incomplete: %+v", pair)
	}
	if pair.TokenType != "Bearer" {
		t.Errorf("token type = %q, want Bearer", pair.TokenType)
	}

	login, err := svc.Login(ctx, model.LoginInput{Email: "budi@example.com", Password: "secret123"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if login.UserID != pair.UserID {
		t.Errorf("login user = %q, want %q", login.UserID, pair.UserID)
	}

	result, err := svc.Verify(ctx, login.AccessToken)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.UserID != pair.UserID || result.Email != "budi@example.com" {
		t.Errorf("verify result = %+v", result)
	}
}

func TestSeedLoginWithPassword123(t *testing.T) {
	svc := newService()

	pair, err := svc.Login(context.Background(), model.LoginInput{Email: "aji@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("seed login failed: %v", err)
	}
	if pair.UserID != "123" {
		t.Errorf("user id = %q, want 123", pair.UserID)
	}
}

func TestRegisterValidation(t *testing.T) {
	svc := newService()
	ctx := context.Background()

	tests := []struct {
		name string
		in   model.RegisterInput
	}{
		{name: "invalid email", in: model.RegisterInput{Email: "nope", Password: "secret123"}},
		{name: "short password", in: model.RegisterInput{Email: "x@example.com", Password: "short"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := svc.Register(ctx, tt.in); !errors.Is(err, ErrInvalidInput) {
				t.Errorf("err = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	svc := newService()
	ctx := context.Background()

	_, err := svc.Register(ctx, model.RegisterInput{Email: "aji@example.com", Password: "whatever1"})
	if !errors.Is(err, ErrUserAlreadyExists) {
		t.Errorf("err = %v, want ErrUserAlreadyExists", err)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	svc := newService()

	_, err := svc.Login(context.Background(), model.LoginInput{Email: "aji@example.com", Password: "wrong-password"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestLoginUnknownEmail(t *testing.T) {
	svc := newService()

	_, err := svc.Login(context.Background(), model.LoginInput{Email: "ghost@example.com", Password: "whatever1"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestRefreshRotatesTokens(t *testing.T) {
	svc := newService()
	ctx := context.Background()

	first, err := svc.Login(ctx, model.LoginInput{Email: "aji@example.com", Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}

	second, err := svc.Refresh(ctx, first.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if second.AccessToken == first.AccessToken {
		t.Error("access token should rotate")
	}
	if second.RefreshToken == first.RefreshToken {
		t.Error("refresh token should rotate")
	}

	if _, err := svc.Refresh(ctx, first.RefreshToken); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("reusing old refresh token: err = %v, want ErrInvalidToken", err)
	}
}

func TestRefreshUnknownToken(t *testing.T) {
	svc := newService()

	if _, err := svc.Refresh(context.Background(), "not-a-real-token"); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("err = %v, want ErrInvalidToken", err)
	}
}

func TestLogoutRevokesRefreshToken(t *testing.T) {
	svc := newService()
	ctx := context.Background()

	pair, err := svc.Login(ctx, model.LoginInput{Email: "aji@example.com", Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.Logout(ctx, pair.RefreshToken); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, err := svc.Refresh(ctx, pair.RefreshToken); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("refresh after logout: err = %v, want ErrInvalidToken", err)
	}
}

func TestVerifyRejectsBadTokens(t *testing.T) {
	svc := newService()
	ctx := context.Background()

	good, err := svc.Login(ctx, model.LoginInput{Email: "aji@example.com", Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		token string
	}{
		{name: "tampered", token: good.AccessToken + "x"},
		{name: "refresh used as access", token: good.RefreshToken},
		{name: "garbage", token: "abc.def.ghi"},
		{name: "empty", token: ""},
		{name: "unknown user", token: mustSign(t, svc, "deadbeef")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := svc.Verify(ctx, tt.token); !errors.Is(err, ErrInvalidToken) {
				t.Errorf("err = %v, want ErrInvalidToken", err)
			}
		})
	}
}

func TestVerifyRejectsExpiredToken(t *testing.T) {
	svc := newService()
	ctx := context.Background()

	base := time.Now()
	svc.now = func() time.Time { return base }
	token := mustSign(t, svc, "123")

	svc.now = func() time.Time { return base.Add(2 * time.Minute) }
	if _, err := svc.Verify(ctx, token); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("err = %v, want ErrInvalidToken", err)
	}
}
