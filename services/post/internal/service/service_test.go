package service_test

import (
	"context"
	"errors"
	"testing"

	"post/internal/model"
	"post/internal/repository"
	"post/internal/service"
)

func newService() *service.Service {
	return service.New(repository.NewInMemory())
}

func strPtr(s string) *string { return &s }

func TestCreateValidPost(t *testing.T) {
	svc := newService()

	post, err := svc.Create(context.Background(), "123", model.CreatePostInput{Title: "Hello", Content: "World"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if post.ID == "" {
		t.Error("post should have generated id")
	}
	if post.AuthorID != "123" {
		t.Errorf("author = %q, want 123", post.AuthorID)
	}
	if post.CreatedAt.IsZero() || post.UpdatedAt.IsZero() {
		t.Error("timestamps must be set")
	}
}

func TestCreateValidation(t *testing.T) {
	svc := newService()
	ctx := context.Background()

	tests := []struct {
		name     string
		authorID string
		in       model.CreatePostInput
	}{
		{name: "no author", authorID: "", in: model.CreatePostInput{Title: "T", Content: "C"}},
		{name: "empty title", in: model.CreatePostInput{Title: "  ", Content: "C"}},
		{name: "empty content", in: model.CreatePostInput{Title: "T", Content: ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := svc.Create(ctx, tt.authorID, tt.in); !errors.Is(err, service.ErrInvalidInput) {
				t.Errorf("err = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestGetByIDAndList(t *testing.T) {
	svc := newService()
	ctx := context.Background()

	post, err := svc.GetByID(ctx, "p1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if post.Title == "" {
		t.Error("seeded post should have title")
	}

	if _, err := svc.GetByID(ctx, "nope"); !errors.Is(err, service.ErrPostNotFound) {
		t.Errorf("err = %v, want ErrPostNotFound", err)
	}

	posts, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(posts) != 2 {
		t.Errorf("got %d posts, want 2", len(posts))
	}
}

func TestUpdateOwnership(t *testing.T) {
	svc := newService()
	ctx := context.Background()

	t.Run("author can update", func(t *testing.T) {
		post, err := svc.Update(ctx, "p1", "123", model.UpdatePostInput{Title: strPtr("New Title")})
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if post.Title != "New Title" {
			t.Errorf("title = %q, want New Title", post.Title)
		}
		if post.Content != "First post on the POC. Gateway, user, and auth services are up." {
			t.Error("content should be untouched")
		}
	})

	t.Run("non-author forbidden", func(t *testing.T) {
		_, err := svc.Update(ctx, "p1", "999", model.UpdatePostInput{Title: strPtr("Hijack")})
		if !errors.Is(err, service.ErrForbidden) {
			t.Errorf("err = %v, want ErrForbidden", err)
		}
	})

	t.Run("no fields invalid", func(t *testing.T) {
		_, err := svc.Update(ctx, "p1", "123", model.UpdatePostInput{})
		if !errors.Is(err, service.ErrInvalidInput) {
			t.Errorf("err = %v, want ErrInvalidInput", err)
		}
	})

	t.Run("unknown post", func(t *testing.T) {
		_, err := svc.Update(ctx, "nope", "123", model.UpdatePostInput{Title: strPtr("X")})
		if !errors.Is(err, service.ErrPostNotFound) {
			t.Errorf("err = %v, want ErrPostNotFound", err)
		}
	})
}

func TestDeleteOwnership(t *testing.T) {
	svc := newService()
	ctx := context.Background()

	if err := svc.Delete(ctx, "p1", "999"); !errors.Is(err, service.ErrForbidden) {
		t.Errorf("non-author delete err = %v, want ErrForbidden", err)
	}
	if err := svc.Delete(ctx, "p1", "123"); err != nil {
		t.Fatalf("author delete: %v", err)
	}
	if err := svc.Delete(ctx, "p1", "123"); !errors.Is(err, service.ErrPostNotFound) {
		t.Errorf("second delete err = %v, want ErrPostNotFound", err)
	}
}

func TestLike(t *testing.T) {
	svc := newService()
	ctx := context.Background()

	t.Run("like increments count", func(t *testing.T) {
		post, err := svc.Like(ctx, "p1", "u1")
		if err != nil {
			t.Fatalf("Like: %v", err)
		}
		if post.Likes != 1 {
			t.Errorf("likes = %d, want 1", post.Likes)
		}
	})

	t.Run("repeat like idempotent", func(t *testing.T) {
		post, err := svc.Like(ctx, "p1", "u1")
		if err != nil {
			t.Fatal(err)
		}
		if post.Likes != 1 {
			t.Errorf("likes = %d, want 1 (idempotent)", post.Likes)
		}
	})

	t.Run("no user id invalid", func(t *testing.T) {
		if _, err := svc.Like(ctx, "p1", ""); !errors.Is(err, service.ErrInvalidInput) {
			t.Errorf("err = %v, want ErrInvalidInput", err)
		}
	})

	t.Run("unknown post", func(t *testing.T) {
		if _, err := svc.Like(ctx, "nope", "u1"); !errors.Is(err, service.ErrPostNotFound) {
			t.Errorf("err = %v, want ErrPostNotFound", err)
		}
	})
}
