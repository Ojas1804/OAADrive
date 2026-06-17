package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"family-cloud/internal/models"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) AdminExists(ctx context.Context) (bool, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM users WHERE role = 'admin'`,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("admin check failed: %w", err)
	}
	return count > 0, nil
}

func (r *Repository) CreateUser(ctx context.Context, email, passwordHash string, role models.Role) (*models.User, error) {
	user := &models.User{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, role)
		 VALUES ($1, $2, $3)
		 RETURNING id, email, password_hash, role, is_active, failed_logins, locked_until, created_at, updated_at`,
		email, passwordHash, string(role),
	).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Role,
		&user.IsActive, &user.FailedLogins, &user.LockedUntil,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create user failed: %w", err)
	}
	return user, nil
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	user := &models.User{}
	err := r.db.QueryRow(ctx,
		`SELECT id, email, password_hash, role, is_active, failed_logins, locked_until, created_at, updated_at
		 FROM users WHERE email = $1`,
		email,
	).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Role,
		&user.IsActive, &user.FailedLogins, &user.LockedUntil,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by email failed: %w", err)
	}
	return user, nil
}

func (r *Repository) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	user := &models.User{}
	err := r.db.QueryRow(ctx,
		`SELECT id, email, password_hash, role, is_active, failed_logins, locked_until, created_at, updated_at
		 FROM users WHERE id = $1`,
		id,
	).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Role,
		&user.IsActive, &user.FailedLogins, &user.LockedUntil,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by id failed: %w", err)
	}
	return user, nil
}

func (r *Repository) IncrementFailedLogins(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE users SET failed_logins = failed_logins + 1, updated_at = NOW()
		 WHERE id = $1`,
		userID,
	)
	return err
}

func (r *Repository) LockUser(ctx context.Context, userID uuid.UUID, until time.Time) error {
	_, err := r.db.Exec(ctx,
		`UPDATE users SET locked_until = $1, updated_at = NOW() WHERE id = $2`,
		until, userID,
	)
	return err
}

func (r *Repository) ResetFailedLogins(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE users SET failed_logins = 0, locked_until = NULL, updated_at = NOW()
		 WHERE id = $1`,
		userID,
	)
	return err
}

func (r *Repository) CreateRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) (*models.RefreshToken, error) {
	rt := &models.RefreshToken{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		 VALUES ($1, $2, $3)
		 RETURNING id, user_id, token_hash, expires_at, created_at, revoked_at`,
		userID, tokenHash, expiresAt,
	).Scan(&rt.ID, &rt.UserID, &rt.TokenHash, &rt.ExpiresAt, &rt.CreatedAt, &rt.RevokedAt)
	if err != nil {
		return nil, fmt.Errorf("create refresh token failed: %w", err)
	}
	return rt, nil
}

func (r *Repository) GetRefreshToken(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	rt := &models.RefreshToken{}
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, token_hash, expires_at, created_at, revoked_at
		 FROM refresh_tokens
		 WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > NOW()`,
		tokenHash,
	).Scan(&rt.ID, &rt.UserID, &rt.TokenHash, &rt.ExpiresAt, &rt.CreatedAt, &rt.RevokedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrInvalidToken
		}
		return nil, fmt.Errorf("get refresh token failed: %w", err)
	}
	return rt, nil
}

func (r *Repository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	result, err := r.db.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = NOW() WHERE token_hash = $1`,
		tokenHash,
	)
	if err != nil {
		return fmt.Errorf("revoke refresh token failed: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrInvalidToken
	}
	return nil
}

func (r *Repository) RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = NOW()
		 WHERE user_id = $1 AND revoked_at IS NULL`,
		userID,
	)
	return err
}

func (r *Repository) CreateInvitation(ctx context.Context, email string, role models.Role, tokenHash string, invitedBy uuid.UUID, expiresAt time.Time) (*models.Invitation, error) {
	inv := &models.Invitation{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO invitations (email, role, token_hash, invited_by, expires_at)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, email, role, token_hash, invited_by, expires_at, accepted_at, created_at`,
		email, string(role), tokenHash, invitedBy, expiresAt,
	).Scan(&inv.ID, &inv.Email, &inv.Role, &inv.TokenHash, &inv.InvitedBy, &inv.ExpiresAt, &inv.AcceptedAt, &inv.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create invitation failed: %w", err)
	}
	return inv, nil
}

func (r *Repository) GetInvitationByToken(ctx context.Context, tokenHash string) (*models.Invitation, error) {
	inv := &models.Invitation{}
	err := r.db.QueryRow(ctx,
		`SELECT id, email, role, token_hash, invited_by, expires_at, accepted_at, created_at
		 FROM invitations
		 WHERE token_hash = $1 AND accepted_at IS NULL AND expires_at > NOW()`,
		tokenHash,
	).Scan(&inv.ID, &inv.Email, &inv.Role, &inv.TokenHash, &inv.InvitedBy, &inv.ExpiresAt, &inv.AcceptedAt, &inv.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrInvalidToken
		}
		return nil, fmt.Errorf("get invitation failed: %w", err)
	}
	return inv, nil
}

func (r *Repository) AcceptInvitation(ctx context.Context, tokenHash string) error {
	result, err := r.db.Exec(ctx,
		`UPDATE invitations SET accepted_at = NOW() WHERE token_hash = $1`,
		tokenHash,
	)
	if err != nil {
		return fmt.Errorf("accept invitation failed: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrInvalidToken
	}
	return nil
}
