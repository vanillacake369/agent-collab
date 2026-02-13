// Package logging provides structured logging using zerolog.
package logging

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

// Logger wraps zerolog with application-specific methods.
type Logger struct {
	zl zerolog.Logger
}

// New creates a new logger with the specified level.
// Valid levels: debug, info, warn, error, fatal, panic, trace
func New(w io.Writer, level string) *Logger {
	if w == nil {
		w = os.Stdout
	}

	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	zl := zerolog.New(w).
		Level(lvl).
		With().
		Timestamp().
		Logger()

	return &Logger{zl: zl}
}

// NewConsole creates a logger with human-readable console output.
func NewConsole(level string) *Logger {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}

	zl := zerolog.New(output).
		Level(lvl).
		With().
		Timestamp().
		Logger()

	return &Logger{zl: zl}
}

// Component creates a sub-logger for a specific component.
func (l *Logger) Component(name string) *Logger {
	return &Logger{
		zl: l.zl.With().Str("component", name).Logger(),
	}
}

// With adds fields to the logger context.
func (l *Logger) With(key string, value interface{}) *Logger {
	return &Logger{
		zl: l.zl.With().Interface(key, value).Logger(),
	}
}

// Info logs at info level.
func (l *Logger) Info(msg string, fields ...interface{}) {
	event := l.zl.Info()
	l.addFields(event, fields...)
	event.Msg(msg)
}

// Warn logs at warn level.
func (l *Logger) Warn(msg string, fields ...interface{}) {
	event := l.zl.Warn()
	l.addFields(event, fields...)
	event.Msg(msg)
}

// Error logs at error level.
func (l *Logger) Error(msg string, fields ...interface{}) {
	event := l.zl.Error()
	l.addFields(event, fields...)
	event.Msg(msg)
}

// Debug logs at debug level.
func (l *Logger) Debug(msg string, fields ...interface{}) {
	event := l.zl.Debug()
	l.addFields(event, fields...)
	event.Msg(msg)
}

// Fatal logs at fatal level then exits.
func (l *Logger) Fatal(msg string, fields ...interface{}) {
	event := l.zl.Fatal()
	l.addFields(event, fields...)
	event.Msg(msg)
}

// addFields adds key-value pairs to log event.
// Fields should be provided as key, value, key, value, ...
func (l *Logger) addFields(event *zerolog.Event, fields ...interface{}) {
	for i := 0; i < len(fields)-1; i += 2 {
		key, ok := fields[i].(string)
		if !ok {
			continue
		}

		value := fields[i+1]
		switch v := value.(type) {
		case string:
			event.Str(key, v)
		case int:
			event.Int(key, v)
		case int64:
			event.Int64(key, v)
		case float64:
			event.Float64(key, v)
		case bool:
			event.Bool(key, v)
		case error:
			event.Err(v)
		case time.Duration:
			event.Dur(key, v)
		case time.Time:
			event.Time(key, v)
		default:
			event.Interface(key, v)
		}
	}
}

// SamplingLogger wraps Logger with sampling capability for hot paths.
type SamplingLogger struct {
	*Logger
	sampler *zerolog.BasicSampler
}

// WithSampling creates a sampling logger that only logs N out of every M messages.
// rate is the sampling rate (1 = log everything, 10 = log 1 in 10).
func (l *Logger) WithSampling(rate uint32) *SamplingLogger {
	sampler := &zerolog.BasicSampler{N: rate}
	return &SamplingLogger{
		Logger:  l,
		sampler: sampler,
	}
}

// Sample returns true if this message should be logged based on sampling rate.
func (sl *SamplingLogger) Sample() bool {
	return sl.sampler.Sample(zerolog.InfoLevel)
}

// InfoSampled logs at info level with sampling.
func (sl *SamplingLogger) InfoSampled(msg string, fields ...interface{}) {
	if sl.Sample() {
		sl.Info(msg, fields...)
	}
}

// DebugSampled logs at debug level with sampling.
func (sl *SamplingLogger) DebugSampled(msg string, fields ...interface{}) {
	if sl.Sample() {
		sl.Debug(msg, fields...)
	}
}

// Nop returns a no-op logger that discards all output.
func Nop() *Logger {
	return &Logger{
		zl: zerolog.Nop(),
	}
}

// Global default logger
var defaultLogger = New(os.Stdout, "info")

// Default returns the default logger.
func Default() *Logger {
	return defaultLogger
}

// SetDefault sets the default logger.
func SetDefault(l *Logger) {
	defaultLogger = l
}
