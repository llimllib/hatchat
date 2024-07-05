package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
)

type ContextKey struct {
	Name string
}

// RequestIDKey is the key to use to pull a request out of a context
var RequestIDKey = &ContextKey{"request"}

// GetRequestID returns the request id associated with the context or a blank string
func GetRequestID(ctx context.Context) string {
	str, ok := ctx.Value(RequestIDKey).(string)
	if ok {
		return str
	}
	return ""
}

// RequestIDMiddleware returns a middleware function that adds a request ID to
// the request context
func RequestIDMiddleware(logger *slog.Logger) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(res http.ResponseWriter, req *http.Request) {
			requestID := uuid.New().String()

			// Add the request id to the request context
			rctx := context.WithValue(req.Context(), RequestIDKey, requestID)

			next(res, req.WithContext(rctx))
		}
	}
}
