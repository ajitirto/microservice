// Package repository abstracts auth credential and refresh token
// persistence. The in-memory implementations are placeholders until a
// real database lands.
package repository

import (
	"context"
	"errors"
	"sync"
	"time"

	"auth/internal/password"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrDuplicate = errors.New("duplicate")
)

type Credential struct {
	ID           string
	Email        string
	Salt         string
	PasswordHash string
	CreatedAt    time.Time
}

type UserRepository interface {
	Create(ctx context.Context, cred *Credential) error
	GetByEmail(ctx context.Context, email string) (*Credential, error)
	GetByID(ctx context.Context, id string) (*Credential, error)
}

type InMemory struct {
	Users   *InMemoryUserRepository
	Refresh *InMemoryRefreshTokenStore
}

func NewInMemory() *InMemory {
	byEmail := make(map[string]*Credential)
	byID := make(map[string]*Credential)

	salt, err := password.NewSalt()
	if err != nil {
		panic(err)
	}
	now := time.Now().UTC()
	seed := &Credential{
		ID:           "123",
		Email:        "aji@example.com",
		Salt:         salt,
		PasswordHash: password.Hash("password123", salt),
		CreatedAt:    now,
	}
	byEmail[seed.Email] = seed
	byID[seed.ID] = seed

	return &InMemory{
		Users:   &InMemoryUserRepository{byEmail: byEmail, byID: byID},
		Refresh: &InMemoryRefreshTokenStore{tokens: make(map[string]refreshEntry)},
	}
}

type InMemoryUserRepository struct {
	mu      sync.RWMutex
	byEmail map[string]*Credential
	byID    map[string]*Credential
}

func (r *InMemoryUserRepository) Create(ctx context.Context, cred *Credential) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byEmail[cred.Email]; exists {
		return ErrDuplicate
	}
	r.byEmail[cred.Email] = cred
	r.byID[cred.ID] = cred
	return nil
}

func (r *InMemoryUserRepository) GetByEmail(ctx context.Context, email string) (*Credential, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cred, ok := r.byEmail[email]
	if !ok {
		return nil, ErrNotFound
	}
	copy := *cred
	return &copy, nil
}

func (r *InMemoryUserRepository) GetByID(ctx context.Context, id string) (*Credential, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cred, ok := r.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	copy := *cred
	return &copy, nil
}

type refreshEntry struct {
	UserID    string
	ExpiresAt time.Time
}

type RefreshTokenStore interface {
	Get(ctx context.Context, hash string) (userID string, expiresAt time.Time, ok bool)
	Store(ctx context.Context, hash, userID string, expiresAt time.Time) error
	Delete(ctx context.Context, hash string)
}

type InMemoryRefreshTokenStore struct {
	mu     sync.RWMutex
	tokens map[string]refreshEntry
}

func (s *InMemoryRefreshTokenStore) Get(ctx context.Context, hash string) (string, time.Time, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, ok := s.tokens[hash]
	if !ok {
		return "", time.Time{}, false
	}
	return e.UserID, e.ExpiresAt, true
}

func (s *InMemoryRefreshTokenStore) Store(ctx context.Context, hash, userID string, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tokens[hash] = refreshEntry{UserID: userID, ExpiresAt: expiresAt}
	return nil
}

func (s *InMemoryRefreshTokenStore) Delete(ctx context.Context, hash string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.tokens, hash)
}
