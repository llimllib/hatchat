package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// statusRecorder is a wrapper around http.ResponseWriter that captures the response code.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

// newStatusRecorder creates a new statusRecorder that wraps the given http.ResponseWriter.
func newStatusRecorder(w http.ResponseWriter) *statusRecorder {
	return &statusRecorder{
		ResponseWriter: w,
		status:         http.StatusOK,
	}
}

// WriteHeader records the response code and calls the underlying http.ResponseWriter's WriteHeader method.
func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

func RequestLogMiddleware(logger *slog.Logger) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(res http.ResponseWriter, req *http.Request) {
			resw := newStatusRecorder(res)

			start := time.Now()

			next(resw, req)

			if resw.status < 400 {
				logger.Info(req.URL.Path,
					"status", resw.status,
					"clientIP", req.RemoteAddr,
					"user-agent", req.UserAgent(),
					"referer", req.Referer(),
					"method", req.Method,
					"host", req.Host,
					"duration", time.Since(start),
					"requestID", GetRequestID(req.Context()),
				)
			} else {
				logger.Error(req.URL.Path,
					"status", resw.status,
					"clientIP", req.RemoteAddr,
					"user-agent", req.UserAgent(),
					"referer", req.Referer(),
					"method", req.Method,
					"host", req.Host,
					"duration", time.Since(start),
					"requestID", GetRequestID(req.Context()),
				)
			}
		}
	}
}
