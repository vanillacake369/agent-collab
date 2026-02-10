package libp2p

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestSpanStatus_String(t *testing.T) {
	tests := []struct {
		status   SpanStatus
		expected string
	}{
		{StatusUnset, "unset"},
		{StatusOK, "ok"},
		{StatusError, "error"},
		{SpanStatus(99), "unset"},
	}

	for _, tt := range tests {
		if got := tt.status.String(); got != tt.expected {
			t.Errorf("SpanStatus(%d).String() = %s, want %s", tt.status, got, tt.expected)
		}
	}
}

func TestDefaultTracerConfig(t *testing.T) {
	config := DefaultTracerConfig()

	if config.ServiceName != "agent-collab" {
		t.Errorf("ServiceName = %s, want agent-collab", config.ServiceName)
	}

	if !config.Enabled {
		t.Error("Enabled should be true")
	}

	if config.MaxCompleted != 1000 {
		t.Errorf("MaxCompleted = %d, want 1000", config.MaxCompleted)
	}
}

func TestNewTracer(t *testing.T) {
	config := DefaultTracerConfig()
	tracer := NewTracer(config)

	if tracer.serviceName != "agent-collab" {
		t.Errorf("serviceName = %s, want agent-collab", tracer.serviceName)
	}

	if !tracer.enabled {
		t.Error("Tracer should be enabled")
	}
}

func TestTracer_SetEnabled(t *testing.T) {
	tracer := NewTracer(DefaultTracerConfig())

	if !tracer.IsEnabled() {
		t.Error("Should be enabled initially")
	}

	tracer.SetEnabled(false)
	if tracer.IsEnabled() {
		t.Error("Should be disabled after SetEnabled(false)")
	}
}

func TestTracer_StartEndSpan(t *testing.T) {
	tracer := NewTracer(DefaultTracerConfig())

	span := tracer.StartSpan("test-operation")
	if span == nil {
		t.Fatal("Span should not be nil")
	}

	if span.Name != "test-operation" {
		t.Errorf("Name = %s, want test-operation", span.Name)
	}

	if span.TraceID == "" {
		t.Error("TraceID should be generated")
	}

	if span.SpanID == "" {
		t.Error("SpanID should be generated")
	}

	if span.Service != "agent-collab" {
		t.Errorf("Service = %s, want agent-collab", span.Service)
	}

	// End the span
	time.Sleep(10 * time.Millisecond)
	tracer.EndSpan(span)

	if span.EndTime.IsZero() {
		t.Error("EndTime should be set")
	}

	if span.Duration < 10*time.Millisecond {
		t.Errorf("Duration = %v, should be >= 10ms", span.Duration)
	}

	if span.Status != StatusOK {
		t.Errorf("Status = %v, want StatusOK", span.Status)
	}
}

func TestTracer_StartSpanWithParent(t *testing.T) {
	tracer := NewTracer(DefaultTracerConfig())

	parentSpan := tracer.StartSpan("parent")
	childSpan := tracer.StartSpanWithParent("child", parentSpan.TraceID, parentSpan.SpanID)

	if childSpan.TraceID != parentSpan.TraceID {
		t.Error("Child should have same TraceID as parent")
	}

	if childSpan.ParentID != parentSpan.SpanID {
		t.Error("Child ParentID should be parent SpanID")
	}

	tracer.EndSpan(childSpan)
	tracer.EndSpan(parentSpan)
}

func TestTracer_StartSpanFromContext(t *testing.T) {
	tracer := NewTracer(DefaultTracerConfig())

	// Start root span
	rootSpan, ctx := tracer.StartSpanFromContext(context.Background(), "root")
	if rootSpan == nil {
		t.Fatal("Root span should not be nil")
	}

	// Start child span from context
	childSpan, _ := tracer.StartSpanFromContext(ctx, "child")
	if childSpan == nil {
		t.Fatal("Child span should not be nil")
	}

	if childSpan.TraceID != rootSpan.TraceID {
		t.Error("Child should inherit TraceID from context")
	}

	if childSpan.ParentID != rootSpan.SpanID {
		t.Error("Child should have root as parent")
	}

	tracer.EndSpan(childSpan)
	tracer.EndSpan(rootSpan)
}

func TestTracer_EndSpanWithError(t *testing.T) {
	tracer := NewTracer(DefaultTracerConfig())

	span := tracer.StartSpan("failing-operation")
	tracer.EndSpanWithError(span, errors.New("something went wrong"))

	if span.Status != StatusError {
		t.Errorf("Status = %v, want StatusError", span.Status)
	}

	if span.Tags["error.message"] != "something went wrong" {
		t.Error("Error message should be in tags")
	}

	if len(span.Logs) != 1 {
		t.Fatal("Should have 1 log entry")
	}

	if span.Logs[0].Level != "error" {
		t.Error("Log level should be error")
	}
}

func TestTracer_Disabled(t *testing.T) {
	tracer := NewTracer(TracerConfig{Enabled: false})

	span := tracer.StartSpan("test")
	if span != nil {
		t.Error("Span should be nil when tracing is disabled")
	}

	// Should not panic
	tracer.EndSpan(span)
}

