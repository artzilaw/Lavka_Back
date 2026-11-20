package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"lavka/internal/logger"

	"github.com/google/uuid"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

// AccessLogMiddleware добавляет единый X-Request-ID в контекст и ответ,
// выставляет базовые security-заголовки (защита от XSS/clickjacking)
// и пишет access log по каждому HTTP-запросу.
func AccessLogMiddleware(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			reqID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
			if reqID == "" {
				reqID = uuid.NewString()
			}

			w.Header().Set("X-Request-ID", reqID)
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			ctx := logger.WithRequestID(r.Context(), reqID)
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rec, r.WithContext(ctx))

			if log != nil {
				log.Info("http access",
					slog.String("request_id", reqID),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Int("status", rec.status),
					slog.Int64("duration_ms", time.Since(start).Milliseconds()),
					slog.String("remote_addr", r.RemoteAddr),
				)
			}
		})
	}
}
