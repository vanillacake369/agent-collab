# Refactoring Design V2: 수정된 설계

## 비평 반영 요약

### Critical Issues 수정
1. **순환 의존성 해결** - 패키지 구조 재설계
2. **slog → zerolog** - 성능 최적화 (zap은 libp2p와 이미 의존성 있음)
3. **타임라인 현실화** - 3-4주 → 12-14주
4. **점진적 마이그레이션** - Big Bang 대신 단계적 접근

### 수정된 패키지 구조

```
src/
├── pkg/                          # 공유 유틸리티 (의존성 없음)
│   ├── errors/                   # 에러 인터페이스만 (도메인 에러 없음)
│   │   └── errors.go
│   └── logging/                  # zerolog 래퍼
│       └── logger.go
│
├── domain/
│   ├── lock/
│   │   ├── lock.go
│   │   ├── errors.go             # 락 관련 에러 (여기서 정의)
│   │   └── constants.go          # 락 관련 상수 (비공개)
│   ├── context/
│   │   ├── context.go
│   │   ├── errors.go
│   │   └── constants.go
│   └── token/                    # 새로 구현 필요
│       ├── tracker.go
│       ├── store.go
│       └── cost.go
│
├── infrastructure/
│   ├── http/
│   │   └── constants.go          # HTTP 관련 상수
│   └── metrics/
│       ├── collector.go
│       └── cache.go              # 메트릭 캐싱
│
└── interface/
    ├── daemon/
    │   ├── server.go
    │   ├── leave.go              # Leave 상태 머신
    │   └── ratelimit.go          # Rate limiting
    └── tui/
        └── ...
```

---

## Phase 1: Foundation (Week 1-2)

### 1.1 zerolog 기반 로깅 시스템

**파일**: `src/pkg/logging/logger.go`

```go
package logging

import (
    "io"
    "os"
    "time"

    "github.com/rs/zerolog"
)

// Logger wraps zerolog with application-specific methods
type Logger struct {
    zl zerolog.Logger
}

// New creates a new logger
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

// Component creates a sub-logger for a specific component
func (l *Logger) Component(name string) *Logger {
    return &Logger{
        zl: l.zl.With().Str("component", name).Logger(),
    }
}

// Info logs at info level
func (l *Logger) Info(msg string, fields ...interface{}) {
    event := l.zl.Info()
    l.addFields(event, fields...)
    event.Msg(msg)
}

// Warn logs at warn level
func (l *Logger) Warn(msg string, fields ...interface{}) {
    event := l.zl.Warn()
    l.addFields(event, fields...)
    event.Msg(msg)
}

// Error logs at error level
func (l *Logger) Error(msg string, fields ...interface{}) {
    event := l.zl.Error()
    l.addFields(event, fields...)
    event.Msg(msg)
}

// Debug logs at debug level
func (l *Logger) Debug(msg string, fields ...interface{}) {
    event := l.zl.Debug()
    l.addFields(event, fields...)
    event.Msg(msg)
}

func (l *Logger) addFields(event *zerolog.Event, fields ...interface{}) {
    for i := 0; i < len(fields)-1; i += 2 {
        key, ok := fields[i].(string)
        if !ok {
            continue
        }
        event.Interface(key, fields[i+1])
    }
}

// Sampling logger for hot paths
type SamplingLogger struct {
    *Logger
    rate      float64
    lastLog   time.Time
    interval  time.Duration
}

func (l *Logger) WithSampling(rate float64, interval time.Duration) *SamplingLogger {
    return &SamplingLogger{
        Logger:   l,
        rate:     rate,
        interval: interval,
    }
}

func (sl *SamplingLogger) ShouldLog() bool {
    if time.Since(sl.lastLog) < sl.interval {
        return false
    }
    sl.lastLog = time.Now()
    return true
}
```

### 1.2 에러 인터페이스 (도메인 에러 없음)

**파일**: `src/pkg/errors/errors.go`

