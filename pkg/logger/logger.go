package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
)

type Format string

const (
	FormatJSON Format = "json"
	FormatText Format = "text"
)

type contextKey string

const (
	traceIDKey contextKey = "trace_id"
	userIDKey  contextKey = "user_id"
)

// WithTraceID injects trace_id into context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

// WithUserID injects user_id into context
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// Config defines logger configuration
type Config struct {
	Level  slog.Level
	Writer io.Writer
	Format Format
}

// Logger wraps standard log/slog with context extraction
type Logger struct {
	inner *slog.Logger
}

// New creates a new structured Logger using standard library slog
func New(cfg Config) *Logger {
	writer := cfg.Writer
	if writer == nil {
		writer = os.Stdout
	}

	opts := &slog.HandlerOptions{
		Level: cfg.Level,
	}

	var handler slog.Handler
	if cfg.Format == FormatText {
		handler = slog.NewTextHandler(writer, opts)
	} else {
		handler = slog.NewJSONHandler(writer, opts)
	}

	return &Logger{
		inner: slog.New(handler),
	}
}

func (l *Logger) logWithContext(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
	if !l.inner.Enabled(ctx, level) {
		return
	}

	var allAttrs []slog.Attr

	if tid, ok := ctx.Value(traceIDKey).(string); ok && tid != "" {
		allAttrs = append(allAttrs, slog.String(string(traceIDKey), tid))
	}
	if uid, ok := ctx.Value(userIDKey).(string); ok && uid != "" {
		allAttrs = append(allAttrs, slog.String(string(userIDKey), uid))
	}

	allAttrs = append(allAttrs, attrs...)
	l.inner.LogAttrs(ctx, level, msg, allAttrs...)
}

// Debug logs at DEBUG level
func (l *Logger) Debug(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.logWithContext(ctx, slog.LevelDebug, msg, attrs...)
}

// Info logs at INFO level
func (l *Logger) Info(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.logWithContext(ctx, slog.LevelInfo, msg, attrs...)
}

// Warn logs at WARN level
func (l *Logger) Warn(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.logWithContext(ctx, slog.LevelWarn, msg, attrs...)
}

// Error logs at ERROR level
func (l *Logger) Error(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.logWithContext(ctx, slog.LevelError, msg, attrs...)
}
