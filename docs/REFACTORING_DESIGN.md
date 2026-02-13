# Refactoring Design: Code Quality & TODO Implementation

## Executive Summary

This document provides a comprehensive architectural design for merging two refactoring initiatives:
1. **Code Smell Fixes**: Replace panic(), implement structured logging, extract constants, fix error handling
2. **TODO Implementations**: TUI integrations, new API endpoints, metrics tracking

The design follows a phased approach with clear dependencies, shared infrastructure, and risk mitigation strategies.

---

## 1. Shared Infrastructure Design

### 1.1 Structured Logging System

**Location**: `src/infrastructure/logging/`

**Design**:
```go
package logging

import (
    "context"
    "log/slog"
    "os"
)

// Logger is the application-wide structured logger
type Logger struct {
    *slog.Logger
}

// LogLevel represents log severity
type LogLevel string

const (
    LevelDebug LogLevel = "DEBUG"
    LevelInfo  LogLevel = "INFO"
    LevelWarn  LogLevel = "WARN"
    LevelError LogLevel = "ERROR"
)

// New creates a new structured logger
func New(level LogLevel) *Logger {
    var slogLevel slog.Level
    switch level {
    case LevelDebug:
        slogLevel = slog.LevelDebug
    case LevelInfo:
        slogLevel = slog.LevelInfo
    case LevelWarn:
        slogLevel = slog.LevelWarn
    case LevelError:
        slogLevel = slog.LevelError
    default:
        slogLevel = slog.LevelInfo
    }

    handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slogLevel,
    })

    return &Logger{slog.New(handler)}
}

// WithContext adds context fields to logger
func (l *Logger) WithContext(ctx context.Context) *Logger {
    // Extract common context values (nodeID, agentID, etc.)
    return l
}

// Component creates a logger for a specific component
func (l *Logger) Component(name string) *Logger {
    return &Logger{l.Logger.With("component", name)}
}
```

**Usage Pattern**:
```go
// Initialize in App
app.logger = logging.New(logging.LevelInfo).Component("app")

// Use throughout codebase
app.logger.Info("cluster initialized",
    "project", projectName,
    "nodeID", nodeID,
    "peers", peerCount)

app.logger.Error("failed to acquire lock",
    "error", err,
    "filePath", filePath,
    "intention", intention)
```

### 1.2 Application-Wide Constants

**Location**: `src/domain/constants/`

**Design**:
```go
package constants

import "time"

// Lock system constants
const (
    DefaultLockTTL        = 30 * time.Second
    MaxLockTTL            = 5 * time.Minute
    LockHeartbeatInterval = 10 * time.Second
    MaxLockRenewals       = 100
    LockIDPrefix          = "lock-"
)

// Network timeouts
const (
    DefaultNetworkTimeout  = 30 * time.Second
    BootstrapTimeout      = 60 * time.Second
    DHTPutTimeout         = 30 * time.Second
    DHTGetTimeout         = 30 * time.Second
    PeerDiscoveryInterval = 15 * time.Second
)

// HTTP/Daemon constants
const (
    DaemonSocketName     = "daemon.sock"
    DaemonPIDName        = "daemon.pid"
    HTTPReadTimeout      = 10 * time.Second
    HTTPWriteTimeout     = 10 * time.Second
    HTTPIdleTimeout      = 60 * time.Second
    HTTPShutdownGrace    = 100 * time.Millisecond
)

// Hook execution constants
const (
    HookTimeout           = 3 * time.Second
    HookMaxRetries        = 3
    HookLockStateFile     = "hook-locks.json"
)

// Retry patterns
const (
    DefaultRetryDelay     = 1 * time.Second
    MaxRetryDelay         = 30 * time.Second
    RetryBackoffFactor    = 2.0
)

// TUI refresh intervals
const (
    TUITickInterval       = 1 * time.Second
    TUIMetricsRefresh     = 5 * time.Second
    TUIResultDisplayTime  = 5 * time.Second
)

// Metrics collection
const (
    MetricsBufferSize     = 1000
    MetricsFlushInterval  = 30 * time.Second
    MetricsRetention      = 24 * time.Hour
)

// ID prefixes for type safety
const (
    PrefixLock      = "lock-"
    PrefixAgent     = "agent-"
    PrefixContext   = "ctx-"
    PrefixToken     = "tok-"
    PrefixPeer      = "peer-"
)
```

### 1.3 Error Types & Handling

**Location**: `src/domain/errors/`

**Design**:
```go
package errors

import (
    "errors"
    "fmt"
)

// Domain errors - exported for type checking
var (
    // Lock errors
    ErrLockConflict        = errors.New("lock conflict: resource already locked")
    ErrLockNotFound        = errors.New("lock not found")
    ErrLockExpired         = errors.New("lock expired")
    ErrMaxRenewalsExceeded = errors.New("max lock renewals exceeded")
    ErrInvalidLockTarget   = errors.New("invalid lock target")

    // Network errors
    ErrNodeNotInitialized  = errors.New("node not initialized")
    ErrPeerNotFound        = errors.New("peer not found")
    ErrBootstrapFailed     = errors.New("bootstrap failed")
    ErrConnectionFailed    = errors.New("connection failed")

    // Context sync errors
    ErrContextNotFound     = errors.New("context not found")
    ErrSyncFailed          = errors.New("synchronization failed")
    ErrEmbeddingFailed     = errors.New("embedding generation failed")

    // Daemon errors
    ErrDaemonNotRunning    = errors.New("daemon not running")
    ErrDaemonAlreadyRunning = errors.New("daemon already running")
    ErrInvalidConfig       = errors.New("invalid configuration")

    // Validation errors
    ErrInvalidInput        = errors.New("invalid input")
    ErrMissingParameter    = errors.New("missing required parameter")
)

// ValidationError wraps validation failures
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed for %s: %s", e.Field, e.Message)
}

// NewValidationError creates a validation error
func NewValidationError(field, message string) *ValidationError {
    return &ValidationError{Field: field, Message: message}
}

// WrapError adds context to an error
func WrapError(err error, context string) error {
    if err == nil {
        return nil
    }
    return fmt.Errorf("%s: %w", context, err)
}

// IsRetryable checks if an error should trigger a retry
func IsRetryable(err error) bool {
    return errors.Is(err, ErrConnectionFailed) ||
           errors.Is(err, ErrBootstrapFailed) ||
           errors.Is(err, ErrSyncFailed)
}
```