```go
package errors

import (
    "errors"
    "fmt"
)

// Category represents error classification
type Category string

const (
    CategoryValidation Category = "validation"
    CategoryNetwork    Category = "network"
    CategoryRetryable  Category = "retryable"
    CategoryPermanent  Category = "permanent"
)

// Categorized is an error with category
type Categorized interface {
    error
    Category() Category
}

// IsRetryable checks if error should trigger retry
func IsRetryable(err error) bool {
    var cat Categorized
    if errors.As(err, &cat) {
        return cat.Category() == CategoryRetryable
    }
    return false
}

// Wrap adds context to an error
func Wrap(err error, msg string) error {
    if err == nil {
        return nil
    }
    return fmt.Errorf("%s: %w", msg, err)
}

// New creates a simple error
func New(msg string) error {
    return errors.New(msg)
}

// Is is errors.Is
func Is(err, target error) bool {
    return errors.Is(err, target)
}

// As is errors.As
func As(err error, target any) bool {
    return errors.As(err, target)
}
```

### 1.3 도메인별 에러 (lock 패키지 내부)

**파일**: `src/domain/lock/errors.go`

```go
package lock

import (
    "fmt"

    pkgerrors "agent-collab/src/pkg/errors"
)

// LockError represents a lock-related error
type LockError struct {
    Code     string
    Message  string
    category pkgerrors.Category
    LockID   string
    FilePath string
}

func (e *LockError) Error() string {
    if e.LockID != "" {
        return fmt.Sprintf("%s: %s (lock: %s)", e.Code, e.Message, e.LockID)
    }
    return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *LockError) Category() pkgerrors.Category {
    return e.category
}

// Predefined errors
var (
    ErrLockConflict = &LockError{
        Code:     "LOCK_CONFLICT",
        Message:  "resource already locked by another agent",
        category: pkgerrors.CategoryRetryable,
    }

    ErrLockNotFound = &LockError{
        Code:     "LOCK_NOT_FOUND",
        Message:  "lock does not exist",
        category: pkgerrors.CategoryPermanent,
    }

    ErrLockExpired = &LockError{
        Code:     "LOCK_EXPIRED",
        Message:  "lock has expired",
        category: pkgerrors.CategoryRetryable,
    }

    ErrInvalidTarget = &LockError{
        Code:     "INVALID_TARGET",
        Message:  "lock target is invalid",
        category: pkgerrors.CategoryValidation,
    }
)

// NewLockError creates a new lock error with context
func NewLockError(base *LockError, lockID, filePath string) *LockError {
    return &LockError{
        Code:     base.Code,
        Message:  base.Message,
        category: base.category,
        LockID:   lockID,
        FilePath: filePath,
    }
}
```

### 1.4 Panic → Error 변환 (점진적)

**파일**: `src/domain/lock/lock.go` 수정

```go
package lock

// NewSemanticLock creates a new lock (DEPRECATED: use NewSemanticLockSafe)
// Deprecated: This function panics on invalid input. Use NewSemanticLockSafe instead.
func NewSemanticLock(target *SemanticTarget, holderID, holderName, intention string) *SemanticLock {
    lock, err := NewSemanticLockSafe(target, holderID, holderName, intention)
    if err != nil {
        panic(err) // 기존 동작 유지 (일시적)
    }
    return lock
}

// NewSemanticLockSafe creates a new lock with error handling
func NewSemanticLockSafe(target *SemanticTarget, holderID, holderName, intention string) (*SemanticLock, error) {
    if target == nil {
        return nil, &ValidationError{
            Field:   "target",
            Message: "cannot be nil",
        }
    }
    if holderID == "" {
        return nil, &ValidationError{
            Field:   "holderID",
            Message: "cannot be empty",
        }
    }
    if holderName == "" {
        holderName = "unknown"
    }

    lock := &SemanticLock{
        ID:           generateLockID(),
        Target:       target,
        HolderID:     holderID,
        HolderName:   holderName,
        Intention:    intention,
        FencingToken: atomic.AddUint64(&fencingTokenCounter, 1),
        AcquiredAt:   time.Now(),
        ExpiresAt:    time.Now().Add(defaultLockTTL),
    }

    return lock, nil
}

// ValidationError for input validation failures
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed: %s %s", e.Field, e.Message)
}

// 상수는 패키지 내부에 (비공개)
const (
    defaultLockTTL = 30 * time.Second
    maxLockTTL     = 5 * time.Minute
    idPrefix       = "lock-"
)
```

