package repository

import (
	"context"
	_ "embed"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"post/internal/model"
)

//go:embed schema.sql
var postgresSchema string

const postColumns = `p.id, p.title, p.content, p.author_id, p.created_at, p.updated_at, COUNT(l.user_id)`

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
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM posts").Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx,
		"INSERT INTO posts (id, title, content, author_id, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6), ($7, $8, $9, $10, $11, $12)",
		"p1", "Hello Microservices", "First post on the POC. Gateway, user, and auth services are up.", "123", now.Add(-2*time.Minute), now.Add(-2*time.Minute),
		"p2", "Post Service Coming Up", "This post is served by the post service using PostgreSQL.", "123", now.Add(-1*time.Minute), now.Add(-1*time.Minute))
	return err
}

func (r *Postgres) Create(ctx context.Context, post *model.Post) error {
	_, err := r.pool.Exec(ctx,
		"INSERT INTO posts (id, title, content, author_id, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6)",
		post.ID, post.Title, post.Content, post.AuthorID, post.CreatedAt, post.UpdatedAt)
	return err
}

func (r *Postgres) GetByID(ctx context.Context, id string) (*model.Post, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+postColumns+` FROM posts p LEFT JOIN post_likes l ON l.post_id = p.id
		 WHERE p.id = $1 GROUP BY p.id`, id)
	post, err := scanPost(row)
	if err != nil {
		return nil, err
	}
	return post, nil
}

func (r *Postgres) List(ctx context.Context) ([]*model.Post, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+postColumns+` FROM posts p LEFT JOIN post_likes l ON l.post_id = p.id
		 GROUP BY p.id ORDER BY p.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	posts := make([]*model.Post, 0)
	for rows.Next() {
		post, err := scanPost(rows)
		if err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	return posts, rows.Err()
}

func (r *Postgres) Update(ctx context.Context, post *model.Post) error {
	tag, err := r.pool.Exec(ctx,
		"UPDATE posts SET title = $2, content = $3, updated_at = $4 WHERE id = $1",
		post.ID, post.Title, post.Content, time.Now().UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Postgres) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Postgres) AddLike(ctx context.Context, postID, userID string) (int, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM posts WHERE id = $1)", postID).Scan(&exists); err != nil {
		return 0, err
	}
	if !exists {
		return 0, ErrNotFound
	}
	if _, err := r.pool.Exec(ctx,
		"INSERT INTO post_likes (post_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
		postID, userID); err != nil {
		return 0, err
	}
	var count int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM post_likes WHERE post_id = $1", postID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func scanPost(row pgx.Row) (*model.Post, error) {
	var post model.Post
	if err := row.Scan(&post.ID, &post.Title, &post.Content, &post.AuthorID, &post.CreatedAt, &post.UpdatedAt, &post.Likes); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	post.CreatedAt = post.CreatedAt.UTC()
	post.UpdatedAt = post.UpdatedAt.UTC()
	return &post, nil
}