**Panic Replacement Pattern**:
```go
// BEFORE (from lock.go:36-40)
if target == nil {
    panic("target cannot be nil")
}
if holderID == "" {
    panic("holderID cannot be empty")
}

// AFTER
func NewSemanticLock(target *SemanticTarget, holderID, holderName, intention string) (*SemanticLock, error) {
    if target == nil {
        return nil, errors.NewValidationError("target", "cannot be nil")
    }
    if holderID == "" {
        return nil, errors.NewValidationError("holderID", "cannot be empty")
    }
    // ... rest of implementation
    return lock, nil
}
```

---

## 2. API Design for New Endpoints

### 2.1 Leave Cluster Endpoint

**Location**: `src/interface/daemon/server.go`

**Implementation**:
```go
// Add to registerRoutes():
mux.HandleFunc("/leave", s.handleLeave)

// Handler implementation:
func (s *Server) handleLeave(w http.ResponseWriter, r *http.Request) {
    logger := s.logger.Component("api.leave")

    // Graceful shutdown sequence
    logger.Info("initiating cluster leave")

    // 1. Release all locks held by this node
    if s.app.LockService() != nil {
        locks := s.app.LockService().ListLocks()
        for _, lock := range locks {
            if lock.HolderID == s.app.NodeID() {
                logger.Info("releasing lock before leave", "lockID", lock.ID)
                if err := s.app.LockService().ReleaseLock(s.ctx, lock.ID); err != nil {
                    logger.Warn("failed to release lock", "lockID", lock.ID, "error", err)
                }
            }
        }
    }

    // 2. Broadcast leave message to peers
    if s.app.Node() != nil {
        logger.Info("broadcasting leave message to peers")
        s.app.BroadcastLeave()
    }

    // 3. Flush any pending context shares
    if s.app.VectorStore() != nil {
        logger.Info("flushing vector store")
        s.app.VectorStore().Flush()
    }

    // 4. Publish leave event
    s.PublishEvent(NewEvent(EventClusterLeave, map[string]any{
        "nodeID": s.app.NodeID(),
        "timestamp": time.Now(),
    }))

    // 5. Stop node gracefully
    if err := s.app.LeaveCluster(s.ctx); err != nil {
        logger.Error("failed to leave cluster", "error", err)
        json.NewEncoder(w).Encode(GenericResponse{
            Success: false,
            Error:   err.Error(),
        })
        return
    }

    logger.Info("successfully left cluster")
    json.NewEncoder(w).Encode(GenericResponse{
        Success: true,
        Message: "Left cluster successfully",
    })
}
```

**New App Method**:
```go
// src/application/app.go
func (a *App) LeaveCluster(ctx context.Context) error {
    a.mu.Lock()
    defer a.mu.Unlock()

    logger := a.logger.Component("leaveCluster")

    // Stop accepting new work
    a.running = false

    // Close libp2p node
    if a.node != nil {
        logger.Info("closing libp2p node")
        if err := a.node.Close(); err != nil {
            logger.Warn("error closing node", "error", err)
        }
        a.node = nil
    }

    // Stop domain services
    if a.lockService != nil {
        a.lockService.Shutdown()
    }
    if a.syncManager != nil {
        a.syncManager.Stop()
    }

    // Clear cluster config but preserve local data
    a.config.ProjectName = ""
    a.config.Bootstrap = nil

    if err := a.SaveConfig(); err != nil {
        logger.Warn("failed to save config", "error", err)
    }

    return nil
}

func (a *App) BroadcastLeave() {
    if a.node == nil {
        return
    }

    msg := map[string]any{
        "type": "leave",
        "nodeID": a.node.ID().String(),
        "timestamp": time.Now().Unix(),
    }

    data, _ := json.Marshal(msg)
    a.node.Publish(a.ctx, "cluster.control", data)
}
```

### 2.2 Metrics Endpoint

**Location**: `src/interface/daemon/server.go` (already exists at line 605)