---

## Phase 2: Structured Logging (Week 3-4)

### 2.1 App에 Logger 추가

**파일**: `src/application/app.go` 수정

```go
type App struct {
    // ... existing fields ...
    logger *logging.Logger
}

func New(cfg *Config) (*App, error) {
    logger := logging.New(os.Stdout, cfg.LogLevel).Component("app")

    app := &App{
        config: cfg,
        logger: logger,
    }

    return app, nil
}

// 기존 fmt.Printf는 유지하면서 새 로깅 추가 (점진적 마이그레이션)
func (a *App) processLockMessage(ctx context.Context, msg []byte) {
    // 새 코드: 구조화된 로깅
    a.logger.Debug("processing lock message", "size", len(msg))

    // 기존 코드: 일단 유지 (나중에 제거)
    // fmt.Printf("Processing lock message: %d bytes\n", len(msg))

    // ... 처리 로직 ...
}
```

### 2.2 로깅 마이그레이션 스크립트

**파일**: `scripts/migrate-logging.sh`

```bash
#!/bin/bash
# fmt.Printf → logger 마이그레이션 도우미

# 1. 모든 fmt.Printf 위치 찾기
echo "=== fmt.Printf 사용 위치 ==="
grep -rn "fmt.Printf" src/ --include="*.go" | grep -v "_test.go"

# 2. 마이그레이션 대상 파일 수
echo ""
echo "=== 마이그레이션 대상 파일 ==="
grep -rl "fmt.Printf" src/ --include="*.go" | grep -v "_test.go" | wc -l

# 3. 우선순위별 분류
echo ""
echo "=== 우선순위: Critical Path ==="
grep -rn "fmt.Printf" src/interface/daemon/ --include="*.go"

echo ""
echo "=== 우선순위: Application ==="
grep -rn "fmt.Printf" src/application/ --include="*.go"

echo ""
echo "=== 우선순위: Infrastructure ==="
grep -rn "fmt.Printf" src/infrastructure/ --include="*.go"
```

---

## Phase 3: New API Endpoints (Week 5-8)

### 3.1 Leave 상태 머신

**파일**: `src/interface/daemon/leave.go` (새 파일)

