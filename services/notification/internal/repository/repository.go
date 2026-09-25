// Package repository abstracts notification persistence. The in-memory
// implementation is a placeholder until a real database lands.
package repository

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"notification/internal/model"
)

var ErrNotFound = errors.New("not found")

type NotificationRepository interface {
	Create(ctx context.Context, n *model.Notification) error
	GetByID(ctx context.Context, id string) (*model.Notification, error)
	ListByUser(ctx context.Context, userID string) ([]*model.Notification, error)
	MarkRead(ctx context.Context, id string) error
}

type InMemoryNotificationRepository struct {
	mu    sync.RWMutex
	items map[string]*model.Notification
}

func NewInMemory() *InMemoryNotificationRepository {
	r := &InMemoryNotificationRepository{items: make(map[string]*model.Notification)}
	now := time.Now().UTC()

	r.items["n1"] = &model.Notification{
		ID:        "n1",
		UserID:    "123",
		Type:      "post_created",
		Payload:   []byte(`{"user_id":"123","post_id":"p1","title":"Hello Microservices"}`),
		Read:      false,
		CreatedAt: now.Add(-2 * time.Minute),
	}
	r.items["n2"] = &model.Notification{
		ID:        "n2",
		UserID:    "123",
		Type:      "post_liked",
		Payload:   []byte(`{"user_id":"123","post_id":"p1","liker_id":"u1"}`),
		Read:      true,
		CreatedAt: now.Add(-1 * time.Minute),
	}
	return r
}

func (r *InMemoryNotificationRepository) Create(ctx context.Context, n *model.Notification) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.items[n.ID] = n
	return nil
}

func (r *InMemoryNotificationRepository) GetByID(ctx context.Context, id string) (*model.Notification, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	n, ok := r.items[id]
	if !ok {
		return nil, ErrNotFound
	}
	copy := *n
	copy.Payload = append([]byte(nil), n.Payload...)
	return &copy, nil
}

func (r *InMemoryNotificationRepository) ListByUser(ctx context.Context, userID string) ([]*model.Notification, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]*model.Notification, 0)
	for _, n := range r.items {
		if n.UserID == userID {
			copy := *n
			copy.Payload = append([]byte(nil), n.Payload...)
			items = append(items, &copy)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	return items, nil
}

func (r *InMemoryNotificationRepository) MarkRead(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	n, ok := r.items[id]
	if !ok {
		return ErrNotFound
	}
	n.Read = true
	return nil
}
