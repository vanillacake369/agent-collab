# TODO 구현 계획

## 개요

TUI (Terminal User Interface)의 미구현 TODO들을 daemon API와 연동하여 구현하는 계획입니다.

---

## TODO 목록 및 구현 계획

### 1. TUI 액션 실행 함수들 (High Priority)

#### 1.1 `executeInit` - 클러스터 초기화
**파일**: `src/interface/tui/update.go:418-423`

**현재 상태**:
```go
func (m *Model) executeInit(projectName string) error {
    // TODO: 실제 init 로직 연동
    m.projectName = projectName
    m.SetResult("프로젝트 '"+projectName+"' 초기화 완료", nil)
    return nil
}
```

**구현 계획**:
```go
func (m *Model) executeInit(projectName string) error {
    client := daemon.NewClient()

    // 데몬이 실행 중인지 확인
    if !client.IsRunning() {
        return fmt.Errorf("데몬이 실행 중이 아닙니다. 'agent-collab daemon start'를 먼저 실행하세요")
    }

    // Init API 호출
    result, err := client.Init(projectName)
    if err != nil {
        m.SetResult("", fmt.Errorf("초기화 실패: %w", err))
        return err
    }

    m.projectName = projectName
    m.SetResult(fmt.Sprintf("프로젝트 '%s' 초기화 완료\n토큰: %s", projectName, result.Token[:20]+"..."), nil)
    return nil
}
```

**필요한 작업**:
- [x] `daemon.Client.Init()` 메서드 이미 존재 (`client.go:101-117`)
- [ ] TUI에서 에러 핸들링 및 결과 표시 개선

---

#### 1.2 `executeJoin` - 클러스터 참여
**파일**: `src/interface/tui/update.go:425-429`

**현재 상태**:
```go
func (m *Model) executeJoin(token string) error {
    // TODO: 실제 join 로직 연동
    m.SetResult("클러스터 참여 완료 (토큰: "+token[:min(10, len(token))]+"...)", nil)
    return nil
}
```

**구현 계획**:
```go
func (m *Model) executeJoin(token string) error {
    client := daemon.NewClient()

    if !client.IsRunning() {
        return fmt.Errorf("데몬이 실행 중이 아닙니다")
    }

    result, err := client.Join(token)
    if err != nil {
        m.SetResult("", fmt.Errorf("클러스터 참여 실패: %w", err))
        return err
    }

    m.projectName = result.ProjectName
    m.SetResult(fmt.Sprintf("클러스터 '%s' 참여 완료\n피어 수: %d", result.ProjectName, result.PeerCount), nil)
    return nil
}
```

**필요한 작업**:
- [x] `daemon.Client.Join()` 메서드 이미 존재 (`client.go:119-135`)
- [ ] `JoinResponse`에 `PeerCount` 필드 추가 필요

---

#### 1.3 `executeLeave` - 클러스터 탈퇴
**파일**: `src/interface/tui/update.go:431-435`

**현재 상태**:
```go
func (m *Model) executeLeave() error {
    // TODO: 실제 leave 로직 연동
    m.SetResult("클러스터 탈퇴 완료", nil)
    return nil
}
```

**구현 계획**:
```go
func (m *Model) executeLeave() error {
    client := daemon.NewClient()

    if !client.IsRunning() {
        return fmt.Errorf("데몬이 실행 중이 아닙니다")
    }

    err := client.Leave()
    if err != nil {
        m.SetResult("", fmt.Errorf("클러스터 탈퇴 실패: %w", err))
        return err
    }

    m.projectName = ""
    m.SetResult("클러스터 탈퇴 완료", nil)
    return nil
}
```

**필요한 작업**:
- [ ] `daemon.Client.Leave()` 메서드 추가 필요
- [ ] `/leave` API 엔드포인트 추가 필요 (daemon server)
- [ ] P2P 연결 해제 로직 구현

---

#### 1.4 `executeReleaseLock` - 락 해제
**파일**: `src/interface/tui/update.go:437-441`