```go
package daemon

import (
    "context"
    "sync"
    "time"
)

// LeaveState represents the state of leave operation
type LeaveState int

const (
    LeaveStateIdle LeaveState = iota
    LeaveStateInProgress
    LeaveStateCompleted
    LeaveStateFailed
)

// LeaveStep represents a single cleanup step
type LeaveStep struct {
    Name     string
    Execute  func(context.Context) error
    Timeout  time.Duration
    Required bool
}

// LeaveManager manages cluster leave operations
type LeaveManager struct {
    mu     sync.Mutex
    state  LeaveState
    steps  []LeaveStep
    logger *logging.Logger
    app    *application.App
}

// NewLeaveManager creates a new leave manager
func NewLeaveManager(app *application.App, logger *logging.Logger) *LeaveManager {
    lm := &LeaveManager{
        state:  LeaveStateIdle,
        logger: logger.Component("leave"),
        app:    app,
    }

    // Define cleanup steps
    lm.steps = []LeaveStep{
        {
            Name:     "release-locks",
            Execute:  lm.releaseLocks,
            Timeout:  10 * time.Second,
            Required: false, // Best effort
        },
        {
            Name:     "broadcast-leave",
            Execute:  lm.broadcastLeave,
            Timeout:  5 * time.Second,
            Required: false, // Best effort
        },
        {
            Name:     "flush-context",
            Execute:  lm.flushContext,
            Timeout:  10 * time.Second,
            Required: false, // Best effort
        },
        {
            Name:     "close-node",
            Execute:  lm.closeNode,
            Timeout:  15 * time.Second,
            Required: true, // Must succeed
        },
    }

    return lm
}

// Leave executes the leave sequence
func (lm *LeaveManager) Leave(ctx context.Context) error {
    lm.mu.Lock()
    if lm.state == LeaveStateInProgress {
        lm.mu.Unlock()
        return ErrLeaveInProgress
    }
    lm.state = LeaveStateInProgress
    lm.mu.Unlock()

    // Overall timeout
    ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
    defer cancel()

    var lastErr error

    for _, step := range lm.steps {
        stepCtx, stepCancel := context.WithTimeout(ctx, step.Timeout)

        lm.logger.Info("executing leave step", "step", step.Name)
        err := step.Execute(stepCtx)
        stepCancel()

        if err != nil {
            lm.logger.Warn("leave step failed",
                "step", step.Name,
                "error", err,
                "required", step.Required)

            if step.Required {
                lm.mu.Lock()
                lm.state = LeaveStateFailed
                lm.mu.Unlock()
                return fmt.Errorf("required step %s failed: %w", step.Name, err)
            }
            lastErr = err
        }
    }

    lm.mu.Lock()
    lm.state = LeaveStateCompleted
    lm.mu.Unlock()

    if lastErr != nil {
        lm.logger.Warn("leave completed with non-critical errors", "lastError", lastErr)
    }

    return nil
}

func (lm *LeaveManager) releaseLocks(ctx context.Context) error {
    lockService := lm.app.LockService()
    if lockService == nil {
        return nil
    }

    locks := lockService.ListLocks()
    nodeID := lm.app.NodeID()

    for _, lock := range locks {
        if lock.HolderID == nodeID {
            if err := lockService.ReleaseLock(ctx, lock.ID); err != nil {
                lm.logger.Warn("failed to release lock", "lockID", lock.ID, "error", err)
            }
        }
    }
    return nil
}

func (lm *LeaveManager) broadcastLeave(ctx context.Context) error {
    node := lm.app.Node()
    if node == nil {
        return nil
    }
    return node.BroadcastLeave(ctx)
}

func (lm *LeaveManager) flushContext(ctx context.Context) error {
    store := lm.app.VectorStore()
    if store == nil {
        return nil
    }
    return store.Flush(ctx)
}

func (lm *LeaveManager) closeNode(ctx context.Context) error {
    node := lm.app.Node()
    if node == nil {
        return nil
    }
    return node.Close()
}

var ErrLeaveInProgress = errors.New("leave operation already in progress")
```

### 3.2 Metrics Caching & Rate Limiting

**파일**: `src/interface/daemon/ratelimit.go` (새 파일)

```go
package daemon

import (
    "net/http"
    "sync"
    "time"

    "golang.org/x/time/rate"
)

// RateLimiter provides per-client rate limiting
type RateLimiter struct {
    mu       sync.Mutex
    clients  map[string]*rate.Limiter
    limit    rate.Limit
    burst    int
    cleanup  time.Duration
    lastSeen map[string]time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(rps float64, burst int) *RateLimiter {
    rl := &RateLimiter{
        clients:  make(map[string]*rate.Limiter),
        limit:    rate.Limit(rps),
        burst:    burst,
        cleanup:  10 * time.Minute,
        lastSeen: make(map[string]time.Time),
    }

    // Cleanup goroutine
    go rl.cleanupLoop()

    return rl
}

// Allow checks if request is allowed
func (rl *RateLimiter) Allow(clientID string) bool {
    rl.mu.Lock()
    defer rl.mu.Unlock()

    limiter, exists := rl.clients[clientID]
    if !exists {
        limiter = rate.NewLimiter(rl.limit, rl.burst)
        rl.clients[clientID] = limiter
    }
    rl.lastSeen[clientID] = time.Now()

    return limiter.Allow()
}

func (rl *RateLimiter) cleanupLoop() {
    ticker := time.NewTicker(rl.cleanup)
    for range ticker.C {
        rl.mu.Lock()
        cutoff := time.Now().Add(-rl.cleanup)
        for client, lastSeen := range rl.lastSeen {
            if lastSeen.Before(cutoff) {
                delete(rl.clients, client)
                delete(rl.lastSeen, client)
            }
        }
        rl.mu.Unlock()
    }
}

// MetricsCache caches expensive metrics calculations
type MetricsCache struct {
    mu        sync.RWMutex
    data      *MetricsResponse
    updatedAt time.Time
    ttl       time.Duration
}

func NewMetricsCache(ttl time.Duration) *MetricsCache {
    return &MetricsCache{
        ttl: ttl,
    }
}

func (mc *MetricsCache) Get() *MetricsResponse {
    mc.mu.RLock()
    defer mc.mu.RUnlock()

    if mc.data == nil || time.Since(mc.updatedAt) > mc.ttl {
        return nil
    }
    return mc.data
}

func (mc *MetricsCache) Set(data *MetricsResponse) {
    mc.mu.Lock()
    defer mc.mu.Unlock()
    mc.data = data
    mc.updatedAt = time.Now()
}

// RateLimitMiddleware wraps handler with rate limiting
func RateLimitMiddleware(rl *RateLimiter, next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        clientID := r.RemoteAddr
        if !rl.Allow(clientID) {
            http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
            return
        }
        next(w, r)
    }
}
```

