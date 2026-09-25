package service_test

import (
	"context"
	"errors"
	"testing"

	"user/internal/model"
	"user/internal/repository"
	"user/internal/service"
)

func newService() *service.Service {
	return service.New(repository.NewInMemory())
}

func strPtr(s string) *string { return &s }

func TestGetByIDFound(t *testing.T) {
	u, err := newService().GetByID(context.Background(), "123")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if u.Name != "Aji" {
		t.Errorf("name = %q, want %q", u.Name, "Aji")
	}
}

func TestGetByIDNotFound(t *testing.T) {
	if _, err := newService().GetByID(context.Background(), "nope"); !errors.Is(err, service.ErrUserNotFound) {
		t.Errorf("err = %v, want ErrUserNotFound", err)
	}
}

func TestGetMeEmptyID(t *testing.T) {
	if _, err := newService().GetMe(context.Background(), ""); !errors.Is(err, service.ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput", err)
	}
}

func TestGetMeExistingUser(t *testing.T) {
	u, err := newService().GetMe(context.Background(), "123")
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}
	if u.ID != "123" {
		t.Errorf("id = %q, want %q", u.ID, "123")
	}
}

func TestUpdateValidation(t *testing.T) {
	tests := []struct {
		name string
		upd  model.UserUpdate
		want error
	}{
		{name: "no fields", upd: model.UserUpdate{}, want: service.ErrInvalidInput},
		{name: "empty name", upd: model.UserUpdate{Name: strPtr("   ")}, want: service.ErrInvalidInput},
		{name: "invalid email", upd: model.UserUpdate{Email: strPtr("not-an-email")}, want: service.ErrInvalidInput},
	}

	svc := newService()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := svc.Update(context.Background(), "123", tt.upd); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestUpdateValid(t *testing.T) {
	svc := newService()

	u, err := svc.Update(context.Background(), "123", model.UserUpdate{Name: strPtr("Budi")})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if u.Name != "Budi" {
		t.Errorf("name = %q, want %q", u.Name, "Budi")
	}
	if u.Email != "aji@example.com" {
		t.Errorf("email should be untouched, got %q", u.Email)
	}
}

func TestUpdateUnknownUser(t *testing.T) {
	if _, err := newService().Update(context.Background(), "nope", model.UserUpdate{Name: strPtr("X")}); !errors.Is(err, service.ErrUserNotFound) {
		t.Errorf("err = %v, want ErrUserNotFound", err)
	}
}

func TestDeleteRoutes(t *testing.T) {
	svc := newService()

	if err := svc.Delete(context.Background(), "123"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := svc.Delete(context.Background(), "123"); !errors.Is(err, service.ErrUserNotFound) {
		t.Errorf("second delete err = %v, want ErrUserNotFound", err)
	}
}