**Enhancement Required**:
```go
// Current implementation returns node metrics only
// Need to add comprehensive system metrics

type MetricsResponse struct {
    // Node metrics (existing)
    Network NetworkMetrics `json:"network"`

    // System metrics (new)
    System SystemMetrics `json:"system"`

    // Application metrics (new)
    Application AppMetrics `json:"application"`

    // Timestamp
    Timestamp time.Time `json:"timestamp"`
}

type SystemMetrics struct {
    CPUUsage    float64 `json:"cpu_usage_percent"`
    MemoryUsage int64   `json:"memory_bytes"`
    GoroutineCount int  `json:"goroutine_count"`
    Uptime      int64   `json:"uptime_seconds"`
}

type AppMetrics struct {
    LocksActive      int     `json:"locks_active"`
    LocksTotal       int64   `json:"locks_total"`
    ContextsShared   int64   `json:"contexts_shared"`
    EventsProcessed  int64   `json:"events_processed"`
    ErrorsTotal      int64   `json:"errors_total"`
    RequestsPerSec   float64 `json:"requests_per_second"`
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
    node := s.app.Node()
    if node == nil {
        json.NewEncoder(w).Encode(map[string]any{
            "error": "node not initialized",
        })
        return
    }

    // Network metrics from existing implementation
    networkMetrics := node.GetMetricsSnapshot()

    // System metrics
    systemMetrics := s.collectSystemMetrics()

    // Application metrics
    appMetrics := s.collectAppMetrics()

    response := MetricsResponse{
        Network:     networkMetrics,
        System:      systemMetrics,
        Application: appMetrics,
        Timestamp:   time.Now(),
    }

    json.NewEncoder(w).Encode(response)
}

func (s *Server) collectSystemMetrics() SystemMetrics {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)

    return SystemMetrics{
        CPUUsage:       s.metrics.GetCPUUsage(), // Implement CPU tracking
        MemoryUsage:    int64(m.Alloc),
        GoroutineCount: runtime.NumGoroutine(),
        Uptime:         int64(time.Since(s.startedAt).Seconds()),
    }
}

func (s *Server) collectAppMetrics() AppMetrics {
    lockCount := 0
    if s.app.LockService() != nil {
        lockCount = len(s.app.LockService().ListLocks())
    }

    return AppMetrics{
        LocksActive:     lockCount,
        LocksTotal:      s.metrics.GetTotalLocks(),
        ContextsShared:  s.metrics.GetTotalContexts(),
        EventsProcessed: s.metrics.GetTotalEvents(),
        ErrorsTotal:     s.metrics.GetTotalErrors(),
        RequestsPerSec:  s.metrics.GetRequestRate(),
    }
}
```

### 2.3 Context Stats Endpoint

**Location**: `src/interface/daemon/server.go`

```go
// Add to registerRoutes():
mux.HandleFunc("/context/stats", s.handleContextStats)

type ContextStatsResponse struct {
    TotalDocuments   int64              `json:"total_documents"`
    TotalSize        int64              `json:"total_size_bytes"`
    Collections      []CollectionStats  `json:"collections"`
    RecentActivity   []ContextActivity  `json:"recent_activity"`
    SyncStatus       SyncStatusInfo     `json:"sync_status"`
}

type CollectionStats struct {
    Name      string  `json:"name"`
    Documents int64   `json:"documents"`
    Size      int64   `json:"size_bytes"`
    AvgScore  float32 `json:"avg_similarity_score"`
}

type ContextActivity struct {
    FilePath  string    `json:"file_path"`
    Agent     string    `json:"agent"`
    Timestamp time.Time `json:"timestamp"`
    Action    string    `json:"action"` // "shared", "updated", "synced"
}

type SyncStatusInfo struct {
    Healthy       bool              `json:"healthy"`
    LastSyncAt    time.Time         `json:"last_sync_at"`
    PendingShares int               `json:"pending_shares"`
    Progress      map[string]float64 `json:"progress"` // peer -> percentage
}

func (s *Server) handleContextStats(w http.ResponseWriter, r *http.Request) {
    vectorStore := s.app.VectorStore()
    syncManager := s.app.SyncManager()

    if vectorStore == nil {
        json.NewEncoder(w).Encode(ContextStatsResponse{})
        return
    }

    // Get storage stats
    stats := vectorStore.Stats()

    // Get recent activity from event bus
    recentActivity := s.getRecentContextActivity(20)

    // Get sync status
    syncStatus := SyncStatusInfo{
        Healthy: true,
        LastSyncAt: time.Now(), // Get from sync manager
        PendingShares: 0,
        Progress: make(map[string]float64),
    }

    if syncManager != nil {
        syncStatus = syncManager.GetStatus()
    }

    response := ContextStatsResponse{
        TotalDocuments: stats.TotalDocuments,
        TotalSize:      stats.TotalSize,
        Collections:    stats.Collections,
        RecentActivity: recentActivity,
        SyncStatus:     syncStatus,
    }

    json.NewEncoder(w).Encode(response)
}

func (s *Server) getRecentContextActivity(limit int) []ContextActivity {
    events := s.eventBus.GetEventsByType(EventContextUpdated, limit)

    activity := make([]ContextActivity, 0, len(events))
    for _, event := range events {
        if data, ok := event.Data.(ContextEventData); ok {
            activity = append(activity, ContextActivity{
                FilePath:  data.FilePath,
                Agent:     data.Agent,
                Timestamp: event.Timestamp,
                Action:    "shared",
            })
        }
    }

    return activity
}
```

### 2.4 Token Usage Endpoint

**Location**: `src/interface/daemon/server.go`

```go
// Add to registerRoutes():
mux.HandleFunc("/tokens/usage", s.handleTokenUsage)

type TokenUsageResponse struct {
    Today   TokenUsageStats `json:"today"`
    Week    TokenUsageStats `json:"week"`
    Month   TokenUsageStats `json:"month"`

    Breakdown []TokenBreakdown `json:"breakdown"`
    Hourly    []HourlyUsage    `json:"hourly"`

    Limits    TokenLimits      `json:"limits"`
    Cost      CostInfo         `json:"cost"`
}

type TokenUsageStats struct {
    Total       int64   `json:"total"`
    Input       int64   `json:"input"`
    Output      int64   `json:"output"`
    Embedding   int64   `json:"embedding"`
    CostUSD     float64 `json:"cost_usd"`
}

type TokenBreakdown struct {
    Operation string `json:"operation"` // "embed", "search", "lock", etc.
    Count     int64  `json:"count"`
    Tokens    int64  `json:"tokens"`
}

type HourlyUsage struct {
    Hour   time.Time `json:"hour"`
    Tokens int64     `json:"tokens"`
}

type TokenLimits struct {
    Daily   int64 `json:"daily"`
    Weekly  int64 `json:"weekly"`
    Monthly int64 `json:"monthly"`
}

type CostInfo struct {
    Today  float64 `json:"today"`
    Week   float64 `json:"week"`
    Month  float64 `json:"month"`
    Model  string  `json:"model"`
}

func (s *Server) handleTokenUsage(w http.ResponseWriter, r *http.Request) {
    tracker := s.app.TokenTracker()
    if tracker == nil {
        json.NewEncoder(w).Encode(TokenUsageResponse{
            Limits: TokenLimits{
                Daily:   200000, // Default
                Weekly:  1000000,
                Monthly: 4000000,
            },
        })
        return
    }

    now := time.Now()

    response := TokenUsageResponse{
        Today:     tracker.GetUsage(now, now.AddDate(0, 0, -1)),
        Week:      tracker.GetUsage(now, now.AddDate(0, 0, -7)),
        Month:     tracker.GetUsage(now, now.AddDate(0, -1, 0)),
        Breakdown: tracker.GetBreakdown(now.AddDate(0, 0, -1), now),
        Hourly:    tracker.GetHourlyUsage(now.AddDate(0, 0, -1), now),
        Limits:    tracker.GetLimits(),
        Cost:      tracker.GetCost(),
    }

    json.NewEncoder(w).Encode(response)
}
```

