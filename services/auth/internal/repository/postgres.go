package repository

import (
	"context"
	_ "embed"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"auth/internal/password"
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
	r := &Postgres{
		Users:   &PostgresUserRepository{pool: pool},
		Refresh: &PostgresRefreshTokenStore{pool: pool},
	}
	if err := r.seed(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return r, nil
}

type Postgres struct {
	Users   *PostgresUserRepository
	Refresh *PostgresRefreshTokenStore
}

func (r *Postgres) Close() {
	r.Users.pool.Close()
}

func (r *Postgres) seed(ctx context.Context) error {
	var n int
	if err := r.Users.pool.QueryRow(ctx, "SELECT COUNT(*) FROM credentials").Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	salt, err := password.NewSalt()
	if err != nil {
		return err
	}
	h := password.Hash("password123", salt)
	_, err = r.Users.pool.Exec(ctx,
		"INSERT INTO credentials (id, email, salt, password_hash, created_at) VALUES ($1, $2, $3, $4, $5)",
		"123", "aji@example.com", salt, h, time.Now().UTC())
	return err
}

type PostgresUserRepository struct {
	pool *pgxpool.Pool
}

func (r *PostgresUserRepository) Create(ctx context.Context, cred *Credential) error {
	_, err := r.pool.Exec(ctx,
		"INSERT INTO credentials (id, email, salt, password_hash, created_at) VALUES ($1, $2, $3, $4, $5)",
		cred.ID, cred.Email, cred.Salt, cred.PasswordHash, cred.CreatedAt)
	return mapDuplicate(err)
}

func (r *PostgresUserRepository) GetByEmail(ctx context.Context, email string) (*Credential, error) {
	row := r.pool.QueryRow(ctx,
		"SELECT id, email, salt, password_hash, created_at FROM credentials WHERE email = $1", email)
	return scanCredential(row)
}

func (r *PostgresUserRepository) GetByID(ctx context.Context, id string) (*Credential, error) {
	row := r.pool.QueryRow(ctx,
		"SELECT id, email, salt, password_hash, created_at FROM credentials WHERE id = $1", id)
	return scanCredential(row)
}

func scanCredential(row pgx.Row) (*Credential, error) {
	var cred Credential
	if err := row.Scan(&cred.ID, &cred.Email, &cred.Salt, &cred.PasswordHash, &cred.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	cred.CreatedAt = cred.CreatedAt.UTC()
	return &cred, nil
}

type PostgresRefreshTokenStore struct {
	pool *pgxpool.Pool
}

func (s *PostgresRefreshTokenStore) Get(ctx context.Context, hash string) (string, time.Time, bool) {
	var userID string
	var expiresAt time.Time
	err := s.pool.QueryRow(ctx,
		"SELECT user_id, expires_at FROM refresh_tokens WHERE hash = $1", hash).
		Scan(&userID, &expiresAt)
	if err != nil {
		return "", time.Time{}, false
	}
	expiresAt = expiresAt.UTC()
	if !expiresAt.After(time.Now().UTC()) {
		s.Delete(ctx, hash)
		return "", time.Time{}, false
	}
	return userID, expiresAt, true
}

func (s *PostgresRefreshTokenStore) Store(ctx context.Context, hash, userID string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO refresh_tokens (hash, user_id, expires_at) VALUES ($1, $2, $3)
		 ON CONFLICT (hash) DO UPDATE SET user_id = EXCLUDED.user_id, expires_at = EXCLUDED.expires_at`,
		hash, userID, expiresAt)
	return err
}

func (s *PostgresRefreshTokenStore) Delete(ctx context.Context, hash string) {
	_, _ = s.pool.Exec(ctx, "DELETE FROM refresh_tokens WHERE hash = $1", hash)
}

func mapDuplicate(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrDuplicate
	}
	return err
}
