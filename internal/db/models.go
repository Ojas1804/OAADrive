package db

import (
	"errors"
	"time"
)

// Roles recognized by the users.role column.
const (
	RoleAdmin  = "admin"
	RoleMember = "member"
)

// ErrNotFound is returned by store methods when a row does not exist.
var ErrNotFound = errors.New("not found")

// ErrEmailTaken is returned when creating a user with a duplicate email.
var ErrEmailTaken = errors.New("email already taken")

// User mirrors the users table.
type User struct {
	ID                int64
	Name              string
	Email             string
	PasswordHash      string
	Role              string
	StorageQuotaBytes int64
	CreatedAt         time.Time
}

// File mirrors the files table. MimeType and ChecksumSHA256 are nullable.
type File struct {
	ID                int64
	OwnerID           int64
	OriginalFilename  string
	StorageBucket     string
	StorageObjectKey  string
	SizeBytes         int64
	MimeType          *string
	ChecksumSHA256    *string
	CreatedAt         time.Time
	DeletedAt         *time.Time
}

// UserStorageUsage aggregates a user's used bytes against their quota, used
// by the admin storage-usage endpoint.
type UserStorageUsage struct {
	UserID     int64
	Name       string
	UsedBytes  int64
	QuotaBytes int64
}
