package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// FileStore provides CRUD access to the files table.
type FileStore struct {
	pool *pgxpool.Pool
}

func NewFileStore(pool *pgxpool.Pool) *FileStore {
	return &FileStore{pool: pool}
}

func (s *FileStore) Create(ctx context.Context, f *File) (*File, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO files (owner_id, original_filename, storage_bucket, storage_object_key, size_bytes, mime_type, checksum_sha256)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, owner_id, original_filename, storage_bucket, storage_object_key, size_bytes, mime_type, checksum_sha256, created_at, deleted_at
	`, f.OwnerID, f.OriginalFilename, f.StorageBucket, f.StorageObjectKey, f.SizeBytes, f.MimeType, f.ChecksumSHA256)
	return scanFile(row)
}

func (s *FileStore) GetByID(ctx context.Context, id int64) (*File, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, owner_id, original_filename, storage_bucket, storage_object_key, size_bytes, mime_type, checksum_sha256, created_at, deleted_at
		FROM files WHERE id = $1 AND deleted_at IS NULL
	`, id)
	f, err := scanFile(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return f, err
}

// ListByOwner returns a page of the owner's non-deleted files, optionally
// filtered by a mime type prefix (e.g. "image/"), plus the total matching count.
func (s *FileStore) ListByOwner(ctx context.Context, ownerID int64, mimePrefix string, page, pageSize int) ([]*File, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize

	rows, err := s.pool.Query(ctx, `
		SELECT id, owner_id, original_filename, storage_bucket, storage_object_key, size_bytes, mime_type, checksum_sha256, created_at, deleted_at
		FROM files
		WHERE owner_id = $1 AND deleted_at IS NULL AND ($2 = '' OR mime_type LIKE $2 || '%')
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`, ownerID, mimePrefix, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var files []*File
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, 0, err
		}
		files = append(files, f)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var total int
	err = s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM files
		WHERE owner_id = $1 AND deleted_at IS NULL AND ($2 = '' OR mime_type LIKE $2 || '%')
	`, ownerID, mimePrefix).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	return files, total, nil
}

func (s *FileStore) SumSizeByOwner(ctx context.Context, ownerID int64) (int64, error) {
	var sum int64
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(size_bytes), 0) FROM files WHERE owner_id = $1 AND deleted_at IS NULL
	`, ownerID).Scan(&sum)
	return sum, err
}

func (s *FileStore) SoftDelete(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE files SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// StorageUsageByUser returns used-vs-quota bytes for every user, for the
// admin storage-usage dashboard.
func (s *FileStore) StorageUsageByUser(ctx context.Context) ([]UserStorageUsage, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT u.id, u.name, COALESCE(SUM(f.size_bytes), 0) AS used_bytes, u.storage_quota_bytes
		FROM users u
		LEFT JOIN files f ON f.owner_id = u.id AND f.deleted_at IS NULL
		GROUP BY u.id, u.name, u.storage_quota_bytes
		ORDER BY u.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var usage []UserStorageUsage
	for rows.Next() {
		var uu UserStorageUsage
		if err := rows.Scan(&uu.UserID, &uu.Name, &uu.UsedBytes, &uu.QuotaBytes); err != nil {
			return nil, err
		}
		usage = append(usage, uu)
	}
	return usage, rows.Err()
}

func scanFile(row rowScanner) (*File, error) {
	var f File
	if err := row.Scan(
		&f.ID, &f.OwnerID, &f.OriginalFilename, &f.StorageBucket, &f.StorageObjectKey,
		&f.SizeBytes, &f.MimeType, &f.ChecksumSHA256, &f.CreatedAt, &f.DeletedAt,
	); err != nil {
		return nil, err
	}
	return &f, nil
}