**현재 상태**:
```go
func (m *Model) executeReleaseLock(lockID string) error {
    // TODO: 실제 lock release 로직 연동
    m.SetResult("락 '"+lockID+"' 해제 완료", nil)
    return nil
}
```

**구현 계획**:
```go
func (m *Model) executeReleaseLock(lockID string) error {
    client := daemon.NewClient()

    if !client.IsRunning() {
        return fmt.Errorf("데몬이 실행 중이 아닙니다")
    }

    err := client.ReleaseLock(lockID)
    if err != nil {
        m.SetResult("", fmt.Errorf("락 해제 실패: %w", err))
        return err
    }

    m.SetResult(fmt.Sprintf("락 '%s' 해제 완료", lockID), nil)
    return nil
}
```

**필요한 작업**:
- [x] `daemon.Client.ReleaseLock()` 메서드 이미 존재 (`client.go:157-173`)
- [ ] TUI에서 락 목록 새로고침 로직 추가

---

### 2. TUI 데이터 Fetch 함수들 (Medium Priority)

#### 2.1 `fetchMetrics` - 시스템 메트릭
**파일**: `src/interface/tui/update.go:475-486`

**현재 상태**:
```go
func (m Model) fetchMetrics() tea.Cmd {
    return func() tea.Msg {
        // TODO: 실제 메트릭 가져오기 (daemon 연동)
        return MetricsMsg{
            CPUUsage:    0,
            MemUsage:    0,
            NetUpload:   0,
            NetDownload: 0,
            TokensRate:  0,
        }
    }
}
```

**구현 계획**:

**Option A: 시스템 메트릭 (runtime 패키지 사용)**
```go
func (m Model) fetchMetrics() tea.Cmd {
    return func() tea.Msg {
        var memStats runtime.MemStats
        runtime.ReadMemStats(&memStats)

        return MetricsMsg{
            CPUUsage:    getCPUUsage(),        // 별도 구현 필요
            MemUsage:    float64(memStats.Alloc) / float64(memStats.Sys) * 100,
            NetUpload:   0,                    // 네트워크 메트릭 구현 필요
            NetDownload: 0,
            TokensRate:  0,                    // MCP 토큰 추적 필요
        }
    }
}
```

**Option B: Daemon에 /metrics API 추가**
```go
// daemon/types.go
type MetricsResponse struct {
    CPUUsage    float64 `json:"cpu_usage"`
    MemUsage    float64 `json:"mem_usage"`
    NetUpload   int64   `json:"net_upload"`
    NetDownload int64   `json:"net_download"`
    GoroutineCount int  `json:"goroutine_count"`
}

// daemon/server.go에 /metrics 엔드포인트 추가
```

**권장**: Option B - Daemon에서 중앙 집중식으로 메트릭 관리

**필요한 작업**:
- [ ] `MetricsResponse` 타입 정의
- [ ] `/metrics` API 엔드포인트 추가
- [ ] `daemon.Client.Metrics()` 메서드 추가
- [ ] 네트워크 I/O 추적 (libp2p 메트릭 활용)

---

#### 2.2 `fetchContext` - 벡터 DB 컨텍스트
**파일**: `src/interface/tui/update.go:585-595`

**현재 상태**:
```go
func (m Model) fetchContext() tea.Cmd {
    return func() tea.Msg {
        // TODO: 실제 컨텍스트 가져오기 (daemon 연동)
        return ContextMsg{
            TotalEmbeddings: 0,
            DatabaseSize:    0,
            SyncProgress:    nil,
            RecentDeltas:    nil,
        }
    }
}
```

**구현 계획**:
```go
// daemon/types.go
type ContextStatsResponse struct {
    TotalEmbeddings int               `json:"total_embeddings"`
    DatabaseSize    int64             `json:"database_size"`
    SyncProgress    map[string]float64 `json:"sync_progress"`
    RecentDeltas    []DeltaInfo       `json:"recent_deltas"`
}

// tui/update.go
func (m Model) fetchContext() tea.Cmd {
    return func() tea.Msg {
        client := daemon.NewClient()
        if !client.IsRunning() {
            return ContextMsg{}
        }

        stats, err := client.ContextStats()
        if err != nil {
            return ContextMsg{}
        }

        return ContextMsg{
            TotalEmbeddings: stats.TotalEmbeddings,
            DatabaseSize:    stats.DatabaseSize,
            SyncProgress:    stats.SyncProgress,
            RecentDeltas:    convertDeltas(stats.RecentDeltas),
        }
    }
}
```

