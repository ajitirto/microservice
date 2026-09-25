package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"post/internal/model"
	"post/internal/repository"
	"post/internal/service"
	"post/internal/userresolver"
)

type recordedEvent struct {
	eventType string
	payload   any
}

type recordingPublisher struct {
	mu     sync.Mutex
	events []recordedEvent
}

func (p *recordingPublisher) Publish(ctx context.Context, eventType string, payload any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, recordedEvent{eventType: eventType, payload: payload})
	return nil
}

func (p *recordingPublisher) count(eventType string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for _, e := range p.events {
		if e.eventType == eventType {
			n++
		}
	}
	return n
}

func (p *recordingPublisher) last(eventType string) (any, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := len(p.events) - 1; i >= 0; i-- {
		if p.events[i].eventType == eventType {
			return p.events[i].payload, true
		}
	}
	return nil, false
}

type failingPublisher struct{}

func (failingPublisher) Publish(ctx context.Context, eventType string, payload any) error {
	return errors.New("notification service down")
}

func newService() (*service.Service, *recordingPublisher) {
	pub := &recordingPublisher{}
	return service.New(repository.NewInMemory(), pub, knownUsers()), pub
}

type stubUsers struct {
	users map[string]*model.UserInfo
	err   error
}

func (s stubUsers) GetUser(ctx context.Context, userID string) (*model.UserInfo, error) {
	if s.err != nil {
		return nil, s.err
	}
	if u, ok := s.users[userID]; ok {
		return u, nil
	}
	return nil, userresolver.ErrUserNotFound
}

func knownUsers() stubUsers {
	return stubUsers{users: map[string]*model.UserInfo{
		"123": {ID: "123", Name: "Aji", Email: "aji@example.com"},
	}}
}

// waitFor polls until cond is true or the deadline passes; publishing
// is asynchronous by design (fire-and-forget).
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met within deadline")
}

func strPtr(s string) *string { return &s }

func TestCreateValidPost(t *testing.T) {
	svc, _ := newService()

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

func TestCreateResolvesAuthorName(t *testing.T) {
	svc, _ := newService()

	post, err := svc.Create(context.Background(), "123", model.CreatePostInput{Title: "Hello", Content: "World"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if post.AuthorName != "Aji" {
		t.Errorf("author_name = %q, want Aji", post.AuthorName)
	}
}

func TestCreateRejectsUnknownAuthor(t *testing.T) {
	svc, _ := newService()

	_, err := svc.Create(context.Background(), "999", model.CreatePostInput{Title: "Hello", Content: "World"})
	if !errors.Is(err, userresolver.ErrUserNotFound) {
		t.Errorf("err = %v, want ErrUserNotFound", err)
	}
}

func TestCreateWhenUserServiceUnavailable(t *testing.T) {
	svc := service.New(repository.NewInMemory(), failingPublisher{}, stubUsers{err: userresolver.ErrUserUnavailable})

	_, err := svc.Create(context.Background(), "123", model.CreatePostInput{Title: "Hello", Content: "World"})
	if !errors.Is(err, userresolver.ErrUserUnavailable) {
		t.Errorf("err = %v, want ErrUserUnavailable", err)
	}
	if got, _ := svc.List(context.Background()); len(got) != 2 {
		t.Errorf("post should not be created; list len = %d, want 2", len(got))
	}
}

func TestGetByIDAndListEnrichAuthor(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()

	post, err := svc.GetByID(ctx, "p1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if post.AuthorName != "Aji" {
		t.Errorf("author_name = %q, want Aji", post.AuthorName)
	}

	posts, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, p := range posts {
		if p.AuthorName != "Aji" {
			t.Errorf("post %s author_name = %q, want Aji", p.ID, p.AuthorName)
		}
	}
}

func TestCreateValidation(t *testing.T) {
	svc, _ := newService()
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
	svc, _ := newService()
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
	svc, _ := newService()
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
	svc, _ := newService()
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
	svc, _ := newService()
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

func TestCreatePublishesPostCreated(t *testing.T) {
	svc, pub := newService()

	post, err := svc.Create(context.Background(), "123", model.CreatePostInput{Title: "Hello", Content: "World"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	waitFor(t, func() bool { return pub.count("post_created") == 1 })

	payload, ok := pub.last("post_created")
	if !ok {
		t.Fatal("post_created event not published")
	}
	event, ok := payload.(model.PostCreatedEvent)
	if !ok {
		t.Fatalf("payload type = %T, want PostCreatedEvent", payload)
	}
	if event.UserID != "123" || event.PostID != post.ID || event.Title != "Hello" {
		t.Errorf("event = %+v, want user_id 123, post_id %s, title Hello", event, post.ID)
	}
}

func TestLikePublishesPostLiked(t *testing.T) {
	svc, pub := newService()

	if _, err := svc.Like(context.Background(), "p1", "u1"); err != nil {
		t.Fatalf("Like: %v", err)
	}

	waitFor(t, func() bool { return pub.count("post_liked") == 1 })

	payload, ok := pub.last("post_liked")
	if !ok {
		t.Fatal("post_liked event not published")
	}
	event, ok := payload.(model.PostLikedEvent)
	if !ok {
		t.Fatalf("payload type = %T, want PostLikedEvent", payload)
	}
	if event.UserID != "123" || event.PostID != "p1" || event.LikerID != "u1" {
		t.Errorf("event = %+v, want recipient 123, post p1, liker u1", event)
	}
}

func TestCreateSucceedsWhenPublisherFails(t *testing.T) {
	svc := service.New(repository.NewInMemory(), failingPublisher{}, knownUsers())

	post, err := svc.Create(context.Background(), "123", model.CreatePostInput{Title: "Hello", Content: "World"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if post.ID == "" {
		t.Error("post should still be created despite publisher failure")
	}
}
