package event

import (
	"sync"
	"time"
)

// EventLogConfig holds configuration for EventLog.
type EventLogConfig struct {
	MaxSize          int
	EventTTL         time.Duration
	CompactInterval  time.Duration
	MaxEventsPerFile int // 0 = unlimited
}

// DefaultEventLogConfig returns default configuration.
func DefaultEventLogConfig() *EventLogConfig {
	return &EventLogConfig{
		MaxSize:          10000,
		EventTTL:         DefaultEventTTL,
		CompactInterval:  5 * time.Minute,
		MaxEventsPerFile: 10,
	}
}

// EventLog stores events with indexing and lifecycle management.
type EventLog struct {
	mu       sync.RWMutex
	config   *EventLogConfig
	events   []*Event
	byID     map[string]*Event
	byType   map[EventType][]*Event
	bySource map[string][]*Event
	byFile   map[string][]*Event // Index by file path for compaction

	stopCh chan struct{}
}

// NewEventLog creates a new event log.
func NewEventLog(cfg *EventLogConfig) *EventLog {
	if cfg == nil {
		cfg = DefaultEventLogConfig()
	}

	if cfg.MaxSize <= 0 {
		cfg.MaxSize = 10000
	}

	el := &EventLog{
		config:   cfg,
		events:   make([]*Event, 0),
		byID:     make(map[string]*Event),
		byType:   make(map[EventType][]*Event),
		bySource: make(map[string][]*Event),
		byFile:   make(map[string][]*Event),
		stopCh:   make(chan struct{}),
	}

	if cfg.CompactInterval > 0 {
		go el.compactionLoop()
	}

	return el
}

// compactionLoop runs periodic compaction.
func (el *EventLog) compactionLoop() {
	ticker := time.NewTicker(el.config.CompactInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			el.Compact()
		case <-el.stopCh:
			return
		}
	}
}

// Stop stops the compaction loop.
func (el *EventLog) Stop() {
	close(el.stopCh)
}

// Append adds an event to the log.
func (el *EventLog) Append(event *Event) {
	el.mu.Lock()
	defer el.mu.Unlock()

	if _, exists := el.byID[event.ID]; exists {
		return
	}

	el.events = append(el.events, event)
	el.byID[event.ID] = event
	el.byType[event.Type] = append(el.byType[event.Type], event)
	el.bySource[event.SourceID] = append(el.bySource[event.SourceID], event)

	if event.FilePath != "" {
		el.byFile[event.FilePath] = append(el.byFile[event.FilePath], event)
		el.compactFileEventsLocked(event.FilePath)
	}

	el.enforceSizeLimitLocked()
}

// compactFileEventsLocked compacts events for a specific file.
// Must be called with lock held.
func (el *EventLog) compactFileEventsLocked(filePath string) {
	if el.config.MaxEventsPerFile <= 0 {
		return
	}

	fileEvents := el.byFile[filePath]
	if len(fileEvents) <= el.config.MaxEventsPerFile {
		return
	}

	// Keep only the most recent events
	toRemove := len(fileEvents) - el.config.MaxEventsPerFile
	for i := 0; i < toRemove; i++ {
		oldEvent := fileEvents[i]
		oldEvent.Status = EventStatusArchived
		oldEvent.SupersededBy = fileEvents[len(fileEvents)-1].ID
	}

	// Update byFile index to only reference active events
	el.byFile[filePath] = fileEvents[toRemove:]
}

// enforceSizeLimitLocked removes oldest events when size limit exceeded.
// Must be called with lock held.
func (el *EventLog) enforceSizeLimitLocked() {
	for len(el.events) > el.config.MaxSize {
		el.removeOldestLocked()
	}
}

// removeOldestLocked removes the oldest event.
// Must be called with lock held.
func (el *EventLog) removeOldestLocked() {
	if len(el.events) == 0 {
		return
	}

	oldest := el.events[0]
	el.events = el.events[1:]
	delete(el.byID, oldest.ID)

	el.removeFromTypeIndex(oldest)
	el.removeFromSourceIndex(oldest)
	el.removeFromFileIndex(oldest)
}

func (el *EventLog) removeFromTypeIndex(event *Event) {
	events := el.byType[event.Type]
	for i, e := range events {
		if e.ID == event.ID {
			el.byType[event.Type] = append(events[:i], events[i+1:]...)
			break
		}
	}
}

func (el *EventLog) removeFromSourceIndex(event *Event) {
	events := el.bySource[event.SourceID]
	for i, e := range events {
		if e.ID == event.ID {
			el.bySource[event.SourceID] = append(events[:i], events[i+1:]...)
			if len(el.bySource[event.SourceID]) == 0 {
				delete(el.bySource, event.SourceID)
			}
			break
		}
	}
}

func (el *EventLog) removeFromFileIndex(event *Event) {
	if event.FilePath == "" {
		return
	}

	events := el.byFile[event.FilePath]
	for i, e := range events {
		if e.ID == event.ID {
			el.byFile[event.FilePath] = append(events[:i], events[i+1:]...)
			if len(el.byFile[event.FilePath]) == 0 {
				delete(el.byFile, event.FilePath)
			}
			break
		}
	}
}

