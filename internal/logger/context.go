package logger

import (
	"context"
	"log/slog"
)

type ctxKey string

const requestIDKey ctxKey = "request_id"

// WithRequestID сохраняет единый request_id в контекст запроса.
func WithRequestID(ctx context.Context, reqID string) context.Context {
	return context.WithValue(ctx, requestIDKey, reqID)
}

// RequestIDFromContext возвращает request_id из контекста или пустую строку.
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// FromContext возвращает логгер, обогащённый полем request_id из контекста.
func FromContext(ctx context.Context, base *slog.Logger) *slog.Logger {
	if base == nil {
		base = slog.Default()
	}
	if reqID := RequestIDFromContext(ctx); reqID != "" {
		return base.With(slog.String("request_id", reqID))
	}
	return base
}
