// Package service contains the notification domain business logic:
// event ingestion and notification retrieval.
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"notification/internal/model"
	"notification/internal/repository"
)

var (
	ErrNotificationNotFound = errors.New("notification not found")
	ErrInvalidInput         = errors.New("invalid input")
	ErrForbidden            = errors.New("forbidden")
)

type Service struct {
	repo repository.NotificationRepository
	now  func() time.Time
}

func New(repo repository.NotificationRepository) *Service {
	return &Service{
		repo: repo,
		now:  time.Now,
	}
}

// Ingest creates a notification for the recipient named by the
// "user_id" field of the event payload.
func (s *Service) Ingest(ctx context.Context, eventType string, payload []byte) (*model.Notification, error) {
	eventType = strings.TrimSpace(eventType)
	if eventType == "" {
		return nil, fmt.Errorf("%w: event type required", ErrInvalidInput)
	}

	var envelope struct {
		UserID string `json:"user_id"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, fmt.Errorf("%w: invalid event payload", ErrInvalidInput)
	}
	if envelope.UserID == "" {
		return nil, fmt.Errorf("%w: user_id required", ErrInvalidInput)
	}

	id, err := newID()
	if err != nil {
		return nil, err
	}
	n := &model.Notification{
		ID:        id,
		UserID:    envelope.UserID,
		Type:      eventType,
		Payload:   payload,
		CreatedAt: s.now().UTC(),
	}
	if err := s.repo.Create(ctx, n); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *Service) ListByUser(ctx context.Context, userID string) ([]*model.Notification, error) {
	if userID == "" {
		return nil, fmt.Errorf("%w: user required", ErrInvalidInput)
	}
	return s.repo.ListByUser(ctx, userID)
}

func (s *Service) GetByID(ctx context.Context, id, userID string) (*model.Notification, error) {
	n, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotificationNotFound
		}
		return nil, err
	}
	if n.UserID != userID {
		return nil, fmt.Errorf("%w: only the recipient can access this notification", ErrForbidden)
	}
	return n, nil
}

func (s *Service) MarkRead(ctx context.Context, id, userID string) (*model.Notification, error) {
	if _, err := s.GetByID(ctx, id, userID); err != nil {
		return nil, err
	}
	if err := s.repo.MarkRead(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotificationNotFound
		}
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func newID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
