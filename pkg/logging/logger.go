// Package logging provides a structured, leveled logging system for the application.
// It defines a common logging interface that can be implemented by different
// logging backends, allowing for flexibility in how logs are processed and stored.
//
// The logging package is designed around these key principles:
//
//  1. Structured logging: Rather than simple text messages, logs include structured
//     data (fields) that can be easily parsed, filtered, and analyzed.
//
//  2. Leveled logging: Different severity levels (Debug, Info, Warn, Error) allow
//     for appropriate filtering and handling of log messages.
//
//  3. Context-aware: Logging can include context information such as trace IDs
//     to correlate logs across different parts of the system.
//
//  4. Backend-agnostic: The Logger interface can be implemented by different
//     logging backends (default JSON logger, Zap logger, etc.).
//
// Usage example:
//
//	// Create a new logger
//	logger := logging.NewLogger()
//
//	// Log at different levels with structured fields
//	logger.Debug("Connection attempt started",
//		logging.String("exchange", "bybit"),
//		logging.String("url", wsURL))
//
//	logger.Info("Market data received",
//		logging.String("symbol", "BTCUSDT"),
//		logging.Int("data_points", 24))
//
//	// Add request context (e.g., for tracking)
//	ctxLogger := logger.WithContext(ctx)
//	ctxLogger.Info("Processing request")
//
//	// Add persistent fields
//	exchangeLogger := logger.WithFields(
//		logging.String("exchange", "bybit"),
//		logging.String("component", "websocket"))
//
//	// Log errors with details
//	if err != nil {
//		exchangeLogger.Error("Connection failed",
//			logging.Error(err),
//			logging.Int("retry_count", retryCount))
//	}
//
// This package is used throughout the application to provide consistent,
// structured logging that can be easily integrated with monitoring and
// observability systems.
package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Level represents the severity level of a log message
type Level int

const (
	// Log levels
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

// String returns the string representation of a log level
func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger defines the interface for logging functionality
type Logger interface {
	// Log methods for different severity levels
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)

	// WithContext adds context information to the logger
	WithContext(ctx context.Context) Logger

	// WithFields adds fields to the logger
	WithFields(fields ...Field) Logger

	// SetLevel sets the minimum log level
	SetLevel(level Level)

	// SetOutput sets the output destination
	SetOutput(w io.Writer)
}

// Field represents a key-value pair in a log entry
type Field struct {
	Key   string
	Value interface{}
}

// logger implements the Logger interface
type logger struct {
	mu     sync.Mutex
	out    io.Writer
	level  Level
	fields []Field
	ctx    context.Context
}

// NewLogger creates a new logger with default settings
func NewLogger() Logger {
	return &logger{
		out:   os.Stdout,
		level: INFO,
	}
}

// log writes a log entry with the given level and message
func (l *logger) log(level Level, msg string, fields ...Field) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Combine base fields and additional fields
	allFields := make([]Field, 0, len(l.fields)+len(fields))
	allFields = append(allFields, l.fields...)
	allFields = append(allFields, fields...)

	// Create the log entry
	entry := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"level":     level.String(),
		"message":   msg,
	}

	// Add context information if available
	if l.ctx != nil {
		if traceID, ok := l.ctx.Value("trace_id").(string); ok {
			entry["trace_id"] = traceID
		}
	}

	// Add fields to the entry
	for _, field := range allFields {
		entry[field.Key] = field.Value
	}

	// Marshal to JSON
	data, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error marshaling log entry: %v\n", err)
		return
	}

	// Write to output
	data = append(data, '\n')
	if _, err := l.out.Write(data); err != nil {
		fmt.Fprintf(os.Stderr, "error writing log entry: %v\n", err)
	}
}

// Debug implements Logger interface
func (l *logger) Debug(msg string, fields ...Field) {
	l.log(DEBUG, msg, fields...)
}

// Info implements Logger interface
func (l *logger) Info(msg string, fields ...Field) {
	l.log(INFO, msg, fields...)
}

// Warn implements Logger interface
func (l *logger) Warn(msg string, fields ...Field) {
	l.log(WARN, msg, fields...)
}

// Error implements Logger interface
func (l *logger) Error(msg string, fields ...Field) {
	l.log(ERROR, msg, fields...)
}

// WithContext implements Logger interface
func (l *logger) WithContext(ctx context.Context) Logger {
	newLogger := *l
	newLogger.ctx = ctx
	return &newLogger
}

// WithFields implements Logger interface
func (l *logger) WithFields(fields ...Field) Logger {
	newLogger := *l
	newLogger.fields = make([]Field, len(l.fields)+len(fields))
	copy(newLogger.fields, l.fields)
	copy(newLogger.fields[len(l.fields):], fields)
	return &newLogger
}

// SetLevel implements Logger interface
func (l *logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetOutput implements Logger interface
func (l *logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.out = w
}

// Field constructors for common types
func String(key string, value string) Field {
	return Field{Key: key, Value: value}
}

func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

func Float64(key string, value float64) Field {
	return Field{Key: key, Value: value}
}

func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

func Error(err error) Field {
	return Field{Key: "error", Value: err.Error()}
}

func Duration(key string, value time.Duration) Field {
	return Field{Key: key, Value: value.String()}
}