---

## 3. TUI Integration Design

### 3.1 Action Functions Implementation

**Location**: `src/interface/tui/update.go`

**executeInit** (line 418-423):
```go
func (m *Model) executeInit(projectName string) error {
    client := daemon.NewClient()

    resp, err := client.Init(projectName)
    if err != nil {
        return errors.WrapError(err, "failed to initialize cluster")
    }

    // Update model state
    m.projectName = resp.ProjectName
    m.nodeID = resp.NodeID

    m.SetResult(fmt.Sprintf("Cluster initialized: %s\nNode ID: %s\nInvite Token: %s",
        resp.ProjectName, resp.NodeID, resp.InviteToken), nil)

    return nil
}
```

**executeJoin** (line 425-429):
```go
func (m *Model) executeJoin(token string) error {
    client := daemon.NewClient()

    resp, err := client.Join(token)
    if err != nil {
        return errors.WrapError(err, "failed to join cluster")
    }

    // Update model state
    m.projectName = resp.ProjectName
    m.nodeID = resp.NodeID
    m.peerCount = resp.ConnectedPeers

    m.SetResult(fmt.Sprintf("Joined cluster: %s\nConnected to %d peer(s)",
        resp.ProjectName, resp.ConnectedPeers), nil)

    return nil
}
```

**executeLeave** (line 431-435):
```go
func (m *Model) executeLeave() error {
    client := daemon.NewClient()

    // Call new /leave endpoint
    resp, err := client.Post("/leave", nil)
    if err != nil {
        return errors.WrapError(err, "failed to leave cluster")
    }
    defer resp.Body.Close()

    var result daemon.GenericResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return err
    }

    if result.Error != "" {
        return fmt.Errorf(result.Error)
    }

    // Reset model state
    m.projectName = ""
    m.nodeID = ""
    m.peerCount = 0

    m.SetResult("Successfully left cluster", nil)

    return nil
}
```

**executeReleaseLock** (line 437-441):
```go
func (m *Model) executeReleaseLock(lockID string) error {
    client := daemon.NewClient()

    if err := client.ReleaseLock(lockID); err != nil {
        return errors.WrapError(err, "failed to release lock")
    }

    // Refresh locks view
    cmd := m.fetchLocks()
    if cmd != nil {
        go func() {
            msg := cmd()
            // Send update to TUI
        }()
    }

    m.SetResult(fmt.Sprintf("Released lock: %s", lockID), nil)

    return nil
}
```

### 3.2 Fetch Functions Implementation

**fetchMetrics** (line 474-486):
```go
func (m Model) fetchMetrics() tea.Cmd {
    return func() tea.Msg {
        client := daemon.NewClient()
        if !client.IsRunning() {
            return MetricsMsg{}
        }

        resp, err := client.Get("/metrics")
        if err != nil {
            return MetricsMsg{}
        }
        defer resp.Body.Close()

        var metrics MetricsResponse
        if err := json.NewDecoder(resp.Body).Decode(&metrics); err != nil {
            return MetricsMsg{}
        }

        return MetricsMsg{
            CPUUsage:    metrics.System.CPUUsage,
            MemUsage:    metrics.System.MemoryUsage,
            NetUpload:   metrics.Network.TotalBytesSent,
            NetDownload: metrics.Network.TotalBytesReceived,
            TokensRate:  metrics.Application.RequestsPerSec,
        }
    }
}
```

**fetchContext** (line 584-594):
```go
func (m Model) fetchContext() tea.Cmd {
    return func() tea.Msg {
        client := daemon.NewClient()
        if !client.IsRunning() {
            return ContextMsg{}
        }

        resp, err := client.Get("/context/stats")
        if err != nil {
            return ContextMsg{}
        }
        defer resp.Body.Close()

        var stats ContextStatsResponse
        if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
            return ContextMsg{}
        }

        return ContextMsg{
            TotalEmbeddings: int(stats.TotalDocuments),
            DatabaseSize:    stats.TotalSize,
            SyncProgress:    stats.SyncStatus.Progress,
        }
    }
}
```

**fetchTokens** (line 596-612):
```go
func (m Model) fetchTokens() tea.Cmd {
    return func() tea.Msg {
        client := daemon.NewClient()
        if !client.IsRunning() {
            return TokensMsg{
                DailyLimit: 200000,
            }
        }

        resp, err := client.Get("/tokens/usage")
        if err != nil {
            return TokensMsg{DailyLimit: 200000}
        }
        defer resp.Body.Close()

        var usage TokenUsageResponse
        if err := json.NewDecoder(resp.Body).Decode(&usage); err != nil {
            return TokensMsg{DailyLimit: 200000}
        }

        // Convert hourly data
        hourlyData := make([]float64, len(usage.Hourly))
        for i, h := range usage.Hourly {
            hourlyData[i] = float64(h.Tokens)
        }

        return TokensMsg{
            TodayUsed:   usage.Today.Total,
            DailyLimit:  usage.Limits.Daily,
            Breakdown:   convertBreakdown(usage.Breakdown),
            HourlyData:  hourlyData,
            CostToday:   usage.Cost.Today,
            CostWeek:    usage.Cost.Week,
            CostMonth:   usage.Cost.Month,
            TokensWeek:  usage.Week.Total,
            TokensMonth: usage.Month.Total,
        }
    }
}
```

