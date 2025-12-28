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

// WithContext returns a new context with the logger attached
func (l *Logger) WithContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

// FromContext extracts the logger from context
// Returns the logger with all previously injected fields
// If no logger is found, returns a default logger
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

// ContextWithFields creates a new context with additional fields added to the logger
// This is useful for adding request-scoped fields like request_id, user_id
func ContextWithFields(ctx context.Context, fields Fields) context.Context {
	l := FromContext(ctx).WithFields(fields)
	return l.WithContext(ctx)
}

// ContextWithField creates a new context with a single additional field
func ContextWithField(ctx context.Context, key string, value interface{}) context.Context {
	l := FromContext(ctx).WithField(key, value)
	return l.WithContext(ctx)
}

// SetDefaultLogger sets the default logger used when no logger is found in context
func SetDefaultLogger(l *Logger) {
	if l != nil {
		defaultLoggerMu.Lock()
		defaultLogger = l
		defaultLoggerMu.Unlock()
	}
}
