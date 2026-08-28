package api

import (
	"net/http"

	"github.com/Ojas1804/OAADrive/internal/auth"
	"github.com/Ojas1804/OAADrive/internal/db"
	"github.com/Ojas1804/OAADrive/internal/storage"
)

// Server holds the dependencies shared by all HTTP handlers.
type Server struct {
	Users     *db.UserStore
	Files     *db.FileStore
	Audit     *db.AuditStore
	Storage   *storage.Client
	JWTSecret string
}

func NewServer(users *db.UserStore, files *db.FileStore, audit *db.AuditStore, store *storage.Client, jwtSecret string) *Server {
	return &Server{Users: users, Files: files, Audit: audit, Storage: store, JWTSecret: jwtSecret}
}

// Routes builds the full HTTP handler, matching docs/API_CONTRACT.md.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", s.handleHealthz)

	mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/v1/auth/logout", auth.RequireAuth(s.handleLogout))

	mux.HandleFunc("POST /api/v1/files/upload", auth.RequireAuth(s.handleUploadFile))
	mux.HandleFunc("POST /api/v1/files/presign-upload", auth.RequireAuth(s.handlePresignUpload))
	mux.HandleFunc("GET /api/v1/files", auth.RequireAuth(s.handleListFiles))
	mux.HandleFunc("GET /api/v1/files/{id}/download", auth.RequireAuth(s.handleDownloadFile))
	mux.HandleFunc("DELETE /api/v1/files/{id}", auth.RequireAuth(s.handleDeleteFile))

	mux.HandleFunc("GET /api/v1/admin/users", auth.RequireAdmin(s.handleListUsers))
	mux.HandleFunc("POST /api/v1/admin/users", auth.RequireAdmin(s.handleCreateUser))
	mux.HandleFunc("PATCH /api/v1/admin/users/{id}/quota", auth.RequireAdmin(s.handleUpdateQuota))
	mux.HandleFunc("DELETE /api/v1/admin/users/{id}", auth.RequireAdmin(s.handleDeleteUser))
	mux.HandleFunc("GET /api/v1/admin/storage-usage", auth.RequireAdmin(s.handleStorageUsage))

	return auth.Middleware(s.JWTSecret)(mux)
}