### 3.3 View Type Assertions

**Location**: `src/interface/tui/views/*.go`

Pattern for all view files:
```go
// BEFORE (views/context.go:49)
// TODO: 타입 어설션

// AFTER
func (v *ContextView) Update(data interface{}) error {
    contextData, ok := data.(ContextData)
    if !ok {
        return errors.NewValidationError("data",
            fmt.Sprintf("expected ContextData, got %T", data))
    }

    v.data = contextData
    return nil
}
```

Apply to:
- `views/context.go:49` - ContextData
- `views/locks.go:36` - LocksData
- `views/peers.go:41` - PeersData
- `views/tokens.go:43` - TokensData

### 3.4 Sync Health Calculation

**Location**: `src/interface/tui/app.go:205`

```go
// BEFORE
SyncHealth: 100, // TODO: 실제 sync health 계산

// AFTER
func calculateSyncHealth(status *daemon.StatusResponse, syncStatus SyncStatusInfo) float64 {
    if !syncStatus.Healthy {
        return 0
    }

    // Base health on peer connectivity
    baseHealth := 70.0
    if status.PeerCount > 0 {
        baseHealth = 100.0
    }

    // Reduce based on pending work
    if syncStatus.PendingShares > 0 {
        penalty := float64(syncStatus.PendingShares) * 2.0
        baseHealth -= penalty
    }

    // Factor in sync progress across peers
    if len(syncStatus.Progress) > 0 {
        avgProgress := 0.0
        for _, p := range syncStatus.Progress {
            avgProgress += p
        }
        avgProgress /= float64(len(syncStatus.Progress))
        baseHealth = (baseHealth + avgProgress) / 2.0
    }

    if baseHealth < 0 {
        baseHealth = 0
    }
    if baseHealth > 100 {
        baseHealth = 100
    }

    return baseHealth
}

// Usage
SyncHealth: calculateSyncHealth(status, syncStatus),
```

---

## 4. Hook Refactoring Design

### 4.1 Extract Magic Numbers

**Location**: `plugin/hooks/pre-tool-enforcer.mjs`

```javascript
// BEFORE (scattered throughout)
timeout: 3000
timeout: 5000

// AFTER - Add constants at top of file
const CONSTANTS = {
    TIMEOUTS: {
        CLUSTER_STATUS: 3000,  // 3 seconds
        LOCK_ACQUIRE: 5000,    // 5 seconds
        DEFAULT: 3000
    },
    PATHS: {
        LOCK_STATE_DIR: process.env.AGENT_COLLAB_DATA_DIR ||
                       join(process.env.HOME || '/tmp', '.agent-collab'),
        LOCK_STATE_FILE: 'hook-locks.json'
    },
    LOCK: {
        START_LINE_DEFAULT: 1,
        END_LINE_ENTIRE_FILE: -1
    }
};

// Usage
const result = execSync('agent-collab mcp call cluster_status \'{}\'', {
    encoding: 'utf-8',
    timeout: CONSTANTS.TIMEOUTS.CLUSTER_STATUS,
    stdio: ['pipe', 'pipe', 'pipe']
});
```

**Location**: `plugin/hooks/collab-detector.mjs`

```javascript
const CONSTANTS = {
    TIMEOUTS: {
        MCP_CALL: 3000  // 3 seconds
    },
    DETECTION: {
        MIN_PROMPT_LENGTH: 5,
        MAX_SANITIZED_LENGTH: 10000
    },
    CONTEXT: {
        MAX_RECENT_ACTIVITY: 20,
        MAX_SIMILAR_CONTEXTS: 10
    }
};
```

### 4.2 Error Handling Improvements

**Pattern for all hooks**:

```javascript
// BEFORE
try {
    const result = execSync(...);
    return { active: false };
} catch {
    return { active: false };
}

// AFTER
const logger = createLogger('hook:pre-tool-enforcer');

try {
    const result = execSync(...);
    return { active: false };
} catch (err) {
    logger.warn('failed to check cluster status', {
        error: err.message,
        code: err.code
    });
    return { active: false };
}

// Logger implementation
function createLogger(component) {
    return {
        info: (msg, meta) => console.error(JSON.stringify({
            level: 'INFO',
            component,
            message: msg,
            ...meta,
            timestamp: new Date().toISOString()
        })),
        warn: (msg, meta) => console.error(JSON.stringify({
            level: 'WARN',
            component,
            message: msg,
            ...meta,
            timestamp: new Date().toISOString()
        })),
        error: (msg, meta) => console.error(JSON.stringify({
            level: 'ERROR',
            component,
            message: msg,
            ...meta,
            timestamp: new Date().toISOString()
        }))
    };
}
```

### 4.3 Mixed Language Messages

**Strategy**: Create message catalogs

