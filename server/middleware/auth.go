package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/models"
)

// UserIDKey is the key to use to pull a user ID out of a context
var UserIDKey = &ContextKey{"userID"}

// GetUserID returns the user ID associated with the context or a blank string
func GetUserID(ctx context.Context) string {
	str, ok := ctx.Value(UserIDKey).(string)
	if ok {
		return str
	}
	return ""
}

func AuthMiddleware(db *db.DB, logger *slog.Logger, sessionKey string, maxAge ...time.Duration) func(http.HandlerFunc) http.HandlerFunc {
	// Default session max age: 7 days. Callers can override.
	sessionMaxAge := 7 * 24 * time.Hour
	if len(maxAge) > 0 {
		sessionMaxAge = maxAge[0]
	}

	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(sessionKey)
			if err != nil {
				handleUnauthorized(w, r, sessionKey)
				return
			}

			session, err := models.SessionByID(context.Background(), db, cookie.Value)
			if err != nil {
				logger.Debug("session not found", "err", err)
				handleUnauthorized(w, r, sessionKey)
				return
			}

			// Check if session has expired
			createdAt, err := time.Parse(time.RFC3339, session.CreatedAt)
			if err != nil {
				logger.Error("invalid session created_at", "session_id", session.ID, "err", err)
				handleUnauthorized(w, r, sessionKey)
				return
			}
			if time.Since(createdAt) > sessionMaxAge {
				logger.Debug("session expired", "session_id", session.ID, "age", time.Since(createdAt))
				// Delete expired session
				if err := session.Delete(context.Background(), db); err != nil {
					logger.Error("failed to delete expired session", "err", err)
				}
				handleUnauthorized(w, r, sessionKey)
				return
			}

			// Set the user ID in the request context for the next handler
			ctx := context.WithValue(r.Context(), UserIDKey, session.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
	}
}

// handleUnauthorized clears the session cookie and either redirects to the
// login page (for browser navigation) or returns a 401 (for API/WS requests).
func handleUnauthorized(w http.ResponseWriter, r *http.Request, sessionKey string) {
	// Clear the invalid/expired cookie
	http.SetCookie(w, &http.Cookie{
		Name:     sessionKey,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	// For API and WebSocket requests, return 401
	if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/ws") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// For page requests, redirect to login
	http.Redirect(w, r, "/", http.StatusFound)
}
