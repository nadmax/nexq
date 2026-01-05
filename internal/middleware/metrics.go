// Package middleware provides HTTP middleware for metrics collection.
package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nadmax/nexq/internal/metrics"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		endpoint := normalizeEndpoint(r.URL.Path)
		status := strconv.Itoa(wrapped.statusCode)

		metrics.RecordHTTPRequest(r.Method, endpoint, status, duration)
	})
}

func normalizeEndpoint(path string) string {
	switch {
	case strings.HasPrefix(path, "/api/tasks/") && !strings.Contains(path[11:], "/"):
		return "/api/tasks/:id"
	case strings.HasPrefix(path, "/api/dlq/tasks/"):
		parts := strings.Split(strings.TrimPrefix(path, "/api/dlq/tasks/"), "/")
		if len(parts) >= 2 && parts[1] == "retry" {
			return "/api/dlq/tasks/:id/retry"
		}

		return "/api/dlq/tasks/:id"
	case strings.HasPrefix(path, "/api/history/task/"):
		return "/api/history/task/:id"
	case strings.HasPrefix(path, "/api/history/type/"):
		return "/api/history/type/:type"
	default:
		return path
	}
}
