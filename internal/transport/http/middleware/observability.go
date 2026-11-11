package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/platform/logger"
	"github.com/Soyaib10/farm-fusion/internal/transport/http/response"
	"github.com/google/uuid"
)

// RequestID assigns or propagates a safe correlation ID via X-Request-ID.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := sanitizeRequestID(r.Header.Get("X-Request-ID"))
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(WithRequestID(r.Context(), id)))
	})
}

func sanitizeRequestID(raw string) string {
	if len(raw) == 0 || len(raw) > 64 {
		return ""
	}
	for _, ch := range raw {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_') {
			return ""
		}
	}
	return raw
}

// Recoverer intercepts panics, logs a stack trace, closes the connection, and returns 500 JSON.
func Recoverer(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					w.Header().Set("Connection", "close")
					err := fmt.Errorf("panic recovered: %v", rec)
					if log != nil {
						log.ErrorWithStack(err, map[string]string{
							"request_id":     RequestIDFromContext(r.Context()),
							"request_method": r.Method,
							"request_path":   r.URL.Path,
						})
					}
					response.Error(w, r, log, http.StatusInternalServerError,
						"the server encountered a problem and could not process your request")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

// RequestLogger logs structured HTTP request metadata without exposing query parameters or bodies.
func RequestLogger(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rec, r)

			if log != nil {
				log.Info("HTTP request completed", map[string]string{
					"request_id":  RequestIDFromContext(r.Context()),
					"method":      r.Method,
					"path":        r.URL.Path,
					"status":      strconv.Itoa(rec.status),
					"duration_ms": strconv.FormatInt(time.Since(start).Milliseconds(), 10),
				})
			}
		})
	}
}