### 3.3 Token Tracker 구현

**파일**: `src/domain/token/tracker.go` (새 파일)

```go
package token

import (
    "sync"
    "time"
)

// Usage represents token usage statistics
type Usage struct {
    Total     int64   `json:"total"`
    Input     int64   `json:"input"`
    Output    int64   `json:"output"`
    Embedding int64   `json:"embedding"`
    CostUSD   float64 `json:"cost_usd"`
}

// HourlyUsage represents usage for a specific hour
type HourlyUsage struct {
    Hour   time.Time `json:"hour"`
    Tokens int64     `json:"tokens"`
}

// Breakdown represents usage by operation type
type Breakdown struct {
    Operation string `json:"operation"`
    Count     int64  `json:"count"`
    Tokens    int64  `json:"tokens"`
}

// Tracker tracks token usage
type Tracker struct {
    mu      sync.RWMutex
    daily   map[string]*Usage // "2024-01-15" -> Usage
    hourly  map[string]int64  // "2024-01-15T10" -> tokens
    byOp    map[string]int64  // "embed" -> tokens
    opCount map[string]int64  // "embed" -> count
}

// NewTracker creates a new token tracker
func NewTracker() *Tracker {
    return &Tracker{
        daily:   make(map[string]*Usage),
        hourly:  make(map[string]int64),
        byOp:    make(map[string]int64),
        opCount: make(map[string]int64),
    }
}

// Record records token usage
func (t *Tracker) Record(operation string, input, output int64) {
    t.mu.Lock()
    defer t.mu.Unlock()

    now := time.Now()
    dayKey := now.Format("2006-01-02")
    hourKey := now.Format("2006-01-02T15")

    // Update daily
    if t.daily[dayKey] == nil {
        t.daily[dayKey] = &Usage{}
    }
    t.daily[dayKey].Input += input
    t.daily[dayKey].Output += output
    t.daily[dayKey].Total += input + output
    t.daily[dayKey].CostUSD += calculateCost(input, output)

    // Update hourly
    t.hourly[hourKey] += input + output

    // Update by operation
    t.byOp[operation] += input + output
    t.opCount[operation]++
}

// GetUsage returns usage for a time range
func (t *Tracker) GetUsage(start, end time.Time) Usage {
    t.mu.RLock()
    defer t.mu.RUnlock()

    var total Usage
    for date, usage := range t.daily {
        d, _ := time.Parse("2006-01-02", date)
        if d.After(start) && d.Before(end) {
            total.Total += usage.Total
            total.Input += usage.Input
            total.Output += usage.Output
            total.CostUSD += usage.CostUSD
        }
    }
    return total
}

// GetHourly returns hourly usage
func (t *Tracker) GetHourly(start, end time.Time) []HourlyUsage {
    t.mu.RLock()
    defer t.mu.RUnlock()

    var result []HourlyUsage
    for hourStr, tokens := range t.hourly {
        hour, _ := time.Parse("2006-01-02T15", hourStr)
        if hour.After(start) && hour.Before(end) {
            result = append(result, HourlyUsage{
                Hour:   hour,
                Tokens: tokens,
            })
        }
    }
    return result
}

// GetBreakdown returns usage by operation
func (t *Tracker) GetBreakdown() []Breakdown {
    t.mu.RLock()
    defer t.mu.RUnlock()

    var result []Breakdown
    for op, tokens := range t.byOp {
        result = append(result, Breakdown{
            Operation: op,
            Tokens:    tokens,
            Count:     t.opCount[op],
        })
    }
    return result
}

// Cost calculation (example rates)
func calculateCost(input, output int64) float64 {
    inputRate := 0.00001   // $0.01 per 1000 tokens
    outputRate := 0.00003  // $0.03 per 1000 tokens
    return float64(input)*inputRate + float64(output)*outputRate
}
```

