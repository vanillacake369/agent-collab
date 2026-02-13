package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// BDD-style tests for logging package
// Feature: Structured Logging System
// As a developer
// I want a structured logging system
// So that I can track application behavior with contextual information

// Scenario: Creating a new logger with default configuration
func TestFeature_StructuredLogging_Scenario_CreateLogger(t *testing.T) {
	t.Run("Given a writer and log level", func(t *testing.T) {
		var buf bytes.Buffer
		level := "info"

		t.Run("When I create a new logger", func(t *testing.T) {
			logger := New(&buf, level)

			t.Run("Then the logger should not be nil", func(t *testing.T) {
				if logger == nil {
					t.Fatal("logger should not be nil")
				}
			})

			t.Run("And it should be ready to log messages", func(t *testing.T) {
				logger.Info("test")
				if buf.Len() == 0 {
					t.Error("logger should write to buffer")
				}
			})
		})
	})
}

// Scenario: Logging with different severity levels
func TestFeature_StructuredLogging_Scenario_LogLevels(t *testing.T) {
	t.Run("Given a logger configured at INFO level", func(t *testing.T) {
		var buf bytes.Buffer
		logger := New(&buf, "info")

		t.Run("When I log a DEBUG message", func(t *testing.T) {
			buf.Reset()
			logger.Debug("debug message")

			t.Run("Then the message should NOT be logged", func(t *testing.T) {
				if buf.Len() > 0 {
					t.Error("debug message should not be logged at info level")
				}
			})
		})

		t.Run("When I log an INFO message", func(t *testing.T) {
			buf.Reset()
			logger.Info("info message")

			t.Run("Then the message should be logged", func(t *testing.T) {
				if buf.Len() == 0 {
					t.Error("info message should be logged")
				}
			})

			t.Run("And the log entry should contain the message", func(t *testing.T) {
				if !strings.Contains(buf.String(), "info message") {
					t.Error("log should contain the message")
				}
			})
		})

		t.Run("When I log a WARN message", func(t *testing.T) {
			buf.Reset()
			logger.Warn("warn message")

			t.Run("Then the message should be logged", func(t *testing.T) {
				if buf.Len() == 0 {
					t.Error("warn message should be logged")
				}
			})
		})

		t.Run("When I log an ERROR message", func(t *testing.T) {
			buf.Reset()
			logger.Error("error message")

			t.Run("Then the message should be logged", func(t *testing.T) {
				if buf.Len() == 0 {
					t.Error("error message should be logged")
				}
			})
		})
	})
}

// Scenario: Adding component context to logs
func TestFeature_StructuredLogging_Scenario_ComponentContext(t *testing.T) {
	t.Run("Given a base logger", func(t *testing.T) {
		var buf bytes.Buffer
		logger := New(&buf, "info")

		t.Run("When I create a component logger", func(t *testing.T) {
			compLogger := logger.Component("database")

			t.Run("And I log a message", func(t *testing.T) {
				compLogger.Info("connection established")

				t.Run("Then the log should contain the component name", func(t *testing.T) {
					var entry map[string]interface{}
					if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
						t.Fatalf("failed to parse JSON: %v", err)
					}

					if entry["component"] != "database" {
						t.Errorf("expected component 'database', got %v", entry["component"])
					}
				})
			})
		})
	})
}

// Scenario: Adding structured fields to logs
func TestFeature_StructuredLogging_Scenario_StructuredFields(t *testing.T) {
	t.Run("Given a logger", func(t *testing.T) {
		var buf bytes.Buffer
		logger := New(&buf, "info")

		t.Run("When I log with key-value pairs", func(t *testing.T) {
			logger.Info("user action",
				"user_id", "user-123",
				"action", "login",
				"duration_ms", 45,
			)

			t.Run("Then all fields should appear in the log entry", func(t *testing.T) {
				var entry map[string]interface{}
				if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
					t.Fatalf("failed to parse JSON: %v", err)
				}

				if entry["user_id"] != "user-123" {
					t.Errorf("expected user_id 'user-123', got %v", entry["user_id"])
				}
				if entry["action"] != "login" {
					t.Errorf("expected action 'login', got %v", entry["action"])
				}
				if entry["duration_ms"] != float64(45) {
					t.Errorf("expected duration_ms 45, got %v", entry["duration_ms"])
				}
			})
		})
	})
}

