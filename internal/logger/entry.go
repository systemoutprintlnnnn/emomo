package logger

import (
	"context"
)

// Entry represents a log entry with metric fields.
// Used to record aggregatable metrics (duration_ms, count, size, etc.)
type Entry struct {
	logger *Logger
	fields Fields
}

// With creates a new Entry with the given metric fields.
// Example: logger.With(logger.Fields{"duration_ms": 1234}).Info(ctx, "Task completed")
func With(fields Fields) *Entry {
	return &Entry{
		logger: getDefaultLogger(),
		fields: fields,
	}
}

// With adds more fields to an existing Entry.
func (e *Entry) With(fields Fields) *Entry {
	merged := make(Fields, len(e.fields)+len(fields))
	for k, v := range e.fields {
		merged[k] = v
	}
	for k, v := range fields {
		merged[k] = v
	}
	return &Entry{
		logger: e.logger,
		fields: merged,
	}
}

// WithField adds a single field to the Entry.
func (e *Entry) WithField(key string, value interface{}) *Entry {
	return e.With(Fields{key: value})
}

// WithDuration adds a duration_ms field to the Entry.
func (e *Entry) WithDuration(ms int64) *Entry {
	return e.WithField(FieldDurationMs, ms)
}

// WithCount adds a count field to the Entry.
func (e *Entry) WithCount(count int) *Entry {
	return e.WithField(FieldCount, count)
}

// WithSize adds a size field to the Entry.
func (e *Entry) WithSize(size int) *Entry {
	return e.WithField(FieldSize, size)
}

// WithStatus adds a status field to the Entry.
func (e *Entry) WithStatus(status string) *Entry {
	return e.WithField(FieldStatus, status)
}

// getLogger returns the logger to use for this entry.
// If ctx is provided, extracts logger from context; otherwise uses the entry's logger.
func (e *Entry) getLogger(ctx context.Context) *Logger {
	if ctx != nil {
		return FromContext(ctx)
	}
	return e.logger
}

// Debug logs at Debug level with metric fields.
func (e *Entry) Debug(ctx context.Context, format string, args ...interface{}) {
	e.getLogger(ctx).WithFields(e.fields).Debugf(format, args...)
}

// Info logs at Info level with metric fields.
func (e *Entry) Info(ctx context.Context, format string, args ...interface{}) {
	e.getLogger(ctx).WithFields(e.fields).Infof(format, args...)
}

// Warn logs at Warn level with metric fields.
func (e *Entry) Warn(ctx context.Context, format string, args ...interface{}) {
	e.getLogger(ctx).WithFields(e.fields).Warnf(format, args...)
}

// Error logs at Error level with metric fields.
func (e *Entry) Error(ctx context.Context, format string, args ...interface{}) {
	e.getLogger(ctx).WithFields(e.fields).Errorf(format, args...)
}

// Fatal logs at Fatal level with metric fields and exits.
func (e *Entry) Fatal(ctx context.Context, format string, args ...interface{}) {
	e.getLogger(ctx).WithFields(e.fields).Fatalf(format, args...)
}