// Compact removes expired and archived events.
func (el *EventLog) Compact() {
	el.mu.Lock()
	defer el.mu.Unlock()

	var active []*Event
	for _, event := range el.events {
		if el.shouldKeep(event) {
			active = append(active, event)
		} else {
			el.removeFromIndexesLocked(event)
		}
	}

	el.events = active
}

// shouldKeep determines if an event should be kept.
func (el *EventLog) shouldKeep(event *Event) bool {
	if event.IsExpired() {
		return false
	}
	if event.Status == EventStatusArchived {
		return false
	}
	return true
}

// removeFromIndexesLocked removes event from all indexes.
// Must be called with lock held.
func (el *EventLog) removeFromIndexesLocked(event *Event) {
	delete(el.byID, event.ID)
	el.removeFromTypeIndex(event)
	el.removeFromSourceIndex(event)
	el.removeFromFileIndex(event)
}

// Get retrieves an event by ID.
func (el *EventLog) Get(eventID string) (*Event, bool) {
	el.mu.RLock()
	defer el.mu.RUnlock()
	event, ok := el.byID[eventID]
	return event, ok
}

// GetByType retrieves events by type.
func (el *EventLog) GetByType(eventType EventType) []*Event {
	el.mu.RLock()
	defer el.mu.RUnlock()
	return el.filterActive(el.byType[eventType])
}

// GetBySource retrieves events by source.
func (el *EventLog) GetBySource(sourceID string) []*Event {
	el.mu.RLock()
	defer el.mu.RUnlock()
	return el.filterActive(el.bySource[sourceID])
}

// GetByFile retrieves events by file path.
func (el *EventLog) GetByFile(filePath string) []*Event {
	el.mu.RLock()
	defer el.mu.RUnlock()
	return el.filterActive(el.byFile[filePath])
}

// GetLatestByFile retrieves the most recent event for a file.
func (el *EventLog) GetLatestByFile(filePath string) *Event {
	el.mu.RLock()
	defer el.mu.RUnlock()

	events := el.byFile[filePath]
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Status != EventStatusArchived {
			return events[i]
		}
	}
	return nil
}

// GetRecent retrieves the most recent active events.
func (el *EventLog) GetRecent(count int) []*Event {
	el.mu.RLock()
	defer el.mu.RUnlock()

	active := el.filterActive(el.events)
	if count <= 0 || count > len(active) {
		count = len(active)
	}

	result := make([]*Event, count)
	copy(result, active[len(active)-count:])
	return result
}

// GetSince retrieves events since a timestamp.
func (el *EventLog) GetSince(since time.Time) []*Event {
	el.mu.RLock()
	defer el.mu.RUnlock()

	var result []*Event
	for _, event := range el.events {
		if event.Status == EventStatusArchived {
			continue
		}
		if event.Timestamp.After(since) || event.Timestamp.Equal(since) {
			result = append(result, event)
		}
	}
	return result
}

// filterActive returns only non-archived events.
func (el *EventLog) filterActive(events []*Event) []*Event {
	var active []*Event
	for _, event := range events {
		if event.Status != EventStatusArchived && !event.IsExpired() {
			active = append(active, event)
		}
	}
	return active
}

// Size returns the number of active events.
func (el *EventLog) Size() int {
	el.mu.RLock()
	defer el.mu.RUnlock()
	return len(el.filterActive(el.events))
}

// TotalSize returns the total number of events including archived.
func (el *EventLog) TotalSize() int {
	el.mu.RLock()
	defer el.mu.RUnlock()
	return len(el.events)
}

// Clear clears the log.
func (el *EventLog) Clear() {
	el.mu.Lock()
	defer el.mu.Unlock()

	el.events = make([]*Event, 0)
	el.byID = make(map[string]*Event)
	el.byType = make(map[EventType][]*Event)
	el.bySource = make(map[string][]*Event)
	el.byFile = make(map[string][]*Event)
}

// MarkFileEventsCompleted marks all events for a file as completed.
func (el *EventLog) MarkFileEventsCompleted(filePath string) {
	el.mu.Lock()
	defer el.mu.Unlock()

	for _, event := range el.byFile[filePath] {
		event.MarkCompleted()
	}
}

// GetActiveFiles returns list of files with active events.
func (el *EventLog) GetActiveFiles() []string {
	el.mu.RLock()
	defer el.mu.RUnlock()

	var files []string
	for filePath, events := range el.byFile {
		for _, event := range events {
			if event.Status == EventStatusActive {
				files = append(files, filePath)
				break
			}
		}
	}
	return files
}

// GetSummary returns a summary of the event log state.
func (el *EventLog) GetSummary() *EventLogSummary {
	el.mu.RLock()
	defer el.mu.RUnlock()

	summary := &EventLogSummary{
		TotalEvents:    len(el.events),
		ActiveEvents:   0,
		ArchivedEvents: 0,
		UniqueFiles:    len(el.byFile),
		UniqueSources:  len(el.bySource),
		EventsByType:   make(map[EventType]int),
	}

	for _, event := range el.events {
		if event.Status == EventStatusArchived {
			summary.ArchivedEvents++
		} else {
			summary.ActiveEvents++
		}
		summary.EventsByType[event.Type]++
	}

	return summary
}

// EventLogSummary contains summary statistics about the event log.
type EventLogSummary struct {
	TotalEvents    int
	ActiveEvents   int
	ArchivedEvents int
	UniqueFiles    int
	UniqueSources  int
	EventsByType   map[EventType]int
}
