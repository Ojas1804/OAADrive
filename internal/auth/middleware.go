package auth

import (
	"context"
	"encoding/json"
	"net/http"
)

// SessionCookieName is the HttpOnly cookie holding the JWT session token,
// per docs/API_CONTRACT.md.
const SessionCookieName = "oaadrive_session"

type contextKey string

const userContextKey contextKey = "auth_user"

// AuthUser is the identity attached to a request's context after the
// session cookie has been validated.
type AuthUser struct {
	ID   int64
	Role string
}

// SetSessionCookie sets the HttpOnly session cookie on a successful login.
func SetSessionCookie(w http.ResponseWriter, token string, ttlSeconds int) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		// Secure should be true once the app is only served over HTTPS
		// (Phase 5 hardening). Left false so local/LAN HTTP setups work.
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   ttlSeconds,
	})
}

// ClearSessionCookie removes the session cookie on logout.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

// Middleware parses the session cookie, if present, and attaches the
// resulting AuthUser to the request context. It never rejects a request by
// itself; use RequireAuth/RequireAdmin on individual handlers for that.
func Middleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(SessionCookieName)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			claims, err := ParseToken(secret, cookie.Value)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			ctx := context.WithValue(r.Context(), userContextKey, &AuthUser{ID: claims.UserID, Role: claims.Role})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserFromContext returns the authenticated user, if any, attached by Middleware.
func UserFromContext(ctx context.Context) (*AuthUser, bool) {
	u, ok := ctx.Value(userContextKey).(*AuthUser)
	return u, ok
}

// RequireAuth rejects the request with 401 unless a valid session is present.
func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := UserFromContext(r.Context()); !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		next(w, r)
	}
}

// RequireAdmin rejects the request with 403 unless the session belongs to an admin.
func RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := UserFromContext(r.Context())
		if !ok || u.Role != "admin" {
			writeError(w, http.StatusForbidden, "admin access required")
			return
		}
		next(w, r)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
