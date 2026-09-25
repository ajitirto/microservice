// Package service contains the user domain business logic.
package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"user/internal/model"
	"user/internal/repository"
)

var (
	ErrUserNotFound = errors.New("user not found")
	ErrInvalidInput = errors.New("invalid input")
)

type Service struct {
	repo repository.UserRepository
}

func New(repo repository.UserRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetByID(ctx context.Context, id string) (*model.User, error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, s.mapRepoErr(err)
	}
	return u, nil
}

func (s *Service) GetMe(ctx context.Context, userID string) (*model.User, error) {
	if userID == "" {
		return nil, fmt.Errorf("%w: user id required", ErrInvalidInput)
	}
	return s.GetByID(ctx, userID)
}

func (s *Service) Update(ctx context.Context, id string, upd model.UserUpdate) (*model.User, error) {
	if upd.Name == nil && upd.Email == nil {
		return nil, fmt.Errorf("%w: at least one field to update", ErrInvalidInput)
	}

	cur, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, s.mapRepoErr(err)
	}

	if upd.Name != nil {
		name := strings.TrimSpace(*upd.Name)
		if name == "" {
			return nil, fmt.Errorf("%w: name must not be empty", ErrInvalidInput)
		}
		cur.Name = name
	}
	if upd.Email != nil {
		email := strings.TrimSpace(*upd.Email)
		if _, err := mail.ParseAddress(email); err != nil {
			return nil, fmt.Errorf("%w: invalid email", ErrInvalidInput)
		}
		cur.Email = email
	}

	return s.repo.Update(ctx, id, cur)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return s.mapRepoErr(err)
	}
	return nil
}

func (s *Service) mapRepoErr(err error) error {
	if errors.Is(err, repository.ErrNotFound) {
		return ErrUserNotFound
	}
	return err
}
