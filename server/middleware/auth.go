package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/xomodels"
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

			session, err := xomodels.SessionByID(context.Background(), db, cookie.Value)
			if err != nil {
				logger.Error("Error finding session", "err", err)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Set the username in the request context for the next handler
			ctx := context.WithValue(r.Context(), UsernameKey, session.Username)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
	}
}