---

## Phase 4: TUI Integration (Week 9-11)

### 4.1 TUI 에러 처리 개선

**파일**: `src/interface/tui/update.go` 수정

```go
// Model에 에러 상태 추가
type Model struct {
    // ... existing fields ...
    lastError    error
    errorTimeout time.Time
}

// SetError sets an error to display
func (m *Model) SetError(err error) {
    m.lastError = err
    m.errorTimeout = time.Now().Add(5 * time.Second)
}

// ClearError clears the error if expired
func (m *Model) ClearError() {
    if time.Now().After(m.errorTimeout) {
        m.lastError = nil
    }
}

// executeLeave with proper error handling
func (m *Model) executeLeave() error {
    client := daemon.NewClient()
    if !client.IsRunning() {
        err := fmt.Errorf("데몬이 실행 중이 아닙니다")
        m.SetError(err)
        return err
    }

    err := client.Leave()
    if err != nil {
        m.SetError(fmt.Errorf("클러스터 탈퇴 실패: %w", err))
        return err
    }

    m.projectName = ""
    m.nodeID = ""
    m.peerCount = 0
    m.SetResult("클러스터 탈퇴 완료", nil)

    return nil
}

// fetchMetrics with daemon client
func (m Model) fetchMetrics() tea.Cmd {
    return func() tea.Msg {
        client := daemon.NewClient()
        if !client.IsRunning() {
            return MetricsMsg{} // Empty metrics
        }

        metrics, err := client.Metrics()
        if err != nil {
            // Log error but don't show to user (background refresh)
            return MetricsMsg{}
        }

        return MetricsMsg{
            CPUUsage:    metrics.System.CPUUsage,
            MemUsage:    float64(metrics.System.MemoryUsage),
            NetUpload:   metrics.Network.BytesSent,
            NetDownload: metrics.Network.BytesReceived,
            TokensRate:  metrics.Application.RequestsPerSec,
        }
    }
}
```

### 4.2 View 타입 어설션 with 에러 처리

**파일**: `src/interface/tui/views/context.go` 수정

```go
// Update updates the view with new data
func (v *ContextView) Update(data interface{}) error {
    switch d := data.(type) {
    case ContextData:
        v.totalEmbeddings = d.TotalEmbeddings
        v.databaseSize = d.DatabaseSize
        v.syncProgress = d.SyncProgress
        v.recentDeltas = d.RecentDeltas
        v.lastUpdated = time.Now()
        return nil

    case ContextMsg:
        // TUI message type
        v.totalEmbeddings = d.TotalEmbeddings
        v.databaseSize = d.DatabaseSize
        v.syncProgress = d.SyncProgress
        v.lastUpdated = time.Now()
        return nil

    default:
        return fmt.Errorf("unexpected data type: %T (expected ContextData or ContextMsg)", data)
    }
}

// SetData is the old interface (deprecated)
// Deprecated: Use Update instead
func (v *ContextView) SetData(data interface{}) {
    if err := v.Update(data); err != nil {
        // Log but don't crash
        fmt.Fprintf(os.Stderr, "warning: %v\n", err)
    }
}
```

---

## Phase 5: Hook Refactoring (Week 12)

### 5.1 상수 추출

**파일**: `plugin/hooks/lib/constants.mjs` (새 파일)

