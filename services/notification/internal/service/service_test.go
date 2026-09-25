package service_test

import (
	"context"
	"errors"
	"testing"

	"notification/internal/repository"
	"notification/internal/service"
)

func newService() *service.Service {
	return service.New(repository.NewInMemory())
}

func TestIngestValid(t *testing.T) {
	svc := newService()

	n, err := svc.Ingest(context.Background(), "post_created", []byte(`{"user_id":"123","post_id":"p9","title":"New"}`))
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if n.ID == "" {
		t.Error("notification should have generated id")
	}
	if n.UserID != "123" || n.Type != "post_created" {
		t.Errorf("got %+v, want user 123 type post_created", n)
	}
	if n.Read {
		t.Error("new notification must be unread")
	}
}

func TestIngestValidation(t *testing.T) {
	svc := newService()
	ctx := context.Background()

	tests := []struct {
		name      string
		eventType string
		payload   string
	}{
		{name: "empty event type", eventType: "  ", payload: `{"user_id":"123"}`},
		{name: "missing user id", eventType: "post_created", payload: `{"title":"x"}`},
		{name: "invalid json", eventType: "post_created", payload: `not-json`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := svc.Ingest(ctx, tt.eventType, []byte(tt.payload)); !errors.Is(err, service.ErrInvalidInput) {
				t.Errorf("err = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestListByUser(t *testing.T) {
	svc := newService()

	items, err := svc.ListByUser(context.Background(), "123")
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("got %d notifications, want 2", len(items))
	}
}

func TestListByUserRequiresUser(t *testing.T) {
	svc := newService()

	if _, err := svc.ListByUser(context.Background(), ""); !errors.Is(err, service.ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput", err)
	}
}

func TestGetByID(t *testing.T) {
	svc := newService()
	ctx := context.Background()

	t.Run("owner can read", func(t *testing.T) {
		n, err := svc.GetByID(ctx, "n1", "123")
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if n.ID != "n1" {
			t.Errorf("id = %q, want n1", n.ID)
		}
	})

	t.Run("unknown", func(t *testing.T) {
		if _, err := svc.GetByID(ctx, "nope", "123"); !errors.Is(err, service.ErrNotificationNotFound) {
			t.Errorf("err = %v, want ErrNotificationNotFound", err)
		}
	})

	t.Run("non-owner forbidden", func(t *testing.T) {
		if _, err := svc.GetByID(ctx, "n1", "999"); !errors.Is(err, service.ErrForbidden) {
			t.Errorf("err = %v, want ErrForbidden", err)
		}
	})
}

func TestMarkRead(t *testing.T) {
	svc := newService()
	ctx := context.Background()

	t.Run("owner marks read", func(t *testing.T) {
		n, err := svc.MarkRead(ctx, "n1", "123")
		if err != nil {
			t.Fatalf("MarkRead: %v", err)
		}
		if !n.Read {
			t.Error("notification should be read after MarkRead")
		}
	})

	t.Run("non-owner forbidden", func(t *testing.T) {
		if _, err := svc.MarkRead(ctx, "n1", "999"); !errors.Is(err, service.ErrForbidden) {
			t.Errorf("err = %v, want ErrForbidden", err)
		}
	})

	t.Run("unknown", func(t *testing.T) {
		if _, err := svc.MarkRead(ctx, "nope", "123"); !errors.Is(err, service.ErrNotificationNotFound) {
			t.Errorf("err = %v, want ErrNotificationNotFound", err)
		}
	})
}
