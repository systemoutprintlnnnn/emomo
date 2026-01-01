package logger

import (
	"context"
	"sync"
)

// contextKey is a private type for context keys to avoid collisions
type contextKey struct{}

// loggerKey is the key used to store logger in context
var loggerKey = contextKey{}

// defaultLogger is used when no logger is found in context
var (
	defaultLogger   *Logger
	defaultLoggerMu sync.RWMutex
)

func init() {
	defaultLogger = New(nil)
}

// WithContext returns a new context with the logger attached.
// Parameters:
//   - ctx: existing context to wrap.
// Returns:
//   - context.Context: context containing the logger.
func (l *Logger) WithContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

// FromContext extracts the logger from context.
// Parameters:
//   - ctx: context to inspect.
// Returns:
//   - *Logger: logger with injected fields or the default logger.
func FromContext(ctx context.Context) *Logger {
	if ctx != nil {
		if l, ok := ctx.Value(loggerKey).(*Logger); ok {
			return l
		}
	}
	defaultLoggerMu.RLock()
	l := defaultLogger
	defaultLoggerMu.RUnlock()
	return l
}

// ContextWithFields creates a new context with additional fields added to the logger.
// Parameters:
//   - ctx: base context.
//   - fields: structured fields to add.
// Returns:
//   - context.Context: context containing the enriched logger.
func ContextWithFields(ctx context.Context, fields Fields) context.Context {
	l := FromContext(ctx).WithFields(fields)
	return l.WithContext(ctx)
}

// ContextWithField creates a new context with a single additional field.
// Parameters:
//   - ctx: base context.
//   - key: field key.
//   - value: field value.
// Returns:
//   - context.Context: context containing the enriched logger.
func ContextWithField(ctx context.Context, key string, value interface{}) context.Context {
	l := FromContext(ctx).WithField(key, value)
	return l.WithContext(ctx)
}

// SetDefaultLogger sets the default logger used when no logger is found in context.
// Parameters:
//   - l: logger to set as default.
// Returns: none.
func SetDefaultLogger(l *Logger) {
	if l != nil {
		defaultLoggerMu.Lock()
		defaultLogger = l
		defaultLoggerMu.Unlock()
	}
}
