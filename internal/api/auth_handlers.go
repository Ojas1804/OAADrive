package api

import (
	"net/http"
	"time"

	goauth "github.com/Ojas1804/OAADrive/internal/auth"
)

const sessionTTL = 24 * time.Hour

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userResponse struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// POST /api/v1/auth/login
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := s.Users.GetByEmail(r.Context(), req.Email)
	if err != nil || !goauth.VerifyPassword(user.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := goauth.GenerateToken(s.JWTSecret, user.ID, user.Role, sessionTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create session")
		return
	}
	goauth.SetSessionCookie(w, token, int(sessionTTL.Seconds()))

	_ = s.Audit.Log(r.Context(), &user.ID, "login", user.Email)

	writeJSON(w, http.StatusOK, map[string]any{
		"user": userResponse{ID: user.ID, Name: user.Name, Email: user.Email, Role: user.Role},
	})
}

// POST /api/v1/auth/logout
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	goauth.ClearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}
