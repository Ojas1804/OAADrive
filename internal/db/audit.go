package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AuditStore writes entries to the audit_log table.
type AuditStore struct {
	pool *pgxpool.Pool
}

func NewAuditStore(pool *pgxpool.Pool) *AuditStore {
	return &AuditStore{pool: pool}
}

// Log records an action. userID may be nil for unauthenticated events.
func (s *AuditStore) Log(ctx context.Context, userID *int64, action, target string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO audit_log (user_id, action, target) VALUES ($1, $2, $3)
	`, userID, action, target)
	return err
}
