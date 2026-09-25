// Package repository abstracts user persistence. The in-memory
// implementation is a placeholder until a real database lands.
package repository

import (
	"context"
	"errors"
	"sync"
	"time"

	"user/internal/model"
)

var ErrNotFound = errors.New("user not found")

type UserRepository interface {
	GetByID(ctx context.Context, id string) (*model.User, error)
	Update(ctx context.Context, id string, u *model.User) (*model.User, error)
	Delete(ctx context.Context, id string) error
}

type InMemoryUserRepository struct {
	mu    sync.RWMutex
	users map[string]*model.User
}

func NewInMemory() *InMemoryUserRepository {
	r := &InMemoryUserRepository{users: make(map[string]*model.User)}
	now := time.Now().UTC()
	r.users["123"] = &model.User{
		ID:        "123",
		Name:      "Aji",
		Email:     "aji@example.com",
		CreatedAt: now,
		UpdatedAt: now,
	}
	return r
}

func (r *InMemoryUserRepository) GetByID(ctx context.Context, id string) (*model.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	u, ok := r.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	copy := *u
	return &copy, nil
}

func (r *InMemoryUserRepository) Update(ctx context.Context, id string, u *model.User) (*model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cur, ok := r.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	cur.Name = u.Name
	cur.Email = u.Email
	cur.UpdatedAt = time.Now().UTC()

	copy := *cur
	return &copy, nil
}

func (r *InMemoryUserRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.users[id]; !ok {
		return ErrNotFound
	}
	delete(r.users, id)
	return nil
}
