package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"post/internal/model"
	"post/internal/repository"
)

func ctx() context.Context { return context.Background() }

func TestSeedPostsExist(t *testing.T) {
	repo := repository.NewInMemory()

	posts, err := repo.List(ctx())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("got %d seeded posts, want 2", len(posts))
	}
	if posts[0].ID != "p2" {
		t.Errorf("first post = %q, want p2 (newest first)", posts[0].ID)
	}
}

func TestCreateAndGetByID(t *testing.T) {
	repo := repository.NewInMemory()
	now := time.Now().UTC()

	post := &model.Post{
		ID:        "new1",
		Title:     "T",
		Content:   "C",
		AuthorID:  "456",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repo.Create(ctx(), post); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByID(ctx(), "new1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.AuthorID != "456" || got.Title != "T" {
		t.Errorf("got %+v", got)
	}
}

func TestGetByIDUnknownReturnsErrNotFound(t *testing.T) {
	repo := repository.NewInMemory()

	if _, err := repo.GetByID(ctx(), "nope"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdatePreservesAuthorAndCreated(t *testing.T) {
	repo := repository.NewInMemory()

	before, err := repo.GetByID(ctx(), "p1")
	if err != nil {
		t.Fatal(err)
	}

	updated := &model.Post{
		ID:        before.ID,
		Title:     "Updated",
		Content:   "Updated content",
		AuthorID:  before.AuthorID,
		CreatedAt: before.CreatedAt,
		UpdatedAt: time.Now().UTC(),
	}
	if err := repo.Update(ctx(), updated); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := repo.GetByID(ctx(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Updated" {
		t.Errorf("title = %q, want Updated", got.Title)
	}
	if got.AuthorID != "123" {
		t.Errorf("author = %q, want 123 preserved", got.AuthorID)
	}
	if !got.CreatedAt.Equal(before.CreatedAt) {
		t.Error("CreatedAt should be preserved")
	}
}

func TestDelete(t *testing.T) {
	repo := repository.NewInMemory()

	if err := repo.Delete(ctx(), "p1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.GetByID(ctx(), "p1"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("after delete, GetByID err = %v, want ErrNotFound", err)
	}
	if err := repo.Delete(ctx(), "p1"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("second delete err = %v, want ErrNotFound", err)
	}
}

func TestAddLikeIsIdempotent(t *testing.T) {
	repo := repository.NewInMemory()

	count, err := repo.AddLike(ctx(), "p1", "u1")
	if err != nil {
		t.Fatalf("AddLike: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}

	count, err = repo.AddLike(ctx(), "p1", "u1")
	if err != nil {
		t.Fatalf("AddLike again: %v", err)
	}
	if count != 1 {
		t.Errorf("repeat like count = %d, want 1 (idempotent)", count)
	}

	count, err = repo.AddLike(ctx(), "p1", "u2")
	if err != nil {
		t.Fatalf("AddLike third: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestAddLikeUnknownPost(t *testing.T) {
	repo := repository.NewInMemory()

	if _, err := repo.AddLike(ctx(), "nope", "u1"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestLikeCountShownOnRead(t *testing.T) {
	repo := repository.NewInMemory()

	if _, err := repo.AddLike(ctx(), "p2", "u1"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AddLike(ctx(), "p2", "u2"); err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetByID(ctx(), "p2")
	if err != nil {
		t.Fatal(err)
	}
	if got.Likes != 2 {
		t.Errorf("likes = %d, want 2", got.Likes)
	}
}
