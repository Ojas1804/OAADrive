package api

import (
	"net/http"
	"strconv"
	"time"

	goauth "github.com/Ojas1804/OAADrive/internal/auth"
	"github.com/Ojas1804/OAADrive/internal/db"
)

// defaultQuotaBytes matches the DEFAULT on users.storage_quota_bytes in init.sql.
const defaultQuotaBytes = 5 * 1024 * 1024 * 1024 // 5 GB

type adminUserResponse struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	Email             string `json:"email"`
	Role              string `json:"role"`
	StorageQuotaBytes int64  `json:"storage_quota_bytes"`
	CreatedAt         string `json:"created_at"`
}

func toAdminUserResponse(u *db.User) adminUserResponse {
	return adminUserResponse{
		ID:                u.ID,
		Name:              u.Name,
		Email:             u.Email,
		Role:              u.Role,
		StorageQuotaBytes: u.StorageQuotaBytes,
		CreatedAt:         u.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// GET /api/v1/admin/users
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.Users.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list users")
		return
	}
	resp := make([]adminUserResponse, 0, len(users))
	for _, u := range users {
		resp = append(resp, toAdminUserResponse(u))
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": resp})
}

type createUserRequest struct {
	Name              string `json:"name"`
	Email             string `json:"email"`
	Password          string `json:"password"`
	Role              string `json:"role"`
	StorageQuotaBytes int64  `json:"storage_quota_bytes"`
}

// POST /api/v1/admin/users
func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Role != db.RoleAdmin && req.Role != db.RoleMember {
		req.Role = db.RoleMember
	}
	if req.StorageQuotaBytes <= 0 {
		req.StorageQuotaBytes = defaultQuotaBytes
	}

	hash, err := goauth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not hash password")
		return
	}

	user, err := s.Users.Create(r.Context(), req.Name, req.Email, hash, req.Role, req.StorageQuotaBytes)
	if err != nil {
		if err == db.ErrEmailTaken {
			writeError(w, http.StatusConflict, "email already taken")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not create user")
		return
	}

	if admin, ok := goauth.UserFromContext(r.Context()); ok {
		_ = s.Audit.Log(r.Context(), &admin.ID, "admin_create_user", user.Email)
	}

	writeJSON(w, http.StatusCreated, toAdminUserResponse(user))
}

type updateQuotaRequest struct {
	StorageQuotaBytes int64 `json:"storage_quota_bytes"`
}

// PATCH /api/v1/admin/users/{id}/quota
func (s *Server) handleUpdateQuota(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	var req updateQuotaRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := s.Users.UpdateQuota(r.Context(), id, req.StorageQuotaBytes)
	if err != nil {
		if err == db.ErrNotFound {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not update quota")
		return
	}

	if admin, ok := goauth.UserFromContext(r.Context()); ok {
		_ = s.Audit.Log(r.Context(), &admin.ID, "admin_update_quota", user.Email)
	}

	writeJSON(w, http.StatusOK, toAdminUserResponse(user))
}

// DELETE /api/v1/admin/users/{id}
func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if err := s.Users.Delete(r.Context(), id); err != nil {
		if err == db.ErrNotFound {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not delete user")
		return
	}

	if admin, ok := goauth.UserFromContext(r.Context()); ok {
		_ = s.Audit.Log(r.Context(), &admin.ID, "admin_delete_user", strconv.FormatInt(id, 10))
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /api/v1/admin/storage-usage
func (s *Server) handleStorageUsage(w http.ResponseWriter, r *http.Request) {
	usage, err := s.Files.StorageUsageByUser(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not compute storage usage")
		return
	}

	type usageResp struct {
		UserID     int64  `json:"user_id"`
		Name       string `json:"name"`
		UsedBytes  int64  `json:"used_bytes"`
		QuotaBytes int64  `json:"quota_bytes"`
	}
	resp := make([]usageResp, 0, len(usage))
	for _, u := range usage {
		resp = append(resp, usageResp{UserID: u.UserID, Name: u.Name, UsedBytes: u.UsedBytes, QuotaBytes: u.QuotaBytes})
	}
	writeJSON(w, http.StatusOK, map[string]any{"usage": resp})
}
