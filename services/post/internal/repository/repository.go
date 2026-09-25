// Package repository abstracts post persistence. The in-memory
// implementation is a placeholder until a real database lands.
package repository

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"post/internal/model"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrDuplicate = errors.New("duplicate")
)

type PostRepository interface {
	Create(ctx context.Context, post *model.Post) error
	GetByID(ctx context.Context, id string) (*model.Post, error)
	List(ctx context.Context) ([]*model.Post, error)
	Update(ctx context.Context, post *model.Post) error
	Delete(ctx context.Context, id string) error
	AddLike(ctx context.Context, postID, userID string) (int, error)
}

type InMemoryPostRepository struct {
	mu    sync.RWMutex
	posts map[string]*model.Post
	likes map[string]map[string]struct{}
}

func NewInMemory() *InMemoryPostRepository {
	r := &InMemoryPostRepository{
		posts: make(map[string]*model.Post),
		likes: make(map[string]map[string]struct{}),
	}
	now := time.Now().UTC()
	r.posts["p1"] = &model.Post{
		ID:        "p1",
		Title:     "Hello Microservices",
		Content:   "First post on the POC. Gateway, user, and auth services are up.",
		AuthorID:  "123",
		CreatedAt: now.Add(-2 * time.Minute),
		UpdatedAt: now.Add(-2 * time.Minute),
	}
	r.posts["p2"] = &model.Post{
		ID:        "p2",
		Title:     "Post Service Coming Up",
		Content:   "This post is served by the post service using an in-memory store.",
		AuthorID:  "123",
		CreatedAt: now.Add(-1 * time.Minute),
		UpdatedAt: now.Add(-1 * time.Minute),
	}
	return r
}

func (r *InMemoryPostRepository) Create(ctx context.Context, post *model.Post) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.posts[post.ID]; exists {
		return ErrDuplicate
	}
	r.posts[post.ID] = post
	return nil
}

func (r *InMemoryPostRepository) GetByID(ctx context.Context, id string) (*model.Post, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	post, ok := r.posts[id]
	if !ok {
		return nil, ErrNotFound
	}
	return postWithLikes(post, r.likes[id]), nil
}

func (r *InMemoryPostRepository) List(ctx context.Context) ([]*model.Post, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	posts := make([]*model.Post, 0, len(r.posts))
	for _, post := range r.posts {
		posts = append(posts, postWithLikes(post, r.likes[post.ID]))
	}
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].CreatedAt.After(posts[j].CreatedAt)
	})
	return posts, nil
}

func (r *InMemoryPostRepository) Update(ctx context.Context, post *model.Post) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cur, ok := r.posts[post.ID]
	if !ok {
		return ErrNotFound
	}
	cur.Title = post.Title
	cur.Content = post.Content
	cur.UpdatedAt = post.UpdatedAt
	return nil
}

func (r *InMemoryPostRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.posts[id]; !ok {
		return ErrNotFound
	}
	delete(r.posts, id)
	delete(r.likes, id)
	return nil
}

func (r *InMemoryPostRepository) AddLike(ctx context.Context, postID, userID string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.posts[postID]; !ok {
		return 0, ErrNotFound
	}
	likers, ok := r.likes[postID]
	if !ok {
		likers = make(map[string]struct{})
		r.likes[postID] = likers
	}
	likers[userID] = struct{}{}
	return len(likers), nil
}

func postWithLikes(post *model.Post, likers map[string]struct{}) *model.Post {
	copy := *post
	copy.Likes = len(likers)
	return &copy
}
