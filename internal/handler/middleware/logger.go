package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"payments-engine/internal/metrics"
)

type responseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytes += n
	return n, err
}

func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			rw := &responseWriter{
				ResponseWriter: w,
				status:         http.StatusOK,
			}

			metrics.HTTPRequestsInFlight.Inc()
			defer metrics.HTTPRequestsInFlight.Dec()

			next.ServeHTTP(rw, r)

			duration := time.Since(start)

			metrics.HTTPRequestsTotal.WithLabelValues(
				r.Method,
				r.URL.Path,
				fmt.Sprintf("%d", rw.status),
			).Inc()

			metrics.HTTPRequestDuration.WithLabelValues(
				r.Method,
				r.URL.Path,
			).Observe(duration.Seconds())

			logger.InfoContext(r.Context(), "request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.status,
				"bytes", rw.bytes,
				"duration_ms", duration.Milliseconds(),
				"request_id", GetRequestID(r.Context()),
				"ip", r.RemoteAddr,
			)
		})
	}
}