**필요한 작업**:
- [ ] `ContextStatsResponse` 타입 정의
- [ ] `/context/stats` API 엔드포인트 추가
- [ ] `daemon.Client.ContextStats()` 메서드 추가
- [ ] VectorDB에서 통계 쿼리 구현

---

#### 2.3 `fetchTokens` - MCP 토큰 사용량
**파일**: `src/interface/tui/update.go:597-607`

**현재 상태**:
```go
func (m Model) fetchTokens() tea.Cmd {
    return func() tea.Msg {
        // TODO: 실제 토큰 사용량 가져오기 (daemon 연동)
        return TokensMsg{
            TodayUsed:   0,
            WeeklyUsed:  0,
            MonthlyUsed: 0,
            Limit:       0,
        }
    }
}
```

**구현 계획**:

토큰 사용량은 MCP 서버에서 추적해야 합니다:

```go
// domain/token/tracker.go (새 파일)
type TokenTracker struct {
    mu          sync.RWMutex
    dailyUsage  map[string]int64  // date -> count
    weeklyUsage int64
    monthlyUsage int64
}

func (t *TokenTracker) Record(tokens int) {
    t.mu.Lock()
    defer t.mu.Unlock()
    today := time.Now().Format("2006-01-02")
    t.dailyUsage[today] += int64(tokens)
}

// daemon/types.go
type TokenUsageResponse struct {
    TodayUsed   int64 `json:"today_used"`
    WeeklyUsed  int64 `json:"weekly_used"`
    MonthlyUsed int64 `json:"monthly_used"`
    Limit       int64 `json:"limit"`
}
```

**필요한 작업**:
- [ ] `TokenTracker` 도메인 객체 생성
- [ ] MCP 요청/응답 시 토큰 카운트 추적
- [ ] `/tokens/usage` API 엔드포인트 추가
- [ ] 일/주/월 집계 로직 구현

---

### 3. TUI View 타입 어설션 (Low Priority)

#### 3.1 `ContextView.SetData`
**파일**: `src/interface/tui/views/context.go:48-50`

**구현 계획**:
```go
func (v *ContextView) SetData(data interface{}) {
    if d, ok := data.(ContextData); ok {
        v.totalEmbeddings = d.TotalEmbeddings
        v.databaseSize = d.DatabaseSize
        v.syncProgress = d.SyncProgress
        v.recentDeltas = d.RecentDeltas
        v.lastUpdated = time.Now()
    }
}
```

#### 3.2 `LocksView.SetLocks`
**파일**: `src/interface/tui/views/locks.go:35-37`

**구현 계획**:
```go
func (v *LocksView) SetLocks(locks interface{}) {
    if l, ok := locks.([]LockInfo); ok {
        v.locks = l
    }
}
```

#### 3.3 `PeersView.SetPeers`
**파일**: `src/interface/tui/views/peers.go:40-42`

**구현 계획**:
```go
func (v *PeersView) SetPeers(peers interface{}) {
    if p, ok := peers.([]PeerInfo); ok {
        v.peers = p
    }
}
```

#### 3.4 `TokensView.SetData`
**파일**: `src/interface/tui/views/tokens.go:42-44`

**구현 계획**:
```go
func (v *TokensView) SetData(data interface{}) {
    if d, ok := data.(TokensData); ok {
        v.todayUsed = d.TodayUsed
        v.weeklyUsed = d.WeeklyUsed
        v.monthlyUsed = d.MonthlyUsed
        v.limit = d.Limit
    }
}
```

---

### 4. Sync Health 계산
**파일**: `src/interface/tui/app.go:205`

**현재 상태**:
```go
SyncHealth: 100, // TODO: 실제 sync health 계산
```

