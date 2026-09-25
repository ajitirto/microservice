package repository_test

import (
	"context"
	"errors"
	"testing"

	"user/internal/repository"
)

func ctx() context.Context { return context.Background() }

func TestGetByIDFindsSeededUser(t *testing.T) {
	repo := repository.NewInMemory()

	u, err := repo.GetByID(ctx(), "123")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if u.ID != "123" || u.Name != "Aji" || u.Email != "aji@example.com" {
		t.Errorf("got %+v, want seeded demo user", u)
	}
	if u.CreatedAt.IsZero() || u.UpdatedAt.IsZero() {
		t.Errorf("timestamps must be set, got %+v", u)
	}
}

func TestGetByIDUnknownReturnsErrNotFound(t *testing.T) {
	repo := repository.NewInMemory()

	if _, err := repo.GetByID(ctx(), "nope"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateExistingUser(t *testing.T) {
	repo := repository.NewInMemory()

	before, err := repo.GetByID(ctx(), "123")
	if err != nil {
		t.Fatal(err)
	}
	created := before.CreatedAt

	updated, err := repo.Update(ctx(), "123", before)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !updated.CreatedAt.Equal(created) {
		t.Errorf("CreatedAt changed: got %v want %v", updated.CreatedAt, created)
	}
	if updated.UpdatedAt.Before(created) {
		t.Errorf("UpdatedAt (%v) should not be before CreatedAt (%v)", updated.UpdatedAt, created)
	}
}

func TestUpdateUnknownReturnsErrNotFound(t *testing.T) {
	repo := repository.NewInMemory()

	if _, err := repo.Update(ctx(), "nope", nil); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestDeleteExistingUser(t *testing.T) {
	repo := repository.NewInMemory()

	if err := repo.Delete(ctx(), "123"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.GetByID(ctx(), "123"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("after delete, GetByID err = %v, want ErrNotFound", err)
	}
}

func TestDeleteUnknownReturnsErrNotFound(t *testing.T) {
	repo := repository.NewInMemory()

	if err := repo.Delete(ctx(), "nope"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}
