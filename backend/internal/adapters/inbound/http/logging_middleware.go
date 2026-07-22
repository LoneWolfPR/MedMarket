package http

import (
	"log/slog"
	"net/http"
	"time"
)

// statusRecorder is a custom ResponseWriter so we can override the
// WriteHeader method
type statusRecorder struct {
	http.ResponseWriter
	status int
}

// WriteHeader is an override so we can store the status code for logging
// in the middleware
func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func newLoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{
				ResponseWriter: w,
				status:         http.StatusOK,
			}
			next.ServeHTTP(rec, r)
			runTime := time.Since(start)
			logArgs := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"duration", runTime,
			}
			if rec.status >= 500 {
				// Some sort of server error. Log at warn level
				logger.WarnContext(r.Context(), "request handled", logArgs...)
			} else {
				logger.InfoContext(r.Context(), "request handled", logArgs...)
			}
		})
	}
}
