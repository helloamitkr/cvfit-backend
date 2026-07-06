package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/helloamitkr/cvfit-backend/service"
	apperrors "github.com/helloamitkr/cvfit-tools/errors"
)

type ctxKey string

const ctxUserIDKey ctxKey = "user_id"

// UserIDFromCtx extracts the authenticated user ID from the request context.
func UserIDFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(ctxUserIDKey).(string); ok {
		return v
	}
	return ""
}

// RequireAuth rejects requests that carry no valid JWT.
func (d *Deps) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := d.extractClaims(r)
		if err != nil || claims == nil {
			writeError(w, apperrors.New(http.StatusUnauthorized, "authentication required"))
			return
		}
		ctx := context.WithValue(r.Context(), ctxUserIDKey, claims.UserID)
		next(w, r.WithContext(ctx))
	}
}

// OptionalAuth injects the user ID when a valid JWT is present; proceeds without it otherwise.
func (d *Deps) OptionalAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if claims, err := d.extractClaims(r); err == nil && claims != nil {
			ctx := context.WithValue(r.Context(), ctxUserIDKey, claims.UserID)
			r = r.WithContext(ctx)
		}
		next(w, r)
	}
}

func (d *Deps) extractClaims(r *http.Request) (*service.AuthClaims, error) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return nil, nil
	}
	return d.Svc.ParseToken(strings.TrimPrefix(auth, "Bearer "))
}

// RequireAdmin rejects requests from non-admin users.
func (d *Deps) RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return d.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		userID := UserIDFromCtx(r.Context())
		if !d.Svc.IsAdmin(r.Context(), userID) {
			writeError(w, apperrors.New(http.StatusForbidden, "admin access required"))
			return
		}
		next(w, r)
	})
}

// CORS wraps a handler with permissive CORS headers.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,PATCH,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Request-ID")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