func TestTracer_GetActiveSpans(t *testing.T) {
	tracer := NewTracer(DefaultTracerConfig())

	span1 := tracer.StartSpan("op1")
	span2 := tracer.StartSpan("op2")
	span3 := tracer.StartSpan("op3")

	active := tracer.GetActiveSpans()
	if len(active) != 3 {
		t.Errorf("Expected 3 active spans, got %d", len(active))
	}

	tracer.EndSpan(span2)

	active = tracer.GetActiveSpans()
	if len(active) != 2 {
		t.Errorf("Expected 2 active spans, got %d", len(active))
	}

	tracer.EndSpan(span1)
	tracer.EndSpan(span3)
}

func TestTracer_GetCompletedSpans(t *testing.T) {
	tracer := NewTracer(DefaultTracerConfig())

	span1 := tracer.StartSpan("op1")
	span2 := tracer.StartSpan("op2")

	tracer.EndSpan(span1)

	completed := tracer.GetCompletedSpans()
	if len(completed) != 1 {
		t.Errorf("Expected 1 completed span, got %d", len(completed))
	}

	tracer.EndSpan(span2)

	completed = tracer.GetCompletedSpans()
	if len(completed) != 2 {
		t.Errorf("Expected 2 completed spans, got %d", len(completed))
	}
}

func TestTracer_GetCompletedSpansSince(t *testing.T) {
	tracer := NewTracer(DefaultTracerConfig())

	span1 := tracer.StartSpan("op1")
	tracer.EndSpan(span1)

	time.Sleep(50 * time.Millisecond)
	checkpoint := time.Now()
	time.Sleep(50 * time.Millisecond)

	span2 := tracer.StartSpan("op2")
	tracer.EndSpan(span2)

	since := tracer.GetCompletedSpansSince(checkpoint)
	if len(since) != 1 {
		t.Errorf("Expected 1 span since checkpoint, got %d", len(since))
	}
}

func TestTracer_GetTraceSpans(t *testing.T) {
	tracer := NewTracer(DefaultTracerConfig())

	span1 := tracer.StartSpan("root")
	span2 := tracer.StartSpanWithParent("child", span1.TraceID, span1.SpanID)

	// Both should be in the trace
	traceSpans := tracer.GetTraceSpans(span1.TraceID)
	if len(traceSpans) != 2 {
		t.Errorf("Expected 2 spans in trace, got %d", len(traceSpans))
	}

	tracer.EndSpan(span1)
	tracer.EndSpan(span2)

	// Should still find them in completed spans
	traceSpans = tracer.GetTraceSpans(span1.TraceID)
	if len(traceSpans) != 2 {
		t.Errorf("Expected 2 spans in trace after completion, got %d", len(traceSpans))
	}
}

func TestTracer_MaxCompleted(t *testing.T) {
	config := DefaultTracerConfig()
	config.MaxCompleted = 10
	tracer := NewTracer(config)

	// Create 15 spans
	for i := 0; i < 15; i++ {
		span := tracer.StartSpan("op")
		tracer.EndSpan(span)
	}

	// Should have removed some
	completed := tracer.GetCompletedSpans()
	if len(completed) > 10 {
		t.Errorf("Should have at most 10 completed spans, got %d", len(completed))
	}
}

func TestTracer_OnSpanComplete(t *testing.T) {
	tracer := NewTracer(DefaultTracerConfig())

	var completedSpans []*Span
	var mu sync.Mutex

	tracer.OnSpanComplete(func(span *Span) {
		mu.Lock()
		completedSpans = append(completedSpans, span)
		mu.Unlock()
	})

	span := tracer.StartSpan("test")
	tracer.EndSpan(span)

	time.Sleep(50 * time.Millisecond) // Wait for async callback

	mu.Lock()
	count := len(completedSpans)
	mu.Unlock()

	if count != 1 {
		t.Errorf("Expected 1 callback, got %d", count)
	}
}

func TestTracer_Stats(t *testing.T) {
	tracer := NewTracer(DefaultTracerConfig())

	span1 := tracer.StartSpan("active")
	span2 := tracer.StartSpan("completed")
	tracer.EndSpan(span2)

	stats := tracer.Stats()

	if stats.ActiveSpans != 1 {
		t.Errorf("ActiveSpans = %d, want 1", stats.ActiveSpans)
	}

	if stats.CompletedSpans != 1 {
		t.Errorf("CompletedSpans = %d, want 1", stats.CompletedSpans)
	}

	if stats.ServiceName != "agent-collab" {
		t.Errorf("ServiceName = %s, want agent-collab", stats.ServiceName)
	}

	tracer.EndSpan(span1)
}

func TestSpan_Methods(t *testing.T) {
	tracer := NewTracer(DefaultTracerConfig())
	span := tracer.StartSpan("test")

	// SetTag
	span.SetTag("key", "value")
	if span.Tags["key"] != "value" {
		t.Error("Tag not set")
	}

	// Log
	span.Log("info", "test message")
	if len(span.Logs) != 1 {
		t.Error("Log not added")
	}

	// LogInfo
	span.LogInfo("info message")
	if len(span.Logs) != 2 {
		t.Error("LogInfo not added")
	}

	// LogError
	span.LogError("error message")
	if len(span.Logs) != 3 {
		t.Error("LogError not added")
	}

	// SetError
	span.SetError()
	if span.Status != StatusError {
		t.Error("SetError did not set status")
	}

	tracer.EndSpan(span)
}

