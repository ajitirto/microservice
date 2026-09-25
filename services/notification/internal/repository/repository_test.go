package repository_test

import (
	"context"
	"errors"
	"testing"

	"notification/internal/model"
	"notification/internal/repository"
)

func TestListByUserReturnsSeedsNewestFirst(t *testing.T) {
	r := repository.NewInMemory()

	items, err := r.ListByUser(context.Background(), "123")
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d notifications, want 2", len(items))
	}
	// n2 (post_liked) is newer than n1 (post_created).
	if items[0].ID != "n2" || items[1].ID != "n1" {
		t.Errorf("order = [%s, %s], want [n2, n1]", items[0].ID, items[1].ID)
	}
}

func TestListByUserFiltersRecipient(t *testing.T) {
	r := repository.NewInMemory()

	items, err := r.ListByUser(context.Background(), "nobody")
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("got %d notifications, want 0", len(items))
	}
}

func TestGetByIDUnknown(t *testing.T) {
	r := repository.NewInMemory()

	if _, err := r.GetByID(context.Background(), "nope"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestCreateThenGetByID(t *testing.T) {
	r := repository.NewInMemory()
	n := &model.Notification{
		ID:      "n3",
		UserID:  "456",
		Type:    "post_liked",
		Payload: []byte(`{"user_id":"456"}`),
	}

	if err := r.Create(context.Background(), n); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := r.GetByID(context.Background(), "n3")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.UserID != "456" || got.Type != "post_liked" {
		t.Errorf("got = %+v, want user 456 type post_liked", got)
	}
}

func TestMarkRead(t *testing.T) {
	r := repository.NewInMemory()

	if err := r.MarkRead(context.Background(), "n1"); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	n, err := r.GetByID(context.Background(), "n1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if !n.Read {
		t.Error("notification should be marked read")
	}
}

func TestMarkReadUnknown(t *testing.T) {
	r := repository.NewInMemory()

	if err := r.MarkRead(context.Background(), "nope"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestGetByIDReturnsCopy(t *testing.T) {
	r := repository.NewInMemory()

	n, err := r.GetByID(context.Background(), "n1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	n.Payload[0] = 'x'
	n.Read = true

	stored, err := r.GetByID(context.Background(), "n1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if string(stored.Payload) == "x" || stored.Read {
		t.Error("mutating returned notification must not affect the store")
	}
}
