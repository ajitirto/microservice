// Package service contains the post domain business logic: CRUD with
// author-ownership enforcement and idempotent likes.
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
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

type Service struct {
	repo repository.PostRepository
	now  func() time.Time
}

func New(repo repository.PostRepository) *Service {
	return &Service{
		repo: repo,
		now:  time.Now,
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

	id, err := newID()
	if err != nil {
		return nil, err
	}
	post := &model.Post{
		ID:        id,
		Title:     title,
		Content:   content,
		AuthorID:  authorID,
		CreatedAt: s.now().UTC(),
		UpdatedAt: s.now().UTC(),
	}
	if err := s.repo.Create(ctx, post); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, post.ID)
}

func (s *Service) GetByID(ctx context.Context, id string) (*model.Post, error) {
	post, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, s.mapRepoErr(err)
	}
	return post, nil
}

func (s *Service) List(ctx context.Context) ([]*model.Post, error) {
	return s.repo.List(ctx)
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
	return s.repo.GetByID(ctx, id)
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
	return s.repo.GetByID(ctx, postID)
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