```javascript
// plugin/hooks/lib/messages.mjs
export const MESSAGES = {
    en: {
        LOCK_ACQUIRED: (filePath, lockId) =>
            `[AUTO-LOCK] ${filePath} acquired (${lockId})`,
        LOCK_CONFLICT: (filePath, error) =>
            `[LOCK CONFLICT] ${filePath}: ${error || 'another agent may be working on this file'}`,
        LOCK_REUSED: (filePath, lockId) =>
            `[LOCK REUSED] ${filePath} (${lockId})`,
        CLUSTER_ACTIVE: () =>
            '[AGENT-COLLAB CLUSTER ACTIVE]',
        COLLABORATION_PROTOCOL: () =>
            'Collaboration Protocol for File Modifications'
    },
    ko: {
        LOCK_ACQUIRED: (filePath, lockId) =>
            `[자동 잠금] ${filePath} 획득 (${lockId})`,
        LOCK_CONFLICT: (filePath, error) =>
            `[잠금 충돌] ${filePath}: ${error || '다른 에이전트가 작업 중일 수 있습니다'}`,
        // ... Korean translations
    }
};

// Usage
const locale = process.env.AGENT_COLLAB_LOCALE || 'en';
const msg = MESSAGES[locale];

message = msg.LOCK_ACQUIRED(filePath, lockResult.lockId);
```

---

## 5. Implementation Order & Dependencies

### Phase 1: Foundation (Week 1)
**Goal**: Establish shared infrastructure

1. **Create logging system** (`src/infrastructure/logging/`)
   - Implement Logger with slog
   - Add component loggers
   - Dependencies: None
   - Risk: Low

2. **Create constants package** (`src/domain/constants/`)
   - Extract all magic numbers
   - Define ID prefixes
   - Dependencies: None
   - Risk: Low

3. **Create error types** (`src/domain/errors/`)
   - Define domain errors
   - Create validation errors
   - Dependencies: None
   - Risk: Low

4. **Replace panic() calls** (4 files)
   - `src/domain/lock/lock.go`
   - `src/infrastructure/network/libp2p/compression.go`
   - `src/controller/manager.go`
   - `src/interface/notification/manager.go`
   - Dependencies: Error types
   - Risk: Medium (breaking changes to APIs)

**Validation**: All tests pass, no panics remain

### Phase 2: Logging Migration (Week 1-2)
**Goal**: Replace fmt.Printf with structured logging

5. **Integrate logger into App**
   - Add logger field to App struct
   - Initialize in NewApp
   - Dependencies: Logging system
   - Risk: Low

6. **Replace fmt.Printf in high-traffic paths** (29 files)
   - Prioritize: daemon, CLI, controllers
   - Use logger.Info/Warn/Error
   - Dependencies: Logger in App
   - Risk: Medium (potential log spam)

**Validation**: Logs are structured JSON, searchable

### Phase 3: New API Endpoints (Week 2)
**Goal**: Implement missing endpoints

7. **Implement /leave endpoint**
   - Add handleLeave to server
   - Implement App.LeaveCluster
   - Dependencies: Logger, errors
   - Risk: Medium (cleanup logic)

8. **Enhance /metrics endpoint**
   - Add system metrics collection
   - Add application metrics
   - Dependencies: Logger
   - Risk: Low

9. **Implement /context/stats endpoint**
   - Add handleContextStats
   - Integrate with VectorStore
   - Dependencies: Logger, errors
   - Risk: Low

10. **Implement /tokens/usage endpoint**
    - Add handleTokenUsage
    - Integrate with TokenTracker
    - Dependencies: Logger, TokenTracker
    - Risk: Low (may not exist yet)

**Validation**: All endpoints return valid JSON, handle errors

### Phase 4: TUI Integration (Week 2-3)
**Goal**: Connect TUI to new APIs

11. **Implement action functions**
    - executeInit (already partial)
    - executeJoin (already partial)
    - executeLeave (new)
    - executeReleaseLock (new)
    - Dependencies: New endpoints
    - Risk: Low

12. **Implement fetch functions**
    - fetchMetrics (complete)
    - fetchContext (complete)
    - fetchTokens (complete)
    - Dependencies: New endpoints
    - Risk: Low

13. **Fix view type assertions**
    - Add type checking in all view Update methods
    - Return validation errors
    - Dependencies: Error types
    - Risk: Low

14. **Implement sync health calculation**
    - Create calculateSyncHealth function
    - Integrate into fetchStatus
    - Dependencies: SyncManager
    - Risk: Low

**Validation**: TUI displays live data, no crashes on invalid data

### Phase 5: Hook Refactoring (Week 3)
**Goal**: Improve hook code quality

15. **Extract constants in hooks**
    - Create CONSTANTS objects
    - Replace all magic numbers
    - Dependencies: None
    - Risk: Low

16. **Improve error handling in hooks**
    - Add structured logging
    - Log specific error codes
    - Dependencies: None
    - Risk: Low

17. **Internationalize messages**
    - Create message catalogs
    - Add locale selection
    - Dependencies: None
    - Risk: Low

**Validation**: Hooks run without errors, logs are clear

### Phase 6: Code Duplication & Polish (Week 3-4)
**Goal**: Reduce duplication, improve maintainability

18. **Extract common HTTP patterns** (daemon client)
    - Create handleAPICall helper
    - Reduce duplicate error handling
    - Dependencies: Error types
    - Risk: Low

19. **Standardize ID generation**
    - Use constants for prefixes
    - Create ID generation utilities
    - Dependencies: Constants
    - Risk: Low

20. **Track TODOs with GitHub issues**
    - Create issues for each TODO
    - Link to code locations
    - Dependencies: None
    - Risk: None

**Validation**: Code coverage maintained, no new TODOs added

---

## 6. Risk Assessment & Mitigation

### High-Risk Changes

#### 1. Panic Removal (Breaking Changes)
**Risk**: Functions that previously panicked now return errors
**Impact**: Callers must handle errors
**Mitigation**:
- Create wrapper functions for backward compatibility
- Use static analysis to find all call sites
- Add comprehensive tests
- Phase rollout: deprecate old functions first

**Example**:
```go
// Old (deprecated)
func NewSemanticLock(target, holderID, ...) *SemanticLock {
    lock, err := NewSemanticLockSafe(target, holderID, ...)
    if err != nil {
        panic(err) // Maintain old behavior temporarily
    }
    return lock
}

// New (safe)
func NewSemanticLockSafe(target, holderID, ...) (*SemanticLock, error) {
    // Validation with error returns
}
```

