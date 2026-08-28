package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UserStore provides CRUD access to the users table.
type UserStore struct {
	pool *pgxpool.Pool
}

func NewUserStore(pool *pgxpool.Pool) *UserStore {
	return &UserStore{pool: pool}
}

func (s *UserStore) Create(ctx context.Context, name, email, passwordHash, role string, quotaBytes int64) (*User, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO users (name, email, password_hash, role, storage_quota_bytes)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, email, password_hash, role, storage_quota_bytes, created_at
	`, name, email, passwordHash, role, quotaBytes)

	u, err := scanUser(row)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrEmailTaken
		}
		return nil, err
	}
	return u, nil
}

func (s *UserStore) GetByEmail(ctx context.Context, email string) (*User, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, name, email, password_hash, role, storage_quota_bytes, created_at
		FROM users WHERE email = $1
	`, email)
	u, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (s *UserStore) GetByID(ctx context.Context, id int64) (*User, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, name, email, password_hash, role, storage_quota_bytes, created_at
		FROM users WHERE id = $1
	`, id)
	u, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (s *UserStore) List(ctx context.Context) ([]*User, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, email, password_hash, role, storage_quota_bytes, created_at
		FROM users ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *UserStore) UpdateQuota(ctx context.Context, id int64, quotaBytes int64) (*User, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE users SET storage_quota_bytes = $1 WHERE id = $2
		RETURNING id, name, email, password_hash, role, storage_quota_bytes, created_at
	`, quotaBytes, id)
	u, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (s *UserStore) Delete(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// rowScanner is satisfied by both pgx.Row and pgx.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (*User, error) {
	var u User
	if err := row.Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.StorageQuotaBytes, &u.CreatedAt); err != nil {
		return nil, err
	}
	return &u, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
