package models

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleMember   Role = "member"
	RoleReadOnly Role = "readonly"
)

type User struct {
	ID           uuid.UUID  `json:"id"         db:"id"`
	Email        string     `json:"email"      db:"email"`
	PasswordHash string     `json:"-"          db:"password_hash"`
	Role         Role       `json:"role"       db:"role"`
	IsActive     bool       `json:"is_active"  db:"is_active"`
	FailedLogins int        `json:"-"          db:"failed_logins"`
	LockedUntil  *time.Time `json:"-"          db:"locked_until"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
}

type File struct {
	ID        uuid.UUID `json:"id"         db:"id"`
	OwnerID   uuid.UUID `json:"owner_id"   db:"owner_id"`
	Bucket    string    `json:"bucket"     db:"bucket"`
	ObjectKey string    `json:"object_key" db:"object_key"`
	FileName  string    `json:"file_name"  db:"file_name"`
	Size      int64     `json:"size"       db:"size"`
	Checksum  string    `json:"checksum"   db:"checksum"`
	MimeType  string    `json:"mime_type"  db:"mime_type"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type FamilyMember struct {
	ID        uuid.UUID  `json:"id"         db:"id"`
	AdminID   uuid.UUID  `json:"admin_id"   db:"admin_id"`
	UserID    uuid.UUID  `json:"user_id"    db:"user_id"`
	InvitedAt time.Time  `json:"invited_at" db:"invited_at"`
	JoinedAt  *time.Time `json:"joined_at"  db:"joined_at"`
}

type UploadSession struct {
	ID          uuid.UUID  `json:"id"           db:"id"`
	UserID      uuid.UUID  `json:"user_id"      db:"user_id"`
	Bucket      string     `json:"bucket"       db:"bucket"`
	ObjectKey   string     `json:"object_key"   db:"object_key"`
	FileName    string     `json:"file_name"    db:"file_name"`
	TotalSize   int64      `json:"total_size"   db:"total_size"`
	UploadID    string     `json:"upload_id"    db:"upload_id"`
	Status      string     `json:"status"       db:"status"`
	CreatedAt   time.Time  `json:"created_at"   db:"created_at"`
	ExpiresAt   time.Time  `json:"expires_at"   db:"expires_at"`
	CompletedAt *time.Time `json:"completed_at" db:"completed_at"`
}

type RefreshToken struct {
	ID        uuid.UUID  `json:"id"         db:"id"`
	UserID    uuid.UUID  `json:"user_id"    db:"user_id"`
	TokenHash string     `json:"-"          db:"token_hash"`
	ExpiresAt time.Time  `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	RevokedAt *time.Time `json:"-"          db:"revoked_at"`
}

type Invitation struct {
	ID          uuid.UUID  `json:"id"           db:"id"`
	Email       string     `json:"email"        db:"email"`
	Role        Role       `json:"role"         db:"role"`
	TokenHash   string     `json:"-"            db:"token_hash"`
	InvitedBy   uuid.UUID  `json:"invited_by"   db:"invited_by"`
	ExpiresAt   time.Time  `json:"expires_at"   db:"expires_at"`
	AcceptedAt  *time.Time `json:"accepted_at"  db:"accepted_at"`
	CreatedAt   time.Time  `json:"created_at"   db:"created_at"`
}

type MFASecret struct {
	ID        uuid.UUID `json:"id"         db:"id"`
	UserID    uuid.UUID `json:"user_id"    db:"user_id"`
	Secret    string    `json:"-"          db:"secret"`
	Enabled   bool      `json:"enabled"    db:"enabled"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type RecoveryCode struct {
	ID        uuid.UUID  `json:"id"         db:"id"`
	UserID    uuid.UUID  `json:"user_id"    db:"user_id"`
	CodeHash  string     `json:"-"          db:"code_hash"`
	UsedAt    *time.Time `json:"-"          db:"used_at"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
}
