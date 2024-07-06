package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
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

func AuthMiddleware(pool *sqlitex.Pool, logger *slog.Logger, session_key string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			conn := pool.Get(r.Context())
			if conn == nil {
				panic("unable to get connection")
			}

			cookie, err := r.Cookie(session_key)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			sessionID := cookie.Value
			var username string
			err = sqlitex.Exec(conn, `
				SELECT username
				FROM sessions
				WHERE id = ?`, func(stmt *sqlite.Stmt) error {
				username = stmt.ColumnText(0)
				return nil
			}, sessionID)
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