**구현 계획**:
```go
// Sync health 계산 로직
func calculateSyncHealth(status *daemon.StatusResponse) int {
    if status.PeerCount == 0 {
        return 0  // 피어 없음
    }

    // 기준:
    // - 연결된 피어 수 (목표 대비)
    // - 최근 동기화 성공률
    // - 락 충돌 빈도

    health := 100

    // 피어가 적으면 감소
    if status.PeerCount < 2 {
        health -= 20
    }

    // TODO: 동기화 실패율에 따라 감소
    // TODO: 락 충돌 빈도에 따라 감소

    return max(0, health)
}
```

**필요한 작업**:
- [ ] `StatusResponse`에 동기화 통계 필드 추가
- [ ] 동기화 성공/실패 이벤트 추적
- [ ] 락 충돌 이벤트 추적

---

## 구현 우선순위

### Phase 1: 핵심 기능 (1-2일)
1. `executeInit` - daemon.Init() 연동
2. `executeJoin` - daemon.Join() 연동
3. `executeReleaseLock` - daemon.ReleaseLock() 연동
4. View 타입 어설션 구현 (4개 파일)

### Phase 2: Leave 기능 (1일)
5. `/leave` API 엔드포인트 추가
6. `daemon.Client.Leave()` 메서드 추가
7. `executeLeave` 구현
8. P2P 연결 해제 로직

### Phase 3: 메트릭 및 통계 (2-3일)
9. `/metrics` API 및 `fetchMetrics` 구현
10. `/context/stats` API 및 `fetchContext` 구현
11. `calculateSyncHealth` 구현

### Phase 4: 토큰 추적 (2일)
12. `TokenTracker` 도메인 객체
13. MCP 토큰 카운트 연동
14. `/tokens/usage` API 및 `fetchTokens` 구현

---

## 새로 추가해야 할 API 엔드포인트

| 엔드포인트 | 메서드 | 설명 |
|-----------|--------|------|
| `/leave` | POST | 클러스터 탈퇴 |
| `/metrics` | GET | 시스템 메트릭 |
| `/context/stats` | GET | 벡터 DB 통계 |
| `/tokens/usage` | GET | MCP 토큰 사용량 |

---

## 새로 추가해야 할 타입

```go
// daemon/types.go에 추가

type LeaveResponse struct {
    Success bool   `json:"success"`
    Error   string `json:"error,omitempty"`
}

type MetricsResponse struct {
    CPUUsage       float64 `json:"cpu_usage"`
    MemUsage       float64 `json:"mem_usage"`
    MemAllocMB     int64   `json:"mem_alloc_mb"`
    GoroutineCount int     `json:"goroutine_count"`
    NetUploadBytes int64   `json:"net_upload_bytes"`
    NetDownloadBytes int64 `json:"net_download_bytes"`
}

type ContextStatsResponse struct {
    TotalEmbeddings int                `json:"total_embeddings"`
    DatabaseSizeKB  int64              `json:"database_size_kb"`
    SyncProgress    map[string]float64 `json:"sync_progress"`
    RecentDeltas    []DeltaInfo        `json:"recent_deltas"`
    LastSyncAt      time.Time          `json:"last_sync_at"`
}

type TokenUsageResponse struct {
    TodayUsed   int64     `json:"today_used"`
    WeeklyUsed  int64     `json:"weekly_used"`
    MonthlyUsed int64     `json:"monthly_used"`
    Limit       int64     `json:"limit"`
    ResetAt     time.Time `json:"reset_at"`
}
```

---

## 예상 작업량

| Phase | 작업 | 예상 시간 |
|-------|------|----------|
| Phase 1 | 핵심 기능 | 4-6시간 |
| Phase 2 | Leave 기능 | 3-4시간 |
| Phase 3 | 메트릭/통계 | 6-8시간 |
| Phase 4 | 토큰 추적 | 4-6시간 |
| **총계** | | **17-24시간** |

---

## 테스트 계획

각 TODO 구현 후:
1. 단위 테스트 작성
2. TUI에서 수동 테스트
3. daemon API 통합 테스트
4. 에러 케이스 테스트 (데몬 미실행, 네트워크 오류 등)