// Scenario: Creating a logger with persistent fields
func TestFeature_StructuredLogging_Scenario_PersistentFields(t *testing.T) {
	t.Run("Given a logger with a persistent field", func(t *testing.T) {
		var buf bytes.Buffer
		logger := New(&buf, "info").With("request_id", "req-456")

		t.Run("When I log multiple messages", func(t *testing.T) {
			logger.Info("first message")
			first := buf.String()
			buf.Reset()

			logger.Info("second message")
			second := buf.String()

			t.Run("Then each message should contain the persistent field", func(t *testing.T) {
				if !strings.Contains(first, "req-456") {
					t.Error("first message should contain request_id")
				}
				if !strings.Contains(second, "req-456") {
					t.Error("second message should contain request_id")
				}
			})
		})
	})
}

// Scenario: Using a no-op logger for testing
func TestFeature_StructuredLogging_Scenario_NopLogger(t *testing.T) {
	t.Run("Given a Nop logger", func(t *testing.T) {
		logger := Nop()

		t.Run("When I log messages", func(t *testing.T) {
			// These should not panic
			logger.Info("info message")
			logger.Error("error message")
			logger.Debug("debug message")
			logger.Warn("warn message")

			t.Run("Then nothing should happen (no panic)", func(t *testing.T) {
				// If we reach here, test passes
			})
		})

		t.Run("When I create component and contextual loggers", func(t *testing.T) {
			compLogger := logger.Component("test")
			withLogger := logger.With("key", "value")

			t.Run("Then they should also work without panic", func(t *testing.T) {
				compLogger.Info("component message")
				withLogger.Info("with message")
			})
		})
	})
}

// Scenario: Sampled logging for high-volume events
func TestFeature_StructuredLogging_Scenario_SampledLogging(t *testing.T) {
	t.Run("Given a sampled logger with rate 10", func(t *testing.T) {
		var buf bytes.Buffer
		logger := New(&buf, "info").WithSampling(10) // 1 in 10

		t.Run("When I log 100 messages", func(t *testing.T) {
			for i := 0; i < 100; i++ {
				logger.InfoSampled("high frequency event")
			}

			t.Run("Then approximately 10% should be logged", func(t *testing.T) {
				lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
				count := len(lines)

				// Allow variance: should be roughly 10 (±10)
				if count < 3 || count > 25 {
					t.Errorf("expected ~10 logs, got %d", count)
				}
			})
		})
	})
}

// Scenario: JSON output format
func TestFeature_StructuredLogging_Scenario_JSONOutput(t *testing.T) {
	t.Run("Given a logger", func(t *testing.T) {
		var buf bytes.Buffer
		logger := New(&buf, "info")

		t.Run("When I log a message", func(t *testing.T) {
			logger.Info("test message", "key", "value")

			t.Run("Then the output should be valid JSON", func(t *testing.T) {
				var entry map[string]interface{}
				if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
					t.Fatalf("output is not valid JSON: %v", err)
				}
			})

			t.Run("And it should contain a timestamp", func(t *testing.T) {
				var entry map[string]interface{}
				json.Unmarshal(buf.Bytes(), &entry)
				if _, ok := entry["time"]; !ok {
					t.Error("log entry should contain timestamp")
				}
			})

			t.Run("And it should contain the log level", func(t *testing.T) {
				var entry map[string]interface{}
				json.Unmarshal(buf.Bytes(), &entry)
				if entry["level"] != "info" {
					t.Errorf("expected level 'info', got %v", entry["level"])
				}
			})

			t.Run("And it should contain the message", func(t *testing.T) {
				var entry map[string]interface{}
				json.Unmarshal(buf.Bytes(), &entry)
				if entry["message"] != "test message" {
					t.Errorf("expected message 'test message', got %v", entry["message"])
				}
			})
		})
	})
}

// Scenario: Invalid log level defaults to INFO
func TestFeature_StructuredLogging_Scenario_InvalidLogLevel(t *testing.T) {
	t.Run("Given an invalid log level", func(t *testing.T) {
		var buf bytes.Buffer

		t.Run("When I create a logger", func(t *testing.T) {
			logger := New(&buf, "invalid-level")

			t.Run("Then it should default to INFO level", func(t *testing.T) {
				// Debug should not log (because we're at info)
				logger.Debug("debug")
				if buf.Len() > 0 {
					t.Error("debug should not log at default info level")
				}

				// Info should log
				logger.Info("info")
				if buf.Len() == 0 {
					t.Error("info should log at default info level")
				}
			})
		})
	})
}
