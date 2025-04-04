package logging

import (
	"context"
	"io"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ZapLogger implements the Logger interface using uber-go/zap
type ZapLogger struct {
	logger *zap.Logger
	fields []Field
	ctx    context.Context
}

// NewZapLogger creates a new Logger implementation backed by zap
func NewZapLogger(options ...ZapOption) Logger {
	// Default config
	config := zap.NewProductionConfig()
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.OutputPaths = []string{"stdout"}
	config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)

	// Apply options
	opts := defaultZapOptions()
	for _, opt := range options {
		opt(opts)
	}

	// Apply options to config
	if opts.development {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}

	if opts.level != nil {
		config.Level = zap.NewAtomicLevelAt(*opts.level)
	}

	if opts.outputPaths != nil && len(opts.outputPaths) > 0 {
		config.OutputPaths = opts.outputPaths
	}

	// Create logger
	logger, err := config.Build(
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	if err != nil {
		// Fall back to default logger if zap logger creation fails
		return NewLogger()
	}

	return &ZapLogger{
		logger: logger,
	}
}

// ZapOption defines a function that can configure a zap logger
type ZapOption func(*zapOptions)

// zapOptions holds configuration for the zap logger
type zapOptions struct {
	development bool
	level       *zapcore.Level
	outputPaths []string
}

// defaultZapOptions returns default options for zap logger
func defaultZapOptions() *zapOptions {
	return &zapOptions{
		development: false,
		outputPaths: []string{"stdout"},
	}
}

// WithDevelopmentMode enables development mode with more verbose logging
func WithDevelopmentMode() ZapOption {
	return func(opts *zapOptions) {
		opts.development = true
	}
}

// WithDebugLevel sets the log level to debug
func WithDebugLevel() ZapOption {
	return func(opts *zapOptions) {
		level := zapcore.DebugLevel
		opts.level = &level
	}
}

// WithLogLevel sets a specific log level
func WithLogLevel(level Level) ZapOption {
	return func(opts *zapOptions) {
		var zapLevel zapcore.Level
		switch level {
		case DEBUG:
			zapLevel = zapcore.DebugLevel
		case INFO:
			zapLevel = zapcore.InfoLevel
		case WARN:
			zapLevel = zapcore.WarnLevel
		case ERROR:
			zapLevel = zapcore.ErrorLevel
		default:
			zapLevel = zapcore.InfoLevel
		}
		opts.level = &zapLevel
	}
}

// WithOutputPaths sets output paths for the logger
func WithOutputPaths(paths ...string) ZapOption {
	return func(opts *zapOptions) {
		opts.outputPaths = paths
	}
}

// Debug implements Logger interface
func (l *ZapLogger) Debug(msg string, fields ...Field) {
	if ce := l.logger.Check(zapcore.DebugLevel, msg); ce != nil {
		ce.Write(l.convertFields(fields...)...)
	}
}

// Info implements Logger interface
func (l *ZapLogger) Info(msg string, fields ...Field) {
	if ce := l.logger.Check(zapcore.InfoLevel, msg); ce != nil {
		ce.Write(l.convertFields(fields...)...)
	}
}

// Warn implements Logger interface
func (l *ZapLogger) Warn(msg string, fields ...Field) {
	if ce := l.logger.Check(zapcore.WarnLevel, msg); ce != nil {
		ce.Write(l.convertFields(fields...)...)
	}
}

// Error implements Logger interface
func (l *ZapLogger) Error(msg string, fields ...Field) {
	if ce := l.logger.Check(zapcore.ErrorLevel, msg); ce != nil {
		ce.Write(l.convertFields(fields...)...)
	}
}

// WithContext implements Logger interface
func (l *ZapLogger) WithContext(ctx context.Context) Logger {
	newLogger := *l
	newLogger.ctx = ctx

	// Extract trace ID if present and add it as a field
	if traceID, ok := ctx.Value("trace_id").(string); ok {
		newLogger.fields = append(newLogger.fields, Field{Key: "trace_id", Value: traceID})
	}

	return &newLogger
}

// WithFields implements Logger interface
func (l *ZapLogger) WithFields(fields ...Field) Logger {
	newLogger := *l
	newLogger.fields = make([]Field, len(l.fields)+len(fields))
	copy(newLogger.fields, l.fields)
	copy(newLogger.fields[len(l.fields):], fields)
	return &newLogger
}

// SetLevel implements Logger interface
func (l *ZapLogger) SetLevel(level Level) {
	// Not directly possible with zap as its level is atomic
	// We'll just log a message that this operation is not supported
	l.logger.Info("SetLevel operation not fully supported with zap logger")
}

// SetOutput implements Logger interface
func (l *ZapLogger) SetOutput(w io.Writer) {
	// Create a new zapcore.Core with the specified output
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Get current level enabler
	atom := zap.NewAtomicLevel()
	atom.SetLevel(l.logger.Level())

	// Create new core
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(w),
		atom,
	)

	// Replace the logger with a new one using the new core
	l.logger = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
}

// GetZapLogger returns the underlying zap.Logger
func (l *ZapLogger) GetZapLogger() *zap.Logger {
	return l.logger
}

// convertFields converts our Field type to zap.Field
func (l *ZapLogger) convertFields(fields ...Field) []zap.Field {
	// Combine fields from the logger instance and the ones passed to the method
	allFields := make([]Field, 0, len(l.fields)+len(fields))
	allFields = append(allFields, l.fields...)
	allFields = append(allFields, fields...)

	// Convert to zap.Field
	zapFields := make([]zap.Field, 0, len(allFields))
	for _, f := range allFields {
		zapFields = append(zapFields, zap.Any(f.Key, f.Value))
	}

	// Add trace ID from context if available
	if l.ctx != nil {
		if traceID, ok := l.ctx.Value("trace_id").(string); ok && traceID != "" {
			zapFields = append(zapFields, zap.String("trace_id", traceID))
		}
	}

	return zapFields
}

// Close flushes any buffered log entries
func (l *ZapLogger) Close() error {
	return l.logger.Sync()
}
