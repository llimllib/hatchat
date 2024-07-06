package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/llimllib/tinychat/server/db"
)

// UsernameKey is the key to use to pull a request out of a context
var UsernameKey = &ContextKey{"username"}

// GetUsername returns the request id associated with the context or a blank
// string
func GetUsername(ctx context.Context) string {
	str, ok := ctx.Value(UsernameKey).(string)
	if ok {
		return str
	}
	return ""
}

func AuthMiddleware(db *db.DB, logger *slog.Logger, session_key string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(session_key)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			sessionID := cookie.Value
			rows, err := db.Select(`
				SELECT username
				FROM sessions
				WHERE id = ?`, sessionID)
			if err != nil || !rows.Next() {
				logger.Error("Error finding session", "err", err)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			var username string
			err = rows.Scan(&username)
			if err != nil {
				logger.Error("Error finding session", "err", err)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Set the username in the request context for the next handler
			ctx := context.WithValue(r.Context(), UsernameKey, username)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
	}
}
