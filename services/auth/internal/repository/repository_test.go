package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"auth/internal/repository"
)

func TestSeedUserExists(t *testing.T) {
	repo := repository.NewInMemory()

	cred, err := repo.Users.GetByEmail(context.Background(), "aji@example.com")
	if err != nil {
		t.Fatalf("GetByEmail seed: %v", err)
	}
	if cred.ID != "123" {
		t.Errorf("id = %q, want %q", cred.ID, "123")
	}
	if cred.PasswordHash == "" || cred.Salt == "" {
		t.Errorf("seed must have salt and hash, got %+v", cred)
	}
}

func TestCreateAndGetByEmail(t *testing.T) {
	repo := repository.NewInMemory()

	cred := &repository.Credential{
		ID:           "abc",
		Email:        "new@example.com",
		Salt:         "s",
		PasswordHash: "h",
		CreatedAt:    time.Now().UTC(),
	}
	if err := repo.Users.Create(context.Background(), cred); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.Users.GetByEmail(context.Background(), "new@example.com")
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if got.ID != "abc" {
		t.Errorf("id = %q, want %q", got.ID, "abc")
	}
}

func TestCreateDuplicateEmail(t *testing.T) {
	repo := repository.NewInMemory()

	cred := &repository.Credential{ID: "x", Email: "aji@example.com"}
	if err := repo.Users.Create(context.Background(), cred); !errors.Is(err, repository.ErrDuplicate) {
		t.Errorf("err = %v, want ErrDuplicate", err)
	}
}

func TestGetUnknownReturnsErrNotFound(t *testing.T) {
	repo := repository.NewInMemory()

	if _, err := repo.Users.GetByEmail(context.Background(), "nope@example.com"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	if _, err := repo.Users.GetByID(context.Background(), "nope"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestRefreshTokenStoreLifecycle(t *testing.T) {
	repo := repository.NewInMemory()
	ctx := context.Background()

	if _, _, ok := repo.Refresh.Get(ctx, "h1"); ok {
		t.Fatal("empty store should not find h1")
	}

	expiry := time.Now().Add(time.Hour)
	if err := repo.Refresh.Store(ctx, "h1", "123", expiry); err != nil {
		t.Fatalf("Store: %v", err)
	}

	uid, exp, ok := repo.Refresh.Get(ctx, "h1")
	if !ok || uid != "123" || !exp.Equal(expiry) {
		t.Errorf("got (%q, %v, %v), want (123, exp, true)", uid, exp, ok)
	}

	repo.Refresh.Delete(ctx, "h1")
	if _, _, ok := repo.Refresh.Get(ctx, "h1"); ok {
		t.Error("token should be gone after Delete")
	}
}
