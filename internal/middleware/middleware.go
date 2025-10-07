package middleware

import (
	"net/http"
	"time"

	"github.com/CLDWare/schoolbox-backend/pkg/logger"
)

// LoggingMiddleware logs HTTP requests
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response writer wrapper to capture status code
		wrapper := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapper, r)

		duration := time.Since(start)
		if wrapper.statusCode >= 500 {
			logger.Err(
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapper.statusCode,
				"duration", duration,
				"remote_addr", r.RemoteAddr,
			)
		} else if wrapper.statusCode >= 400 {
			logger.Warn(
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapper.statusCode,
				"duration", duration,
				"remote_addr", r.RemoteAddr,
			)
		} else {
			logger.Info(
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapper.statusCode,
				"duration", duration,
				"remote_addr", r.RemoteAddr,
			)
		}
	})
}

// CORSMiddleware handles CORS headers
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// responseWriter is a wrapper to capture the status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
