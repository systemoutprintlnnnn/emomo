package logger

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

// Logger wraps logrus.Entry to provide structured logging with context support.
type Logger struct {
	*logrus.Entry
}

// Config holds logger configuration.
type Config struct {
	Level       string    // debug, info, warn, error
	Format      string    // json, text
	Output      io.Writer // output destination
	ServiceName string    // service name for log tagging
}

// DefaultConfig returns sensible defaults.
// Parameters: none.
// Returns:
//   - *Config: default logger configuration.
func DefaultConfig() *Config {
	return &Config{
		Level:       "info",
		Format:      "json",
		Output:      os.Stdout,
		ServiceName: "emomo",
	}
}

// New creates a new Logger with the given configuration.
// Parameters:
//   - cfg: logger configuration; nil uses DefaultConfig.
// Returns:
//   - *Logger: initialized logger instance.
func New(cfg *Config) *Logger {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	log := logrus.New()

	// Set output
	if cfg.Output != nil {
		log.SetOutput(cfg.Output)
	} else {
		log.SetOutput(os.Stdout)
	}

	// Set log level
	level, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	log.SetLevel(level)

	// Enable caller reporting
	log.SetReportCaller(true)

	// Set formatter - JSON format as default
	if cfg.Format == "text" {
		log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
			CallerPrettyfier: callerPrettyfier,
		})
	} else {
		log.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "timestamp",
				logrus.FieldKeyLevel: "level",
				logrus.FieldKeyMsg:   "message",
			},
			CallerPrettyfier: callerPrettyfier,
		})
	}

	// Create base entry with service name
	entry := log.WithField("service", cfg.ServiceName)

	return &Logger{Entry: entry}
}

// WithFields returns a new Logger with additional fields.
// Parameters:
//   - fields: structured fields to add.
// Returns:
//   - *Logger: derived logger with fields applied.
func (l *Logger) WithFields(fields Fields) *Logger {
	return &Logger{Entry: l.Entry.WithFields(logrus.Fields(fields))}
}

// WithField returns a new Logger with a single additional field.
// Parameters:
//   - key: field key.
//   - value: field value.
// Returns:
//   - *Logger: derived logger with field applied.
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return &Logger{Entry: l.Entry.WithField(key, value)}
}

// WithError returns a new Logger with an error field.
// Parameters:
//   - err: error to attach.
// Returns:
//   - *Logger: derived logger with error field.
func (l *Logger) WithError(err error) *Logger {
	return &Logger{Entry: l.Entry.WithError(err)}
}

// callerPrettyfier simplifies caller information to show only relative path and line number
func callerPrettyfier(frame *runtime.Frame) (function string, file string) {
	// Get short function name (without package path)
	funcName := frame.Function
	if idx := strings.LastIndex(funcName, "/"); idx != -1 {
		funcName = funcName[idx+1:]
	}

	// Get short file path (only filename:line)
	fileName := filepath.Base(frame.File)

	return funcName, fileName + ":" + itoa(frame.Line)
}

// itoa converts int to string without importing strconv
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	n := len(b)
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		n--
		b[n] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		n--
		b[n] = '-'
	}
	return string(b[n:])
}
