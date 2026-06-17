package auth

import "errors"

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrEmailTaken        = errors.New("email already in use")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrAccountLocked     = errors.New("account is temporarily locked")
	ErrAccountInactive   = errors.New("account is inactive")
	ErrInvalidToken      = errors.New("invalid or expired token")
	ErrAdminExists       = errors.New("admin account already exists")
	ErrForbidden         = errors.New("insufficient permissions")
)
