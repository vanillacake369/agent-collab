package libp2p

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// TraceID is a unique identifier for a distributed trace
type TraceID string

// SpanID is a unique identifier for a span within a trace
type SpanID string

// Span represents a unit of work in a distributed trace
type Span struct {
	TraceID   TraceID           `json:"trace_id"`
	SpanID    SpanID            `json:"span_id"`
	ParentID  SpanID            `json:"parent_id,omitempty"` // Empty for root span
	Name      string            `json:"name"`
	Service   string            `json:"service"`
	StartTime time.Time         `json:"start_time"`
	EndTime   time.Time         `json:"end_time,omitempty"`
	Duration  time.Duration     `json:"duration,omitempty"`
	Status    SpanStatus        `json:"status"`
	Tags      map[string]string `json:"tags,omitempty"`
	Logs      []SpanLog         `json:"logs,omitempty"`
}

// SpanStatus represents the status of a span
type SpanStatus int

const (
	StatusUnset SpanStatus = iota
	StatusOK
	StatusError
)

func (s SpanStatus) String() string {
	switch s {
	case StatusOK:
		return "ok"
	case StatusError:
		return "error"
	default:
		return "unset"
	}
}

// SpanLog represents a log entry within a span
type SpanLog struct {
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
	Level     string    `json:"level,omitempty"` // "info", "warn", "error"
}

// TracedMessage wraps a message with tracing information
type TracedMessage struct {
	TraceID   TraceID `json:"trace_id"`
	SpanID    SpanID  `json:"span_id"`
	Timestamp int64   `json:"timestamp"`
	Payload   []byte  `json:"payload"`
}

// Tracer provides distributed tracing functionality
type Tracer struct {
	mu sync.RWMutex

	serviceName string
	enabled     bool

	// Active spans
	spans map[SpanID]*Span

	// Completed spans (limited buffer for export)
	completedSpans []*Span
	maxCompleted   int

	// Callbacks
	onSpanComplete func(*Span)
	exporter       SpanExporter
}

// SpanExporter exports completed spans
type SpanExporter interface {
	Export(ctx context.Context, spans []*Span) error
}

// TracerConfig configures the tracer
type TracerConfig struct {
	ServiceName  string
	Enabled      bool
	MaxCompleted int // Max completed spans to buffer
	Exporter     SpanExporter
}

// DefaultTracerConfig returns sensible defaults
func DefaultTracerConfig() TracerConfig {
	return TracerConfig{
		ServiceName:  "agent-collab",
		Enabled:      true,
		MaxCompleted: 1000,
	}
}

// NewTracer creates a new tracer
func NewTracer(config TracerConfig) *Tracer {
	if config.MaxCompleted == 0 {
		config.MaxCompleted = 1000
	}

	return &Tracer{
		serviceName:    config.ServiceName,
		enabled:        config.Enabled,
		spans:          make(map[SpanID]*Span),
		completedSpans: make([]*Span, 0, config.MaxCompleted),
		maxCompleted:   config.MaxCompleted,
		exporter:       config.Exporter,
	}
}

// SetEnabled enables or disables tracing
func (t *Tracer) SetEnabled(enabled bool) {
	t.mu.Lock()
	t.enabled = enabled
	t.mu.Unlock()
}

// IsEnabled returns whether tracing is enabled
func (t *Tracer) IsEnabled() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.enabled
}

// OnSpanComplete registers a callback for when spans complete
func (t *Tracer) OnSpanComplete(fn func(*Span)) {
	t.mu.Lock()
	t.onSpanComplete = fn
	t.mu.Unlock()
}

// SetExporter sets the span exporter
func (t *Tracer) SetExporter(exporter SpanExporter) {
	t.mu.Lock()
	t.exporter = exporter
	t.mu.Unlock()
}

// StartSpan starts a new root span
func (t *Tracer) StartSpan(name string) *Span {
	return t.StartSpanWithParent(name, "", "")
}

