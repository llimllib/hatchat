package middleware

import (
	"log/slog"
	"net/http"
)

// RecoverMiddleware is a middleware function that recovers from any panics and
// returns a 500 Internal Server Error response.
func RecoverMiddleware(logger *slog.Logger) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("Panic occurred", "err", err)

					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte("Internal Server Error")) //nolint:errcheck
				}
			}()

			next(w, r)
		}
	}
}
