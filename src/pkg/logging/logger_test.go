package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, "info")

	logger.Info("test message")

	if !strings.Contains(buf.String(), "test message") {
		t.Errorf("expected log to contain 'test message', got: %s", buf.String())
	}
}

func TestLogger_Component(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, "info")
	compLogger := logger.Component("mycomponent")

	compLogger.Info("component test")

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log: %v", err)
	}

	if logEntry["component"] != "mycomponent" {
		t.Errorf("expected component 'mycomponent', got: %v", logEntry["component"])
	}
}

func TestLogger_With(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, "info")
	withLogger := logger.With("request_id", "abc123")

	withLogger.Info("with test")

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log: %v", err)
	}

	if logEntry["request_id"] != "abc123" {
		t.Errorf("expected request_id 'abc123', got: %v", logEntry["request_id"])
	}
}

func TestLogger_Levels(t *testing.T) {
	tests := []struct {
		name     string
		logFunc  func(*Logger)
		level    string
		wantLog  bool
	}{
		{"debug at info level", func(l *Logger) { l.Debug("test") }, "info", false},
		{"info at info level", func(l *Logger) { l.Info("test") }, "info", true},
		{"warn at info level", func(l *Logger) { l.Warn("test") }, "info", true},
		{"error at info level", func(l *Logger) { l.Error("test") }, "info", true},
		{"debug at debug level", func(l *Logger) { l.Debug("test") }, "debug", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := New(&buf, tt.level)
			tt.logFunc(logger)

			hasLog := buf.Len() > 0
			if hasLog != tt.wantLog {
				t.Errorf("expected hasLog=%v, got=%v", tt.wantLog, hasLog)
			}
		})
	}
}

func TestLogger_Fields(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, "info")

	logger.Info("with fields",
		"string_field", "value",
		"int_field", 42,
		"float_field", 3.14,
		"bool_field", true,
	)

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log: %v", err)
	}

	if logEntry["string_field"] != "value" {
		t.Errorf("expected string_field 'value', got: %v", logEntry["string_field"])
	}
	if logEntry["int_field"] != float64(42) { // JSON numbers are float64
		t.Errorf("expected int_field 42, got: %v", logEntry["int_field"])
	}
	if logEntry["float_field"] != 3.14 {
		t.Errorf("expected float_field 3.14, got: %v", logEntry["float_field"])
	}
	if logEntry["bool_field"] != true {
		t.Errorf("expected bool_field true, got: %v", logEntry["bool_field"])
	}
}

func TestSamplingLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, "info")
	sampledLogger := logger.WithSampling(10) // Log 1 in 10

	// Log 100 messages
	for i := 0; i < 100; i++ {
		sampledLogger.InfoSampled("sampled message")
	}

	// Should have roughly 10 messages (with some variance)
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) < 5 || len(lines) > 20 {
		t.Errorf("expected roughly 10 log lines, got: %d", len(lines))
	}
}

func TestNop(t *testing.T) {
	logger := Nop()
	// Should not panic
	logger.Info("this should be discarded")
	logger.Error("this too")
	logger.Component("test").Debug("and this")
}

func BenchmarkLogger_Info(b *testing.B) {
	var buf bytes.Buffer
	logger := New(&buf, "info")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message", "iteration", i)
	}
}

func BenchmarkLogger_InfoWithComponent(b *testing.B) {
	var buf bytes.Buffer
	logger := New(&buf, "info").Component("benchmark")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message", "iteration", i)
	}
}