// StartSpanWithParent starts a new span with a parent
func (t *Tracer) StartSpanWithParent(name string, traceID TraceID, parentID SpanID) *Span {
	if !t.IsEnabled() {
		return nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Generate IDs
	if traceID == "" {
		traceID = generateTraceID()
	}
	spanID := generateSpanID()

	span := &Span{
		TraceID:   traceID,
		SpanID:    spanID,
		ParentID:  parentID,
		Name:      name,
		Service:   t.serviceName,
		StartTime: time.Now(),
		Status:    StatusUnset,
		Tags:      make(map[string]string),
	}

	t.spans[spanID] = span
	return span
}

// StartSpanFromContext starts a span from context trace info
func (t *Tracer) StartSpanFromContext(ctx context.Context, name string) (*Span, context.Context) {
	if !t.IsEnabled() {
		return nil, ctx
	}

	// Extract trace info from context
	traceID, parentID := extractTraceFromContext(ctx)

	span := t.StartSpanWithParent(name, traceID, parentID)
	if span == nil {
		return nil, ctx
	}

	// Inject trace info into context
	ctx = injectTraceToContext(ctx, span.TraceID, span.SpanID)
	return span, ctx
}

// EndSpan ends a span
func (t *Tracer) EndSpan(span *Span) {
	if span == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	span.EndTime = time.Now()
	span.Duration = span.EndTime.Sub(span.StartTime)

	if span.Status == StatusUnset {
		span.Status = StatusOK
	}

	// Remove from active spans
	delete(t.spans, span.SpanID)

	// Add to completed spans
	t.completedSpans = append(t.completedSpans, span)

	// Trim if over limit
	if len(t.completedSpans) > t.maxCompleted {
		// Remove oldest 10%
		removeCount := t.maxCompleted / 10
		t.completedSpans = t.completedSpans[removeCount:]
	}

	// Notify callback
	if t.onSpanComplete != nil {
		go t.onSpanComplete(span)
	}
}

// EndSpanWithError ends a span with an error
func (t *Tracer) EndSpanWithError(span *Span, err error) {
	if span == nil {
		return
	}

	span.Status = StatusError
	if err != nil {
		span.SetTag("error.message", err.Error())
		span.Log("error", err.Error())
	}

	t.EndSpan(span)
}

// GetSpan returns an active span by ID
func (t *Tracer) GetSpan(id SpanID) *Span {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.spans[id]
}

// GetActiveSpans returns all active spans
func (t *Tracer) GetActiveSpans() []*Span {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]*Span, 0, len(t.spans))
	for _, span := range t.spans {
		result = append(result, span)
	}
	return result
}

// GetCompletedSpans returns completed spans
func (t *Tracer) GetCompletedSpans() []*Span {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]*Span, len(t.completedSpans))
	copy(result, t.completedSpans)
	return result
}

// GetCompletedSpansSince returns completed spans since a timestamp
func (t *Tracer) GetCompletedSpansSince(since time.Time) []*Span {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []*Span
	for _, span := range t.completedSpans {
		if span.EndTime.After(since) {
			result = append(result, span)
		}
	}
	return result
}

// GetTraceSpans returns all spans for a trace ID
func (t *Tracer) GetTraceSpans(traceID TraceID) []*Span {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []*Span

	// Check active spans
	for _, span := range t.spans {
		if span.TraceID == traceID {
			result = append(result, span)
		}
	}

	// Check completed spans
	for _, span := range t.completedSpans {
		if span.TraceID == traceID {
			result = append(result, span)
		}
	}

	return result
}

// Flush exports all completed spans
func (t *Tracer) Flush(ctx context.Context) error {
	t.mu.Lock()
	exporter := t.exporter
	spans := t.completedSpans
	t.completedSpans = make([]*Span, 0, t.maxCompleted)
	t.mu.Unlock()

	if exporter == nil || len(spans) == 0 {
		return nil
	}

	return exporter.Export(ctx, spans)
}

// Stats returns tracing statistics
func (t *Tracer) Stats() TracerStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return TracerStats{
		Enabled:        t.enabled,
		ActiveSpans:    len(t.spans),
		CompletedSpans: len(t.completedSpans),
		ServiceName:    t.serviceName,
	}
}

