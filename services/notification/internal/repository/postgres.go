package repository

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"notification/internal/model"
)

//go:embed schema.sql
var postgresSchema string

func NewPostgres(ctx context.Context, dsn string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if _, err := pool.Exec(ctx, postgresSchema); err != nil {
		pool.Close()
		return nil, err
	}
	r := &Postgres{pool: pool}
	if err := r.seed(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return r, nil
}

type Postgres struct {
	pool *pgxpool.Pool
}

func (r *Postgres) Close() {
	r.pool.Close()
}

func (r *Postgres) seed(ctx context.Context) error {
	var n int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM notifications").Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	payload := `{"user_id":"123","post_id":"p1","title":"Hello Microservices"}`
	_, err := r.pool.Exec(ctx,
		"INSERT INTO notifications (id, user_id, type, payload, read, created_at) VALUES ($1, $2, $3, $4::jsonb, $5, $6)",
		"n1", "123", "post_created", payload, false, time.Now().UTC().Add(-2*time.Minute))
	return err
}

func (r *Postgres) Create(ctx context.Context, n *model.Notification) error {
	_, err := r.pool.Exec(ctx,
		"INSERT INTO notifications (id, user_id, type, payload, read, created_at) VALUES ($1, $2, $3, $4::jsonb, $5, $6)",
		n.ID, n.UserID, n.Type, []byte(n.Payload), n.Read, n.CreatedAt)
	return err
}

func (r *Postgres) GetByID(ctx context.Context, id string) (*model.Notification, error) {
	row := r.pool.QueryRow(ctx,
		"SELECT id, user_id, type, payload, read, created_at FROM notifications WHERE id = $1", id)
	return scanNotification(row)
}

func (r *Postgres) ListByUser(ctx context.Context, userID string) ([]*model.Notification, error) {
	rows, err := r.pool.Query(ctx,
		"SELECT id, user_id, type, payload, read, created_at FROM notifications WHERE user_id = $1 ORDER BY created_at DESC",
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*model.Notification, 0)
	for rows.Next() {
		item, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Postgres) MarkRead(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, "UPDATE notifications SET read = TRUE WHERE id = $1", id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanNotification(row pgx.Row) (*model.Notification, error) {
	var n model.Notification
	var payload []byte
	if err := row.Scan(&n.ID, &n.UserID, &n.Type, &payload, &n.Read, &n.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	n.Payload = json.RawMessage(payload)
	n.CreatedAt = n.CreatedAt.UTC()
	return &n, nil
}
