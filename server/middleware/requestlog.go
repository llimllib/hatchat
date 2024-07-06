package middleware

import (
	"bufio"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// statusRecorder is a wrapper around http.ResponseWriter that captures the
// response code.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

// newStatusRecorder creates a new statusRecorder that wraps the given
// http.ResponseWriter.
func newStatusRecorder(w http.ResponseWriter) *statusRecorder {
	return &statusRecorder{
		ResponseWriter: w,
		status:         http.StatusOK,
	}
}

// WriteHeader records the response code and calls the underlying
// http.ResponseWriter's WriteHeader method.
func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

// Hijack allows the HTTP connection to be taken over for websockets:
// https://pkg.go.dev/net/http#Hijacker
func (sr *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := sr.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("hijack not supported")
	}
	return h.Hijack()
}

func RequestLogMiddleware(logger *slog.Logger) func(string) func(http.HandlerFunc) http.HandlerFunc {
	return func(route string) func(http.HandlerFunc) http.HandlerFunc {
		return func(next http.HandlerFunc) http.HandlerFunc {
			return func(res http.ResponseWriter, req *http.Request) {
				resw := newStatusRecorder(res)

				start := time.Now()

				next(resw, req)

				if resw.status < 400 {
					logger.Info(route,
						"status", resw.status,
						"path", req.URL.Path,
						"clientIP", req.RemoteAddr,
						"user-agent", req.UserAgent(),
						"referer", req.Referer(),
						"method", req.Method,
						"host", req.Host,
						"duration", time.Since(start),
						"requestID", GetRequestID(req.Context()),
					)
				} else {
					logger.Error(route,
						"status", resw.status,
						"path", req.URL.Path,
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
}