```javascript
// Timeout constants (in milliseconds)
export const TIMEOUTS = {
    CLUSTER_STATUS: 3000,
    LOCK_ACQUIRE: 5000,
    LOCK_RELEASE: 3000,
    CONTEXT_SHARE: 5000,
    DEFAULT: 3000,
};

// Path constants
export const PATHS = {
    getDataDir: () => {
        return process.env.AGENT_COLLAB_DATA_DIR ||
            join(process.env.HOME || process.env.USERPROFILE || '/tmp', '.agent-collab');
    },
    LOCK_STATE_FILE: 'hook-locks.json',
};

// Lock constants
export const LOCK = {
    START_LINE_DEFAULT: 1,
    END_LINE_ENTIRE_FILE: -1,
};

// Detection constants
export const DETECTION = {
    MIN_PROMPT_LENGTH: 5,
    MAX_SANITIZED_LENGTH: 10000,
    MAX_SIMILAR_CONTEXTS: 10,
};
```

### 5.2 구조화된 로깅

**파일**: `plugin/hooks/lib/logger.mjs` (새 파일)

```javascript
// Simple structured logger for hooks
export function createLogger(component) {
    const log = (level, msg, meta = {}) => {
        // Output to stderr (Claude Code reads stdout for hook response)
        console.error(JSON.stringify({
            level,
            component,
            message: msg,
            ...meta,
            timestamp: new Date().toISOString(),
        }));
    };

    return {
        debug: (msg, meta) => log('DEBUG', msg, meta),
        info: (msg, meta) => log('INFO', msg, meta),
        warn: (msg, meta) => log('WARN', msg, meta),
        error: (msg, meta) => log('ERROR', msg, meta),
    };
}
```

### 5.3 훅 파일 리팩토링 예시

**파일**: `plugin/hooks/pre-tool-enforcer.mjs` 수정

```javascript
#!/usr/bin/env node

import { execSync } from 'child_process';
import { writeFileSync, readFileSync, existsSync, mkdirSync } from 'fs';
import { join } from 'path';

// Import shared modules
import { TIMEOUTS, PATHS, LOCK } from './lib/constants.mjs';
import { createLogger } from './lib/logger.mjs';

const logger = createLogger('hook:pre-tool-enforcer');

// ... rest of implementation using constants and logger ...

function getClusterStatus() {
    try {
        const result = execSync('agent-collab mcp call cluster_status \'{}\'', {
            encoding: 'utf-8',
            timeout: TIMEOUTS.CLUSTER_STATUS,
            stdio: ['pipe', 'pipe', 'pipe']
        });
        const status = JSON.parse(result);
        return {
            active: status.running === true && (status.peer_count || 0) > 0,
            peerCount: status.peer_count || 0,
        };
    } catch (err) {
        logger.warn('failed to check cluster status', {
            error: err.message,
            code: err.code,
        });
        return { active: false, peerCount: 0 };
    }
}
```

---

## Phase 6: Migration & Cleanup (Week 13-14)

### 6.1 Call Site 마이그레이션

```bash
# 1. deprecated 함수 사용처 찾기
grep -rn "NewSemanticLock(" src/ --include="*.go" | grep -v "_test.go"

# 2. gofmt 리라이트로 일괄 변환
gofmt -r 'NewSemanticLock(a, b, c, d) -> NewSemanticLockSafe(a, b, c, d)' -w src/

# 3. 컴파일해서 에러 확인 및 수정
go build ./...

# 4. 테스트 실행
go test ./...
```

### 6.2 fmt.Printf 제거

```bash
# 1. 남은 fmt.Printf 확인
grep -rn "fmt.Printf" src/ --include="*.go" | grep -v "_test.go"

# 2. 파일별로 수동 마이그레이션 (자동화 어려움)
# - logger.Info/Warn/Error로 변경
# - 필요시 컨텍스트 필드 추가
```

---

## 수정된 타임라인

| Phase | 내용 | 기간 | 누적 |
|-------|------|------|------|
| 1 | Foundation (로깅, 에러, 상수) | 2주 | 2주 |
| 2 | Structured Logging (점진적) | 2주 | 4주 |
| 3 | API Endpoints (leave, metrics, token) | 4주 | 8주 |
| 4 | TUI Integration | 3주 | 11주 |
| 5 | Hook Refactoring | 1주 | 12주 |
| 6 | Migration & Cleanup | 2주 | 14주 |

