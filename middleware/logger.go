package middleware

import (
	"context"

	"go.uber.org/zap"
)

type loggerKey struct{}

// FromContext extracts the logger with correlation ID from context.
// Falls back to a default logger if not found.
func FromContext(ctx context.Context, fallback *zap.Logger) *zap.Logger {
	if l, ok := ctx.Value(loggerKey{}).(*zap.Logger); ok {
		return l
	}
	return fallback
}

// InjectLogger injects a logger into the context.
func InjectLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}
