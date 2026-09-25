// Package service contains the post domain business logic: CRUD with
// author-ownership enforcement and idempotent likes.
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"post/internal/model"
	"post/internal/repository"
)

var (
	ErrPostNotFound = errors.New("post not found")
	ErrInvalidInput = errors.New("invalid input")
	ErrForbidden    = errors.New("forbidden")
)

type Publisher interface {
	Publish(ctx context.Context, eventType string, payload any) error
}

// UserResolver resolves user data owned by the user service. It returns
// userresolver.ErrUserNotFound for unknown users and
// userresolver.ErrUserUnavailable when the user service cannot be reached.
type UserResolver interface {
	GetUser(ctx context.Context, userID string) (*model.UserInfo, error)
}

type Service struct {
	repo  repository.PostRepository
	pub   Publisher
	users UserResolver
	now   func() time.Time
}

func New(repo repository.PostRepository, pub Publisher, users UserResolver) *Service {
	return &Service{
		repo:  repo,
		pub:   pub,
		users: users,
		now:   time.Now,
	}
}

func (s *Service) Create(ctx context.Context, authorID string, in model.CreatePostInput) (*model.Post, error) {
	if authorID == "" {
		return nil, fmt.Errorf("%w: author required", ErrInvalidInput)
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, fmt.Errorf("%w: title must not be empty", ErrInvalidInput)
	}
	content := strings.TrimSpace(in.Content)
	if content == "" {
		return nil, fmt.Errorf("%w: content must not be empty", ErrInvalidInput)
	}

	user, err := s.users.GetUser(ctx, authorID)
	if err != nil {
		return nil, err
	}

	id, err := newID()
	if err != nil {
		return nil, err
	}
	post := &model.Post{
		ID:         id,
		Title:      title,
		Content:    content,
		AuthorID:   authorID,
		AuthorName: user.Name,
		CreatedAt:  s.now().UTC(),
		UpdatedAt:  s.now().UTC(),
	}
	if err := s.repo.Create(ctx, post); err != nil {
		return nil, err
	}
	created, err := s.repo.GetByID(ctx, post.ID)
	if err != nil {
		return nil, err
	}
	s.notify("post_created", model.PostCreatedEvent{
		UserID: created.AuthorID,
		PostID: created.ID,
		Title:  created.Title,
	})
	return created, nil
}

func (s *Service) GetByID(ctx context.Context, id string) (*model.Post, error) {
	post, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, s.mapRepoErr(err)
	}
	s.enrich(ctx, post)
	return post, nil
}

func (s *Service) List(ctx context.Context) ([]*model.Post, error) {
	posts, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, post := range posts {
		s.enrich(ctx, post)
	}
	return posts, nil
}

func (s *Service) Update(ctx context.Context, id, authorID string, in model.UpdatePostInput) (*model.Post, error) {
	if in.Title == nil && in.Content == nil {
		return nil, fmt.Errorf("%w: at least one field to update", ErrInvalidInput)
	}

	cur, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, s.mapRepoErr(err)
	}
	if cur.AuthorID != authorID {
		return nil, fmt.Errorf("%w: only the author can update this post", ErrForbidden)
	}

	if in.Title != nil {
		title := strings.TrimSpace(*in.Title)
		if title == "" {
			return nil, fmt.Errorf("%w: title must not be empty", ErrInvalidInput)
		}
		cur.Title = title
	}
	if in.Content != nil {
		content := strings.TrimSpace(*in.Content)
		if content == "" {
			return nil, fmt.Errorf("%w: content must not be empty", ErrInvalidInput)
		}
		cur.Content = content
	}
	cur.UpdatedAt = s.now().UTC()

	updated := &model.Post{
		ID:        cur.ID,
		Title:     cur.Title,
		Content:   cur.Content,
		AuthorID:  cur.AuthorID,
		CreatedAt: cur.CreatedAt,
		UpdatedAt: cur.UpdatedAt,
	}
	if err := s.repo.Update(ctx, updated); err != nil {
		return nil, s.mapRepoErr(err)
	}
	updated, err = s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, s.mapRepoErr(err)
	}
	s.enrich(ctx, updated)
	return updated, nil
}

func (s *Service) Delete(ctx context.Context, id, authorID string) error {
	cur, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return s.mapRepoErr(err)
	}
	if cur.AuthorID != authorID {
		return fmt.Errorf("%w: only the author can delete this post", ErrForbidden)
	}
	return s.mapRepoErr(s.repo.Delete(ctx, id))
}

func (s *Service) Like(ctx context.Context, postID, userID string) (*model.Post, error) {
	if userID == "" {
		return nil, fmt.Errorf("%w: user required", ErrInvalidInput)
	}
	if _, err := s.repo.AddLike(ctx, postID, userID); err != nil {
		return nil, s.mapRepoErr(err)
	}
	post, err := s.repo.GetByID(ctx, postID)
	if err != nil {
		return nil, s.mapRepoErr(err)
	}
	s.notify("post_liked", model.PostLikedEvent{
		UserID:  post.AuthorID,
		PostID:  post.ID,
		LikerID: userID,
	})
	s.enrich(ctx, post)
	return post, nil
}

// notify publishes an event without blocking the request; failures are
// logged through the default logger.
func (s *Service) notify(eventType string, payload any) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := s.pub.Publish(ctx, eventType, payload); err != nil {
			slog.Error("failed to publish event",
				slog.String("event", eventType),
				slog.Any("error", err),
			)
		}
	}()
}

func (s *Service) enrich(ctx context.Context, post *model.Post) {
	user, err := s.users.GetUser(ctx, post.AuthorID)
	if err != nil {
		return
	}
	post.AuthorName = user.Name
}

func (s *Service) mapRepoErr(err error) error {
	if errors.Is(err, repository.ErrNotFound) {
		return ErrPostNotFound
	}
	return err
}

func newID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