// TracerStats holds tracer statistics
type TracerStats struct {
	Enabled        bool   `json:"enabled"`
	ActiveSpans    int    `json:"active_spans"`
	CompletedSpans int    `json:"completed_spans"`
	ServiceName    string `json:"service_name"`
}

// Span methods

// SetTag sets a tag on the span
func (s *Span) SetTag(key, value string) *Span {
	if s == nil {
		return s
	}
	s.Tags[key] = value
	return s
}

// Log adds a log entry to the span
func (s *Span) Log(level, message string) *Span {
	if s == nil {
		return s
	}
	s.Logs = append(s.Logs, SpanLog{
		Timestamp: time.Now(),
		Message:   message,
		Level:     level,
	})
	return s
}

// LogInfo adds an info log entry
func (s *Span) LogInfo(message string) *Span {
	return s.Log("info", message)
}

// LogError adds an error log entry
func (s *Span) LogError(message string) *Span {
	return s.Log("error", message)
}

// SetError marks the span as error
func (s *Span) SetError() *Span {
	if s == nil {
		return s
	}
	s.Status = StatusError
	return s
}

// Context keys for trace propagation
type traceContextKey struct{}
type spanContextKey struct{}

// extractTraceFromContext extracts trace info from context
func extractTraceFromContext(ctx context.Context) (TraceID, SpanID) {
	traceID, _ := ctx.Value(traceContextKey{}).(TraceID)
	spanID, _ := ctx.Value(spanContextKey{}).(SpanID)
	return traceID, spanID
}

// injectTraceToContext injects trace info into context
func injectTraceToContext(ctx context.Context, traceID TraceID, spanID SpanID) context.Context {
	ctx = context.WithValue(ctx, traceContextKey{}, traceID)
	ctx = context.WithValue(ctx, spanContextKey{}, spanID)
	return ctx
}

// InjectTraceToMessage wraps a message with trace info
func (t *Tracer) InjectTraceToMessage(span *Span, payload []byte) *TracedMessage {
	if span == nil {
		return &TracedMessage{
			Timestamp: time.Now().UnixNano(),
			Payload:   payload,
		}
	}

	return &TracedMessage{
		TraceID:   span.TraceID,
		SpanID:    span.SpanID,
		Timestamp: time.Now().UnixNano(),
		Payload:   payload,
	}
}

// ExtractTraceFromMessage extracts trace info from a message
func (t *Tracer) ExtractTraceFromMessage(msg *TracedMessage) (TraceID, SpanID) {
	if msg == nil {
		return "", ""
	}
	return msg.TraceID, msg.SpanID
}

// ID generation helpers

func generateTraceID() TraceID {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return TraceID(hex.EncodeToString(b))
}

func generateSpanID() SpanID {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return SpanID(hex.EncodeToString(b))
}

// PropagationFormat defines how trace context is propagated
type PropagationFormat int

const (
	// PropagationJSON uses JSON encoding
	PropagationJSON PropagationFormat = iota
	// PropagationW3C uses W3C Trace Context format
	PropagationW3C
)

// TraceContext holds trace context for propagation
type TraceContext struct {
	TraceID TraceID `json:"trace_id"`
	SpanID  SpanID  `json:"span_id"`
}

// ToHeader converts trace context to a header string (W3C format)
func (tc *TraceContext) ToHeader() string {
	if tc.TraceID == "" || tc.SpanID == "" {
		return ""
	}
	// W3C Trace Context: version-trace_id-span_id-flags
	return "00-" + string(tc.TraceID) + "-" + string(tc.SpanID) + "-01"
}

// ParseTraceHeader parses a W3C Trace Context header
func ParseTraceHeader(header string) *TraceContext {
	if header == "" || len(header) < 55 {
		return nil
	}

	// Expected format: 00-trace_id(32)-span_id(16)-flags(2)
	if header[0:3] != "00-" {
		return nil
	}

	parts := header[3:] // Remove "00-"
	if len(parts) < 52 {
		return nil
	}

	traceID := TraceID(parts[0:32])
	spanID := SpanID(parts[33:49])

	return &TraceContext{
		TraceID: traceID,
		SpanID:  spanID,
	}
}