func TestSpan_NilSafe(t *testing.T) {
	var span *Span

	// These should not panic
	span.SetTag("key", "value")
	span.Log("info", "message")
	span.LogInfo("message")
	span.LogError("message")
	span.SetError()
}

func TestTracer_InjectExtractMessage(t *testing.T) {
	tracer := NewTracer(DefaultTracerConfig())

	span := tracer.StartSpan("test")
	payload := []byte("hello world")

	msg := tracer.InjectTraceToMessage(span, payload)

	if msg.TraceID != span.TraceID {
		t.Error("TraceID not injected")
	}

	if msg.SpanID != span.SpanID {
		t.Error("SpanID not injected")
	}

	// Extract
	traceID, spanID := tracer.ExtractTraceFromMessage(msg)
	if traceID != span.TraceID || spanID != span.SpanID {
		t.Error("Failed to extract trace info")
	}

	tracer.EndSpan(span)
}

func TestTracer_InjectNilSpan(t *testing.T) {
	tracer := NewTracer(DefaultTracerConfig())

	msg := tracer.InjectTraceToMessage(nil, []byte("data"))

	if msg.TraceID != "" {
		t.Error("TraceID should be empty for nil span")
	}
}

func TestTracer_ExtractNilMessage(t *testing.T) {
	tracer := NewTracer(DefaultTracerConfig())

	traceID, spanID := tracer.ExtractTraceFromMessage(nil)
	if traceID != "" || spanID != "" {
		t.Error("Should return empty for nil message")
	}
}

func TestTraceContext_ToHeader(t *testing.T) {
	tc := &TraceContext{
		TraceID: "0123456789abcdef0123456789abcdef",
		SpanID:  "0123456789abcdef",
	}

	header := tc.ToHeader()
	expected := "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01"

	if header != expected {
		t.Errorf("Header = %s, want %s", header, expected)
	}
}

func TestTraceContext_ToHeader_Empty(t *testing.T) {
	tc := &TraceContext{}
	if tc.ToHeader() != "" {
		t.Error("Empty context should return empty header")
	}
}

func TestParseTraceHeader(t *testing.T) {
	header := "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01"

	tc := ParseTraceHeader(header)
	if tc == nil {
		t.Fatal("Should parse valid header")
	}

	if tc.TraceID != "0123456789abcdef0123456789abcdef" {
		t.Errorf("TraceID = %s", tc.TraceID)
	}

	if tc.SpanID != "0123456789abcdef" {
		t.Errorf("SpanID = %s", tc.SpanID)
	}
}

func TestParseTraceHeader_Invalid(t *testing.T) {
	tests := []string{
		"",
		"invalid",
		"01-invalid-version",
		"00-short",
	}

	for _, header := range tests {
		tc := ParseTraceHeader(header)
		if tc != nil {
			t.Errorf("Should return nil for invalid header: %s", header)
		}
	}
}

func TestGenerateIDs(t *testing.T) {
	// TraceID should be 32 hex chars
	traceID := generateTraceID()
	if len(traceID) != 32 {
		t.Errorf("TraceID length = %d, want 32", len(traceID))
	}

	// SpanID should be 16 hex chars
	spanID := generateSpanID()
	if len(spanID) != 16 {
		t.Errorf("SpanID length = %d, want 16", len(spanID))
	}

	// Should be unique
	traceID2 := generateTraceID()
	if traceID == traceID2 {
		t.Error("Generated IDs should be unique")
	}
}

// Mock exporter for testing
type mockExporter struct {
	spans []*Span
	err   error
}

func (m *mockExporter) Export(ctx context.Context, spans []*Span) error {
	if m.err != nil {
		return m.err
	}
	m.spans = append(m.spans, spans...)
	return nil
}

func TestTracer_Flush(t *testing.T) {
	exporter := &mockExporter{}

	config := DefaultTracerConfig()
	config.Exporter = exporter
	tracer := NewTracer(config)

	// Create and complete some spans
	for i := 0; i < 5; i++ {
		span := tracer.StartSpan("op")
		tracer.EndSpan(span)
	}

	// Flush
	err := tracer.Flush(context.Background())
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}

	if len(exporter.spans) != 5 {
		t.Errorf("Expected 5 exported spans, got %d", len(exporter.spans))
	}

	// Completed spans should be cleared
	if len(tracer.GetCompletedSpans()) != 0 {
		t.Error("Completed spans should be cleared after flush")
	}
}

func TestTracer_FlushNoExporter(t *testing.T) {
	tracer := NewTracer(DefaultTracerConfig())

	span := tracer.StartSpan("op")
	tracer.EndSpan(span)

	// Should not error without exporter
	err := tracer.Flush(context.Background())
	if err != nil {
		t.Errorf("Flush without exporter should not error: %v", err)
	}
}