#### 2. Logging Migration Volume
**Risk**: Too much logging in hot paths causes performance degradation
**Impact**: High CPU usage, log storage costs
**Mitigation**:
- Start with Info level, measure performance
- Add sampling for high-frequency logs
- Use conditional logging in hot paths
- Monitor log volume metrics

**Example**:
```go
// Hot path: only log errors and sample 1% of info
if err != nil {
    logger.Error("operation failed", "error", err)
} else if rand.Float64() < 0.01 {
    logger.Debug("operation completed", "duration", elapsed)
}
```

#### 3. /leave Endpoint Cleanup
**Risk**: Incomplete cleanup leaves cluster in inconsistent state
**Impact**: Orphaned locks, zombie nodes, resource leaks
**Mitigation**:
- Implement idempotent cleanup
- Add timeout for each cleanup step
- Log all cleanup failures
- Add /leave/force endpoint for emergency

**Example**:
```go
func (a *App) LeaveCluster(ctx context.Context) error {
    // Use timeout for entire operation
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    // Each step has its own timeout
    if err := a.cleanupLocks(ctx); err != nil {
        logger.Warn("lock cleanup failed", "error", err)
        // Continue anyway
    }

    // Best-effort cleanup
    a.cleanupPeers(ctx)
    a.cleanupStorage(ctx)

    return nil
}
```

### Medium-Risk Changes

#### 4. TUI Data Fetching Race Conditions
**Risk**: Concurrent fetches update model inconsistently
**Impact**: UI shows stale/mixed data
**Mitigation**:
- Use mutex for model updates
- Implement fetch debouncing
- Add data versioning

#### 5. Metrics Collection Overhead
**Risk**: Collecting too many metrics impacts performance
**Impact**: Increased CPU/memory usage
**Mitigation**:
- Use sampling for high-frequency metrics
- Implement metric aggregation
- Add metrics toggle

### Low-Risk Changes

#### 6. Constants Extraction
**Risk**: Minimal - purely organizational
**Mitigation**: Code review, grep for hardcoded values

#### 7. Hook Refactoring
**Risk**: Low - hooks are isolated
**Mitigation**: Test with actual Claude workflows

---

## 7. Testing Strategy

### Unit Tests

**New Components**:
```go
// src/infrastructure/logging/logger_test.go
func TestLogger_Component(t *testing.T) {
    logger := New(LevelInfo)
    component := logger.Component("test")
    // Verify component field is added
}

// src/domain/errors/errors_test.go
func TestValidationError(t *testing.T) {
    err := NewValidationError("field", "message")
    assert.Contains(t, err.Error(), "field")
}
```

**Modified Components**:
```go
// src/domain/lock/lock_test.go
func TestNewSemanticLockSafe_Validation(t *testing.T) {
    _, err := NewSemanticLockSafe(nil, "holder", "", "")
    assert.Error(t, err)
    assert.IsType(t, &errors.ValidationError{}, err)
}
```

### Integration Tests

**API Endpoints**:
```go
// src/interface/daemon/server_test.go
func TestHandleLeave(t *testing.T) {
    server, app := setupTestServer(t)

    // Initialize cluster first
    initCluster(t, app)

    // Test leave
    resp := httptest.NewRecorder()
    req := httptest.NewRequest("POST", "/leave", nil)

    server.handleLeave(resp, req)

    assert.Equal(t, http.StatusOK, resp.Code)
    assert.False(t, app.running)
}
```

**TUI Functions**:
```go
// src/interface/tui/update_test.go
func TestExecuteInit(t *testing.T) {
    model := NewTestModel()

    err := model.executeInit("test-project")

    assert.NoError(t, err)
    assert.Equal(t, "test-project", model.projectName)
}
```

### End-to-End Tests

**Workflow Tests**:
```bash
#!/bin/bash
# tests/e2e/cluster_lifecycle.sh

# Start daemon
agent-collab daemon start

# Initialize cluster
agent-collab init -p test-cluster

# Verify TUI shows cluster
agent-collab tui --test-mode &
TUI_PID=$!

sleep 2

# Check TUI connected
ps -p $TUI_PID || exit 1

# Leave cluster
agent-collab leave

# Verify cleanup
! pgrep -f "agent-collab daemon" || exit 1

echo "✓ Cluster lifecycle test passed"
```

---

## 8. Success Criteria

### Code Quality Metrics

- [ ] **Zero panic() calls** in production code
- [ ] **100% structured logging** (no fmt.Printf in daemon/core)
- [ ] **Zero magic numbers** in code (all in constants)
- [ ] **90%+ test coverage** for new code
- [ ] **All TODOs tracked** in GitHub issues

### Functional Metrics

- [ ] **All TUI tabs functional** with live data
- [ ] **All API endpoints respond** with valid JSON
- [ ] **Leave cluster cleanup** completes in <5 seconds
- [ ] **Metrics endpoint** responds in <100ms
- [ ] **Hook execution** completes in <3 seconds

### Performance Metrics

- [ ] **Logging overhead <5%** CPU impact
- [ ] **Metrics collection <2%** memory increase
- [ ] **No regressions** in lock acquisition time
- [ ] **TUI refresh rate** maintains 1 second

### User Experience Metrics

- [ ] **Error messages** are actionable and clear
- [ ] **Logs** are searchable and contextual
- [ ] **TUI** displays sync health accurately
- [ ] **Hooks** provide helpful collaboration hints

---

## 9. Rollback Plan

### If Critical Issues Arise

1. **Immediate**: Revert merged PR
2. **Within 1 hour**: Identify root cause
3. **Within 4 hours**: Fix forward or rollback
4. **Within 24 hours**: Post-mortem

