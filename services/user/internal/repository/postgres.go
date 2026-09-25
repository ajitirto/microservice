package repository

import (
	"context"
	_ "embed"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"user/internal/model"
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
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx,
		"INSERT INTO users (id, name, email, created_at, updated_at) VALUES ($1, $2, $3, $4, $5)",
		"123", "Aji", "aji@example.com", now, now)
	return err
}

func (r *Postgres) GetByID(ctx context.Context, id string) (*model.User, error) {
	row := r.pool.QueryRow(ctx,
		"SELECT id, name, email, created_at, updated_at FROM users WHERE id = $1", id)
	var u model.User
	if err := row.Scan(&u.ID, &u.Name, &u.Email, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	u.CreatedAt = u.CreatedAt.UTC()
	u.UpdatedAt = u.UpdatedAt.UTC()
	return &u, nil
}

func (r *Postgres) Update(ctx context.Context, id string, u *model.User) (*model.User, error) {
	row := r.pool.QueryRow(ctx,
		`UPDATE users SET name = $2, email = $3, updated_at = $4
		 WHERE id = $1
		 RETURNING id, name, email, created_at, updated_at`,
		id, u.Name, u.Email, time.Now().UTC())
	var updated model.User
	if err := row.Scan(&updated.ID, &updated.Name, &updated.Email, &updated.CreatedAt, &updated.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	updated.CreatedAt = updated.CreatedAt.UTC()
	updated.UpdatedAt = updated.UpdatedAt.UTC()
	return &updated, nil
}

func (r *Postgres) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, "DELETE FROM users WHERE id = $1", id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