**총 예상 기간: 14주 (3.5개월)**

---

## 테스트 전략

### Unit Tests

```go
// src/pkg/logging/logger_test.go
func TestLogger_Component(t *testing.T) {
    var buf bytes.Buffer
    logger := New(&buf, "info")
    compLogger := logger.Component("test")

    compLogger.Info("test message")

    assert.Contains(t, buf.String(), `"component":"test"`)
}

// src/domain/lock/lock_test.go
func TestNewSemanticLockSafe_Validation(t *testing.T) {
    tests := []struct {
        name      string
        target    *SemanticTarget
        holderID  string
        wantErr   bool
    }{
        {"nil target", nil, "holder", true},
        {"empty holder", &SemanticTarget{}, "", true},
        {"valid", &SemanticTarget{Type: "file"}, "holder", false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := NewSemanticLockSafe(tt.target, tt.holderID, "name", "intention")
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### Integration Tests (Mock Daemon)

```go
// src/interface/tui/update_test.go
func TestExecuteLeave(t *testing.T) {
    // Mock daemon server
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        switch r.URL.Path {
        case "/leave":
            json.NewEncoder(w).Encode(daemon.GenericResponse{Success: true})
        case "/status":
            json.NewEncoder(w).Encode(daemon.StatusResponse{Running: true})
        }
    }))
    defer server.Close()

    // Point client to mock server (via env or injection)
    t.Setenv("AGENT_COLLAB_DAEMON_URL", server.URL)

    model := NewModel()
    err := model.executeLeave()

    assert.NoError(t, err)
    assert.Empty(t, model.projectName)
}
```

---

## 성공 기준

### 코드 품질
- [ ] Production 코드에 panic() 0개
- [ ] 핵심 경로에 fmt.Printf 0개
- [ ] 새 코드 테스트 커버리지 80%+
- [ ] 모든 TODO에 이슈 번호 부여

### 기능
- [ ] /leave 엔드포인트 정상 작동
- [ ] /metrics 5초 캐싱 적용
- [ ] /tokens/usage 일/주/월 통계 제공
- [ ] TUI 모든 탭 라이브 데이터 표시

### 성능
- [ ] 로깅 오버헤드 CPU 5% 미만
- [ ] /metrics 응답 시간 100ms 미만
- [ ] Rate limiting 작동 (10 req/s)

---

## Feature Flags 구현

```go
// src/infrastructure/config/flags.go
package config

type FeatureFlags struct {
    UseStructuredLogging bool `json:"use_structured_logging"`
    EnableMetricsV2      bool `json:"enable_metrics_v2"`
    EnableLeaveEndpoint  bool `json:"enable_leave_endpoint"`
    EnableTokenTracking  bool `json:"enable_token_tracking"`
}

// Load from config file
func LoadFlags(path string) (*FeatureFlags, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return &FeatureFlags{}, nil // Defaults
    }

    var flags FeatureFlags
    if err := json.Unmarshal(data, &flags); err != nil {
        return nil, err
    }
    return &flags, nil
}

// Runtime check
func (f *FeatureFlags) IsEnabled(name string) bool {
    switch name {
    case "structured_logging":
        return f.UseStructuredLogging
    case "metrics_v2":
        return f.EnableMetricsV2
    case "leave":
        return f.EnableLeaveEndpoint
    case "token_tracking":
        return f.EnableTokenTracking
    default:
        return false
    }
}
```

---

## 결론

이 수정된 설계는:

1. **순환 의존성 해결** - 패키지별 에러/상수 정의
2. **점진적 마이그레이션** - Deprecated 함수 유지하며 새 API 추가
3. **현실적 타임라인** - 14주 (vs 원래 3-4주)
4. **적절한 테스트** - Mock 기반 단위/통합 테스트
5. **Feature Flags** - 안전한 롤아웃 지원

비평에서 지적된 모든 Critical/High 이슈를 해결했습니다.