### Feature Flags

For gradual rollout:
```go
// src/infrastructure/config/flags.go
type FeatureFlags struct {
    UseStructuredLogging bool `json:"use_structured_logging"`
    EnableMetricsV2      bool `json:"enable_metrics_v2"`
    EnableLeaveEndpoint  bool `json:"enable_leave_endpoint"`
}

func (a *App) IsFeatureEnabled(flag string) bool {
    return a.config.Features[flag]
}
```

### Backward Compatibility

Maintain deprecated functions for 2 releases:
```go
// Deprecated: Use NewSemanticLockSafe instead
func NewSemanticLock(...) *SemanticLock {
    lock, err := NewSemanticLockSafe(...)
    if err != nil {
        panic(err) // Maintain old behavior
    }
    return lock
}
```

---

## 10. Documentation Updates

### Developer Documentation

- [ ] Update `CONTRIBUTING.md` with logging guidelines
- [ ] Add "Error Handling" section to architecture docs
- [ ] Document new API endpoints in OpenAPI spec
- [ ] Add TUI development guide

### User Documentation

- [ ] Update CLI reference with `/leave` command
- [ ] Add metrics interpretation guide
- [ ] Document token usage tracking
- [ ] Add troubleshooting for common errors

### Code Documentation

- [ ] Add package-level docs for `logging`, `errors`, `constants`
- [ ] Document panic-to-error migration in CHANGELOG
- [ ] Add inline examples for new error types
- [ ] Generate godoc for new packages

---

## Appendix A: File Modification Checklist

### Go Files (42 total)

**Critical Path** (Replace panic, add logging):
- [ ] `src/domain/lock/lock.go`
- [ ] `src/infrastructure/network/libp2p/compression.go`
- [ ] `src/controller/manager.go`
- [ ] `src/interface/notification/manager.go`

**Daemon/API** (New endpoints):
- [ ] `src/interface/daemon/server.go`
- [ ] `src/interface/daemon/types.go`
- [ ] `src/interface/daemon/client.go`

**Application** (Core logic):
- [ ] `src/application/app.go`

**TUI** (Integration):
- [ ] `src/interface/tui/update.go`
- [ ] `src/interface/tui/model.go`
- [ ] `src/interface/tui/app.go`
- [ ] `src/interface/tui/views/context.go`
- [ ] `src/interface/tui/views/locks.go`
- [ ] `src/interface/tui/views/peers.go`
- [ ] `src/interface/tui/views/tokens.go`

**Logging Migration** (29 files with fmt.Printf):
- [ ] All CLI commands
- [ ] All controllers
- [ ] All infrastructure

### JavaScript Files (6 hooks)

- [ ] `plugin/hooks/pre-tool-enforcer.mjs`
- [ ] `plugin/hooks/collab-detector.mjs`
- [ ] `plugin/hooks/session-start.mjs`
- [ ] `plugin/hooks/session-end.mjs`
- [ ] `plugin/hooks/post-tool-reminder.mjs`
- [ ] `plugin/hooks/subagent-tracker.mjs`

### New Files to Create

- [ ] `src/infrastructure/logging/logger.go`
- [ ] `src/infrastructure/logging/logger_test.go`
- [ ] `src/domain/constants/constants.go`
- [ ] `src/domain/errors/errors.go`
- [ ] `src/domain/errors/errors_test.go`
- [ ] `plugin/hooks/lib/messages.mjs`
- [ ] `plugin/hooks/lib/logger.mjs`

---

## Appendix B: Migration Examples

### Before/After: Panic to Error

```go
// BEFORE: src/domain/lock/lock.go:34-40
func NewSemanticLock(target *SemanticTarget, holderID, holderName, intention string) *SemanticLock {
    if target == nil {
        panic("target cannot be nil")
    }
    if holderID == "" {
        panic("holderID cannot be empty")
    }
    // ...
}

// AFTER
func NewSemanticLock(target *SemanticTarget, holderID, holderName, intention string) (*SemanticLock, error) {
    if target == nil {
        return nil, errors.NewValidationError("target", "cannot be nil")
    }
    if holderID == "" {
        return nil, errors.NewValidationError("holderID", "cannot be empty")
    }
    // ...
    return lock, nil
}

// Caller updates
// BEFORE
lock := lock.NewSemanticLock(target, holderID, holderName, intention)

// AFTER
lock, err := lock.NewSemanticLock(target, holderID, holderName, intention)
if err != nil {
    return fmt.Errorf("failed to create lock: %w", err)
}
```

### Before/After: fmt.Printf to Logger

```go
// BEFORE: src/interface/daemon/server.go:560
fmt.Printf("Warning: failed to broadcast context: %v\n", err)

// AFTER
s.logger.Warn("failed to broadcast context",
    "error", err,
    "filePath", req.FilePath)
```

### Before/After: Magic Number to Constant

```go
// BEFORE: plugin/hooks/pre-tool-enforcer.mjs:30
timeout: 3000,

// AFTER
timeout: CONSTANTS.TIMEOUTS.CLUSTER_STATUS,
```

---

## Conclusion

This refactoring design provides a comprehensive, phased approach to improving code quality while implementing missing functionality. By establishing shared infrastructure first (logging, errors, constants), we create a solid foundation for both bug fixes and new features.

The design prioritizes:
1. **Safety**: Error returns instead of panics
2. **Observability**: Structured logging throughout
3. **Maintainability**: Constants and clear error types
4. **Functionality**: Complete TUI integration and API coverage
5. **User Experience**: Better error messages and i18n support

Implementation should proceed in the order specified, with thorough testing at each phase. The entire refactoring is estimated at 3-4 weeks for a single developer, or 2 weeks with pair programming.

**Next Steps**: Review this design with the team, adjust priorities if needed, and begin Phase 1 implementation.
