# agent-collab 구현 계획서 v4

## 1. 시스템 개요

**목적**: 서로 다른 네트워크에 있는 개발자들의 에이전트를 P2P로 연결하여, 중앙 서버 없이 컨텍스트를 공유하고 코드 작성 시점에 충돌을 사전 예방하는 분산 오퍼레이션 환경.

**UI**: Cobra CLI + Bubbletea TUI 대시보드

---

## 2. 기술 스택

### 2.1 핵심 라이브러리

| 영역 | 라이브러리 | 버전 | 용도 |
|------|-----------|------|------|
| **CLI** | `spf13/cobra` | v1.8+ | 명령어 파싱 |
| **TUI** | `charmbracelet/bubbletea` | v0.25+ | 인터랙티브 TUI |
| **TUI 컴포넌트** | `charmbracelet/bubbles` | v0.18+ | 테이블, 리스트, 스피너 등 |
| **TUI 스타일** | `charmbracelet/lipgloss` | v0.10+ | 색상, 레이아웃 |
| **설정** | `spf13/viper` | v1.18+ | 설정 관리 |
| P2P 네트워크 | `go-libp2p` | v0.36+ | 핵심 네트워킹 |
| QUIC | `quic-go` | (내장) | 고성능 전송 |
| WebRTC | `pion/webrtc` | v3+ | NAT 통과 |
| DHT | `go-libp2p-kad-dht` | v0.26+ | Peer 탐색 |
| PubSub | `go-libp2p-pubsub` | v0.11+ | Gossipsub |
| Vector DB | `milvus-io/milvus-lite` | v2.3+ | 임베딩 저장 |
| CRDT | `automerge/automerge-go` | v0.2+ | 메타데이터 동기화 |
| AST 파싱 | `smacker/go-tree-sitter` | latest | 다중 언어 파싱 |
| 로깅 | `uber-go/zap` | v1.27+ | 구조화된 로깅 |

---

## 3. CLI 명령어 구조

```
agent-collab
│
├── init                        # 클러스터 초기화
│   └── --project, -p <name>    # 프로젝트 이름 (필수)
│
├── join <token>                # 클러스터 참여
│   └── --name, -n <name>       # 표시 이름 (선택)
│
├── leave                       # 클러스터 탈퇴
│   └── --force, -f             # 강제 탈퇴
│
├── status                      # 간단한 상태 출력 (non-interactive)
│   ├── --json                  # JSON 출력
│   └── --watch, -w             # 실시간 갱신
│
├── dashboard                   # TUI 대시보드 (interactive)
│   └── --tab, -t <name>        # 시작 탭 지정
│
├── token                       # 토큰 관리
│   ├── show                    # 현재 초대 토큰 표시
│   ├── refresh                 # 토큰 갱신
│   └── usage                   # 사용량 통계
│       ├── --period <day|week|month>
│       └── --json
│
├── lock                        # 락 관리
│   ├── list                    # 현재 락 목록
│   ├── release <lock-id>       # 락 강제 해제
│   └── history                 # 락 히스토리
│
├── peers                       # Peer 관리
│   ├── list                    # Peer 목록
│   ├── info <peer-id>          # Peer 상세 정보
│   └── ban <peer-id>           # Peer 차단
│
├── config                      # 설정 관리
│   ├── show                    # 현재 설정 출력
│   ├── set <key> <value>       # 설정 변경
│   └── reset                   # 기본값 복원
│
└── version                     # 버전 정보
```

---

## 4. TUI 대시보드 설계

### 4.1 전체 레이아웃

```
┌─────────────────────────────────────────────────────────────────────┐
│  🔗 agent-collab v1.0.0                                             │
│  Project: my-awesome-project | Node: QmXx...Yy                      │
│  Status: ● Connected | Peers: 4 | Sync: 98.5% | Uptime: 2h 34m     │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌─[1]Cluster─┬─[2]Context─┬─[3]Locks─┬─[4]Tokens─┬─[5]Peers─┐     │
│  │            │            │          │           │          │     │
│                                                                     │
│  ╔═══════════════════════════════════════════════════════════════╗ │
│  ║                                                               ║ │
│  ║                    << TAB CONTENT >>                          ║ │
│  ║                                                               ║ │
│  ║                                                               ║ │
│  ║                                                               ║ │
│  ║                                                               ║ │
│  ║                                                               ║ │
│  ║                                                               ║ │
│  ║                                                               ║ │
│  ╚═══════════════════════════════════════════════════════════════╝ │
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│  [q]Quit [r]Refresh [1-5]Tab [↑↓]Navigate [Enter]Select [?]Help    │
│  CPU: 2.3% | MEM: 45MB | NET: ↑12KB/s ↓8KB/s | Tokens: 1.2K/hr     │
└─────────────────────────────────────────────────────────────────────┘
```

### 4.2 탭별 화면

#### [1] Cluster 탭

```
┌─ Cluster Overview ──────────────────────────────────────────────────┐
│                                                                     │
│  Health Score: ████████████████████░░░░ 85%  [Healthy]             │
│                                                                     │
│  ┌─ Network Topology ───────────────────────────────────────────┐  │
│  │                                                               │  │
│  │           [Alice]                                             │  │
│  │              │                                                │  │
│  │    [You] ────┼──── [Bob]                                      │  │
│  │              │                                                │  │
│  │           [Charlie]                                           │  │
│  │                                                               │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌─ Quick Stats ────────────────────────────────────────────────┐  │
│  │  Total Peers      : 4                                        │  │
│  │  Active Locks     : 2                                        │  │
│  │  Pending Syncs    : 0                                        │  │
│  │  Avg Latency      : 34ms                                     │  │
│  │  Messages/sec     : 12.4                                     │  │
│  └───────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

#### [2] Context 탭

```
┌─ Context Sync Status ───────────────────────────────────────────────┐
│                                                                     │
│  Vector Database                                                    │
│  ├─ Total Embeddings : 12,456                                      │
│  ├─ Database Size    : 234.5 MB                                    │
│  └─ Last Updated     : 2 seconds ago                               │
│                                                                     │
│  ┌─ Sync Progress ──────────────────────────────────────────────┐  │
│  │  Alice   ████████████████████  100% (synced)                 │  │
│  │  Bob     ████████████████░░░░   82% (syncing...)             │  │
│  │  Charlie ████████████████████  100% (synced)                 │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌─ Recent Deltas ──────────────────────────────────────────────┐  │
│  │  TIME      FROM     FILES    SIZE     STATUS                 │  │
│  │  12:34:56  Alice    3        12KB     ✓ Applied              │  │
│  │  12:34:45  Bob      1        4KB      ✓ Applied              │  │
│  │  12:34:30  You      5        28KB     ✓ Propagated           │  │
│  │  12:34:12  Charlie  2        8KB      ✓ Applied              │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌─ Indexed Files by Language ──────────────────────────────────┐  │
│  │  Go         ████████████████████  456 files (65%)            │  │
│  │  TypeScript ██████░░░░░░░░░░░░░░  124 files (18%)            │  │
│  │  Python     ████░░░░░░░░░░░░░░░░   78 files (11%)            │  │
│  │  Other      ██░░░░░░░░░░░░░░░░░░   42 files (6%)             │  │
│  └───────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

#### [3] Locks 탭

```
┌─ Semantic Locks ────────────────────────────────────────────────────┐
│                                                                     │
│  Active Locks: 2                                                    │
│                                                                     │
│  ┌─ Current Locks ──────────────────────────────────────────────┐  │
│  │  HOLDER   TARGET                      INTENTION    TTL       │  │
│  │  ● Alice  src/auth/login.go:45-67     리팩토링     25s       │  │
│  │  ● Bob    pkg/api/handler.go:120-145  버그 수정   18s       │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌─ Pending Requests ───────────────────────────────────────────┐  │
│  │  REQUESTER  TARGET                    STATUS                 │  │
│  │  ○ Charlie  src/auth/login.go:60-80   Waiting (conflict)    │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌─ Recent Activity ────────────────────────────────────────────┐  │
│  │  12:34:56  Alice acquired lock on login.go                   │  │
│  │  12:34:45  You released lock on config.go                    │  │
│  │  12:34:30  Bob acquired lock on handler.go                   │  │
│  │  12:33:12  Conflict resolved: Charlie → Alice                │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  [l] View Lock Details  [r] Release My Lock  [h] History           │
└─────────────────────────────────────────────────────────────────────┘
```

#### [4] Tokens 탭

```
┌─ Token Usage ───────────────────────────────────────────────────────┐
│                                                                     │
│  Today's Usage                                                      │
│  ████████████████░░░░░░░░░░░░░░  52% (104,521 / 200,000)           │
│                                                                     │
│  ┌─ Usage Breakdown ────────────────────────────────────────────┐  │
│  │                                                               │  │
│  │  Embedding Generation                                         │  │
│  │  ████████████████░░░░  78,234 tokens (75%)      $0.078       │  │
│  │                                                               │  │
│  │  Context Synchronization                                      │  │
│  │  ████████░░░░░░░░░░░░  21,123 tokens (20%)      $0.021       │  │
│  │                                                               │  │
│  │  Lock Negotiation                                             │  │
│  │  █░░░░░░░░░░░░░░░░░░░   5,164 tokens (5%)       $0.005       │  │
│  │                                                               │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌─ Hourly Trend ───────────────────────────────────────────────┐  │
│  │  15K ┤                                                        │  │
│  │  10K ┤        ▄▄                                              │  │
│  │   5K ┤   ▄▄  ████  ▄▄▄▄                                      │  │
│  │   0K ┼───██──████──████──▄▄────────────────────────          │  │
│  │       00  04  08  12  16  20  (hours)                        │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌─ Period Summary ─────────────────────────────────────────────┐  │
│  │  Today      : 104,521 tokens     Est. $0.10                  │  │
│  │  This Week  : 623,456 tokens     Est. $0.62                  │  │
│  │  This Month : 2,345,678 tokens   Est. $2.35                  │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  [d] Daily  [w] Weekly  [m] Monthly  [e] Export                    │
└─────────────────────────────────────────────────────────────────────┘
```

#### [5] Peers 탭

```
┌─ Connected Peers ───────────────────────────────────────────────────┐
│                                                                     │
│  Total: 4 peers | Online: 4 | Syncing: 0                           │
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐ │
│  │  STATUS  NAME     PEER ID         TRANSPORT  LATENCY  SYNC   │ │
│  ├───────────────────────────────────────────────────────────────┤ │
│  │  ● ──── Alice    QmAbc...123     QUIC       12ms     100%   │ │
│  │  ● ──── Bob      QmDef...456     WebRTC     45ms     100%   │ │
│  │  ● ──── Charlie  QmGhi...789     TCP        89ms     100%   │ │
│  │  ● ──── Diana    QmJkl...012     QUIC       23ms     100%   │ │
│  └───────────────────────────────────────────────────────────────┘ │
│                                                                     │
│  ┌─ Selected: Alice ────────────────────────────────────────────┐  │
│  │  Peer ID    : QmAbc...123                                    │  │
│  │  Name       : Alice                                          │  │
│  │  Connected  : 2 hours ago                                    │  │
│  │  Transport  : QUIC (UDP)                                     │  │
│  │  Address    : /ip4/192.168.1.100/udp/4001/quic-v1           │  │
│  │  Latency    : 12ms (avg), 8ms (min), 23ms (max)             │  │
│  │  Messages   : ↑ 1,234  ↓ 2,345                              │  │
│  │  Sync       : 100% (12,456 vectors)                         │  │
│  │  Caps       : [embedding] [lock] [context]                  │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  [Enter] Details  [p] Ping  [b] Ban  [c] Copy ID                   │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 5. 프로젝트 구조

```
agent-collab/
├── cmd/
│   └── agent-collab/
│       └── main.go                      # 진입점
│
├── internal/
│   ├── domain/                          # 도메인 레이어
│   │   ├── cluster/
│   │   │   ├── config.go
│   │   │   ├── node.go
│   │   │   └── events.go
│   │   ├── context/
│   │   │   ├── delta.go
│   │   │   ├── vibe.go
│   │   │   └── sync.go
│   │   ├── lock/
│   │   │   ├── semantic_lock.go
│   │   │   ├── target.go
│   │   │   ├── negotiator.go
│   │   │   └── conflict.go
│   │   ├── peer/
│   │   │   ├── peer.go
│   │   │   ├── capability.go
│   │   │   └── discovery.go
│   │   └── token/                       # 토큰 추적 도메인
│   │       ├── metrics.go
│   │       ├── usage.go
│   │       └── estimator.go
│   │
│   ├── application/                     # 유스케이스 레이어
│   │   ├── init/
│   │   │   └── usecase.go
│   │   ├── join/
│   │   │   └── usecase.go
│   │   ├── status/
│   │   │   └── usecase.go
│   │   ├── lock/
│   │   │   └── usecase.go
│   │   └── token/
│   │       └── usecase.go
│   │
│   ├── infrastructure/                  # 인프라 레이어
│   │   ├── network/
│   │   │   └── libp2p/
│   │   │       ├── node.go
│   │   │       ├── transports.go
│   │   │       ├── discovery.go
│   │   │       ├── nat.go
│   │   │       ├── pubsub.go
│   │   │       └── protocol/
│   │   │           ├── lock.go
│   │   │           └── context.go
│   │   ├── storage/
│   │   │   ├── vector/
│   │   │   │   └── milvus.go
│   │   │   ├── crdt/
│   │   │   │   └── automerge.go
│   │   │   └── metrics/
│   │   │       └── token_store.go       # 토큰 사용량 저장
│   │   ├── crypto/
│   │   │   ├── keys.go
│   │   │   └── invite_token.go
│   │   └── embedding/
│   │       └── transformer.go
│   │
│   └── interface/                       # 인터페이스 레이어
│       ├── cli/                         # Cobra CLI
│       │   ├── root.go
│       │   ├── init.go
│       │   ├── join.go
│       │   ├── leave.go
│       │   ├── status.go
│       │   ├── dashboard.go             # TUI 진입점
│       │   ├── token.go
│       │   ├── lock.go
│       │   ├── peers.go
│       │   ├── config.go
│       │   └── version.go
│       │
│       ├── tui/                         # Bubbletea TUI
│       │   ├── app.go                   # 앱 초기화
│       │   ├── model.go                 # 메인 모델
│       │   ├── update.go                # Update 로직
│       │   ├── view.go                  # View 렌더링
│       │   ├── keys.go                  # 키 바인딩
│       │   ├── styles.go                # Lipgloss 스타일
│       │   ├── messages.go              # 커스텀 메시지
│       │   │
│       │   ├── components/              # 재사용 컴포넌트
│       │   │   ├── header.go
│       │   │   ├── footer.go
│       │   │   ├── tabs.go
│       │   │   ├── table.go
│       │   │   ├── gauge.go
│       │   │   ├── sparkline.go
│       │   │   └── topology.go          # 네트워크 토폴로지 그래프
│       │   │
│       │   └── views/                   # 탭별 뷰
│       │       ├── cluster.go
│       │       ├── context.go
│       │       ├── locks.go
│       │       ├── tokens.go
│       │       └── peers.go
│       │
│       └── notification/
│           └── lsp/
│               └── server.go
│
├── pkg/                                 # 공개 패키지
│   └── protocol/
│       ├── messages.go
│       └── version.go
│
├── configs/
│   └── default.yaml
│
├── go.mod
├── go.sum
├── Makefile
└── .goreleaser.yaml
```

---

## 6. 핵심 구현 코드

### 6.1 CLI 루트 명령어 (Cobra)

```go
// internal/interface/cli/root.go
package cli

import (
    "os"

    "github.com/spf13/cobra"
    "github.com/spf13/viper"
)

var (
    cfgFile string
    verbose bool
)

var rootCmd = &cobra.Command{
    Use:   "agent-collab",
    Short: "분산 에이전트 협업 시스템",
    Long: `agent-collab은 서로 다른 네트워크의 개발자 에이전트들을
P2P로 연결하여 컨텍스트를 공유하고 충돌을 사전 예방합니다.

시작하기:
  agent-collab init -p my-project   # 새 클러스터 생성
  agent-collab join <token>          # 기존 클러스터 참여
  agent-collab dashboard             # TUI 대시보드 실행`,
}

func Execute() {
    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}

func init() {
    cobra.OnInitialize(initConfig)

    rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "설정 파일 경로")
    rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "상세 출력")

    // 서브커맨드 등록
    rootCmd.AddCommand(initCmd)
    rootCmd.AddCommand(joinCmd)
    rootCmd.AddCommand(leaveCmd)
    rootCmd.AddCommand(statusCmd)
    rootCmd.AddCommand(dashboardCmd)
    rootCmd.AddCommand(tokenCmd)
    rootCmd.AddCommand(lockCmd)
    rootCmd.AddCommand(peersCmd)
    rootCmd.AddCommand(configCmd)
    rootCmd.AddCommand(versionCmd)
}

func initConfig() {
    if cfgFile != "" {
        viper.SetConfigFile(cfgFile)
    } else {
        home, _ := os.UserHomeDir()
        viper.AddConfigPath(home + "/.agent-collab")
        viper.AddConfigPath(".")
        viper.SetConfigName("config")
        viper.SetConfigType("yaml")
    }
    viper.AutomaticEnv()
    viper.ReadInConfig()
}
```

### 6.2 Dashboard 명령어

```go
// internal/interface/cli/dashboard.go
package cli

import (
    "fmt"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/spf13/cobra"

    "agent-collab/internal/interface/tui"
)

var dashboardCmd = &cobra.Command{
    Use:   "dashboard",
    Short: "TUI 대시보드 실행",
    Long:  `인터랙티브 TUI 대시보드를 실행하여 클러스터 상태를 모니터링합니다.`,
    RunE:  runDashboard,
}

var startTab string

func init() {
    dashboardCmd.Flags().StringVarP(&startTab, "tab", "t", "cluster", "시작 탭 (cluster|context|locks|tokens|peers)")
}

func runDashboard(cmd *cobra.Command, args []string) error {
    // 노드 연결 확인
    node, err := getConnectedNode()
    if err != nil {
        return fmt.Errorf("클러스터에 연결되어 있지 않습니다. 먼저 'agent-collab join'을 실행하세요")
    }

    // TUI 앱 생성
    app := tui.NewApp(node, tui.WithStartTab(startTab))

    // Bubbletea 프로그램 실행
    p := tea.NewProgram(
        app,
        tea.WithAltScreen(),       // 대체 화면 사용
        tea.WithMouseCellMotion(), // 마우스 지원
    )

    if _, err := p.Run(); err != nil {
        return fmt.Errorf("대시보드 실행 실패: %w", err)
    }

    return nil
}
```

### 6.3 TUI 메인 모델 (Bubbletea)

```go
// internal/interface/tui/model.go
package tui

import (
    "time"

    "github.com/charmbracelet/bubbles/help"
    "github.com/charmbracelet/bubbles/key"
    tea "github.com/charmbracelet/bubbletea"

    "agent-collab/internal/interface/tui/components"
    "agent-collab/internal/interface/tui/views"
)

type Tab int

const (
    TabCluster Tab = iota
    TabContext
    TabLocks
    TabTokens
    TabPeers
)

type Model struct {
    // 크기
    width  int
    height int

    // 현재 상태
    activeTab Tab
    ready     bool

    // 컴포넌트
    header components.Header
    footer components.Footer
    tabs   components.Tabs
    help   help.Model
    keys   KeyMap

    // 탭별 뷰
    clusterView views.ClusterView
    contextView views.ContextView
    locksView   views.LocksView
    tokensView  views.TokensView
    peersView   views.PeersView

    // 데이터 소스
    node    *Node
    metrics *MetricsCollector

    // 설정
    refreshInterval time.Duration
}

func NewApp(node *Node, opts ...Option) *Model {
    m := &Model{
        activeTab:       TabCluster,
        node:            node,
        metrics:         NewMetricsCollector(node),
        refreshInterval: time.Second,
        keys:            DefaultKeyMap(),
        help:            help.New(),
    }

    // 옵션 적용
    for _, opt := range opts {
        opt(m)
    }

    // 컴포넌트 초기화
    m.header = components.NewHeader(node.ProjectName(), node.ID())
    m.footer = components.NewFooter()
    m.tabs = components.NewTabs([]string{"Cluster", "Context", "Locks", "Tokens", "Peers"})

    // 뷰 초기화
    m.clusterView = views.NewClusterView(node)
    m.contextView = views.NewContextView(node)
    m.locksView = views.NewLocksView(node)
    m.tokensView = views.NewTokensView(node)
    m.peersView = views.NewPeersView(node)

    return m
}

func (m Model) Init() tea.Cmd {
    return tea.Batch(
        m.fetchInitialData(),
        m.tick(),
    )
}

func (m *Model) tick() tea.Cmd {
    return tea.Tick(m.refreshInterval, func(t time.Time) tea.Msg {
        return tickMsg(t)
    })
}
```

### 6.4 TUI Update 로직

```go
// internal/interface/tui/update.go
package tui

import (
    "github.com/charmbracelet/bubbles/key"
    tea "github.com/charmbracelet/bubbletea"
)

// 커스텀 메시지 타입
type tickMsg time.Time
type metricsMsg Metrics
type peersMsg []PeerInfo
type locksMsg []LockInfo
type contextMsg ContextStatus
type tokensMsg TokenMetrics

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd

    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        m.ready = true

        // 각 컴포넌트에 크기 전파
        m.header.SetWidth(msg.Width)
        m.footer.SetWidth(msg.Width)
        m.updateViewSizes()

    case tea.KeyMsg:
        switch {
        case key.Matches(msg, m.keys.Quit):
            return m, tea.Quit

        case key.Matches(msg, m.keys.Tab1):
            m.activeTab = TabCluster
        case key.Matches(msg, m.keys.Tab2):
            m.activeTab = TabContext
        case key.Matches(msg, m.keys.Tab3):
            m.activeTab = TabLocks
        case key.Matches(msg, m.keys.Tab4):
            m.activeTab = TabTokens
        case key.Matches(msg, m.keys.Tab5):
            m.activeTab = TabPeers

        case key.Matches(msg, m.keys.NextTab):
            m.activeTab = (m.activeTab + 1) % 5
        case key.Matches(msg, m.keys.PrevTab):
            m.activeTab = (m.activeTab + 4) % 5

        case key.Matches(msg, m.keys.Refresh):
            cmds = append(cmds, m.fetchAllData())

        case key.Matches(msg, m.keys.Help):
            m.help.ShowAll = !m.help.ShowAll
        }

        // 활성 뷰에 키 이벤트 전달
        cmds = append(cmds, m.updateActiveView(msg))

    case tickMsg:
        cmds = append(cmds, m.fetchMetrics(), m.tick())

    case metricsMsg:
        m.footer.UpdateMetrics(Metrics(msg))
        m.header.UpdateStatus(Metrics(msg))

    case peersMsg:
        m.peersView.Update([]PeerInfo(msg))
        m.header.UpdatePeerCount(len(msg))

    case locksMsg:
        m.locksView.Update([]LockInfo(msg))

    case contextMsg:
        m.contextView.Update(ContextStatus(msg))

    case tokensMsg:
        m.tokensView.Update(TokenMetrics(msg))
        m.footer.UpdateTokenRate(msg.TokensPerHour)
    }

    return m, tea.Batch(cmds...)
}

func (m *Model) updateActiveView(msg tea.Msg) tea.Cmd {
    switch m.activeTab {
    case TabCluster:
        return m.clusterView.Update(msg)
    case TabContext:
        return m.contextView.Update(msg)
    case TabLocks:
        return m.locksView.Update(msg)
    case TabTokens:
        return m.tokensView.Update(msg)
    case TabPeers:
        return m.peersView.Update(msg)
    }
    return nil
}
```

### 6.5 TUI View 렌더링

```go
// internal/interface/tui/view.go
package tui

import (
    "github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
    if !m.ready {
        return "Loading..."
    }

    // 레이아웃 구성
    header := m.header.View()
    tabs := m.tabs.View(int(m.activeTab))
    content := m.renderActiveView()
    footer := m.footer.View()

    // 수직 결합
    return lipgloss.JoinVertical(
        lipgloss.Left,
        header,
        tabs,
        content,
        footer,
    )
}

func (m Model) renderActiveView() string {
    // 컨텐츠 영역 높이 계산
    contentHeight := m.height - m.header.Height() - m.tabs.Height() - m.footer.Height()

    style := lipgloss.NewStyle().
        Width(m.width - 2).
        Height(contentHeight).
        Padding(1).
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("62"))

    var content string
    switch m.activeTab {
    case TabCluster:
        content = m.clusterView.View()
    case TabContext:
        content = m.contextView.View()
    case TabLocks:
        content = m.locksView.View()
    case TabTokens:
        content = m.tokensView.View()
    case TabPeers:
        content = m.peersView.View()
    }

    return style.Render(content)
}
```

### 6.6 스타일 정의 (Lipgloss)

```go
// internal/interface/tui/styles.go
package tui

import "github.com/charmbracelet/lipgloss"

var (
    // 색상 팔레트
    ColorPrimary   = lipgloss.Color("205")  // 핑크
    ColorSecondary = lipgloss.Color("62")   // 청록
    ColorSuccess   = lipgloss.Color("82")   // 초록
    ColorWarning   = lipgloss.Color("214")  // 주황
    ColorError     = lipgloss.Color("196")  // 빨강
    ColorMuted     = lipgloss.Color("240")  // 회색

    // 헤더 스타일
    HeaderStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(ColorPrimary).
        Padding(0, 1)

    // 탭 스타일
    TabStyle = lipgloss.NewStyle().
        Padding(0, 2)

    ActiveTabStyle = TabStyle.Copy().
        Bold(true).
        Foreground(ColorPrimary).
        Border(lipgloss.NormalBorder(), false, false, true, false).
        BorderForeground(ColorPrimary)

    InactiveTabStyle = TabStyle.Copy().
        Foreground(ColorMuted)

    // 테이블 스타일
    TableHeaderStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(ColorSecondary).
        Padding(0, 1)

    TableRowStyle = lipgloss.NewStyle().
        Padding(0, 1)

    TableSelectedStyle = TableRowStyle.Copy().
        Background(lipgloss.Color("236"))

    // 상태 인디케이터
    StatusOnline = lipgloss.NewStyle().
        Foreground(ColorSuccess).
        Render("●")

    StatusOffline = lipgloss.NewStyle().
        Foreground(ColorError).
        Render("○")

    StatusSyncing = lipgloss.NewStyle().
        Foreground(ColorWarning).
        Render("◐")

    // 게이지 스타일
    GaugeFilled = lipgloss.NewStyle().
        Foreground(ColorSuccess)

    GaugeEmpty = lipgloss.NewStyle().
        Foreground(ColorMuted)

    // 박스 스타일
    BoxStyle = lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(ColorSecondary).
        Padding(1)

    // 푸터 스타일
    FooterStyle = lipgloss.NewStyle().
        Foreground(ColorMuted).
        Padding(0, 1)

    FooterKeyStyle = lipgloss.NewStyle().
        Foreground(ColorPrimary).
        Bold(true)
)
```

### 6.7 토큰 뷰

```go
// internal/interface/tui/views/tokens.go
package views

import (
    "fmt"
    "strings"

    "github.com/charmbracelet/lipgloss"

    "agent-collab/internal/interface/tui/components"
)

type TokensView struct {
    width   int
    height  int
    metrics TokenMetrics
}

func NewTokensView(node *Node) TokensView {
    return TokensView{}
}

func (v *TokensView) Update(metrics TokenMetrics) {
    v.metrics = metrics
}

func (v TokensView) View() string {
    var sections []string

    // 오늘 사용량 게이지
    todaySection := v.renderTodayUsage()
    sections = append(sections, todaySection)

    // 사용량 breakdown
    breakdownSection := v.renderBreakdown()
    sections = append(sections, breakdownSection)

    // 시간별 트렌드
    trendSection := v.renderHourlyTrend()
    sections = append(sections, trendSection)

    // 기간별 요약
    summarySection := v.renderPeriodSummary()
    sections = append(sections, summarySection)

    return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (v TokensView) renderTodayUsage() string {
    title := lipgloss.NewStyle().Bold(true).Render("Today's Usage")

    percent := float64(v.metrics.TokensToday) / float64(v.metrics.DailyLimit) * 100
    gauge := components.RenderGauge(percent, 30)

    text := fmt.Sprintf("%.0f%% (%s / %s)",
        percent,
        formatNumber(v.metrics.TokensToday),
        formatNumber(v.metrics.DailyLimit),
    )

    return lipgloss.JoinVertical(lipgloss.Left,
        title,
        gauge+"  "+text,
        "",
    )
}

func (v TokensView) renderBreakdown() string {
    title := lipgloss.NewStyle().Bold(true).Render("Usage Breakdown")

    box := lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        Padding(1).
        Width(v.width - 4)

    var rows []string
    for _, item := range v.metrics.Breakdown {
        gauge := components.RenderGauge(item.Percent, 20)
        row := fmt.Sprintf("%-25s %s  %s tokens (%.0f%%)  $%.3f",
            item.Category,
            gauge,
            formatNumber(item.Tokens),
            item.Percent,
            item.Cost,
        )
        rows = append(rows, row)
    }

    return lipgloss.JoinVertical(lipgloss.Left,
        title,
        box.Render(strings.Join(rows, "\n\n")),
        "",
    )
}

func (v TokensView) renderHourlyTrend() string {
    title := lipgloss.NewStyle().Bold(true).Render("Hourly Trend")

    // 스파크라인 렌더링
    sparkline := components.RenderSparkline(v.metrics.HourlyData, v.width-10)

    return lipgloss.JoinVertical(lipgloss.Left,
        title,
        sparkline,
        "",
    )
}

func (v TokensView) renderPeriodSummary() string {
    title := lipgloss.NewStyle().Bold(true).Render("Period Summary")

    box := lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        Padding(0, 1)

    content := fmt.Sprintf(
        "Today      : %s tokens     Est. $%.2f\n"+
        "This Week  : %s tokens     Est. $%.2f\n"+
        "This Month : %s tokens     Est. $%.2f",
        formatNumber(v.metrics.TokensToday), v.metrics.CostToday,
        formatNumber(v.metrics.TokensWeek), v.metrics.CostWeek,
        formatNumber(v.metrics.TokensMonth), v.metrics.CostMonth,
    )

    return lipgloss.JoinVertical(lipgloss.Left,
        title,
        box.Render(content),
    )
}

func formatNumber(n int64) string {
    if n >= 1_000_000 {
        return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
    }
    if n >= 1_000 {
        return fmt.Sprintf("%.1fK", float64(n)/1_000)
    }
    return fmt.Sprintf("%d", n)
}
```

### 6.8 컴포넌트 - 게이지

```go
// internal/interface/tui/components/gauge.go
package components

import (
    "strings"

    "github.com/charmbracelet/lipgloss"
)

var (
    gaugeFilledChar = "█"
    gaugeEmptyChar  = "░"

    gaugeFilledStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
    gaugeEmptyStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

func RenderGauge(percent float64, width int) string {
    if percent > 100 {
        percent = 100
    }
    if percent < 0 {
        percent = 0
    }

    filled := int(float64(width) * percent / 100)
    empty := width - filled

    filledPart := gaugeFilledStyle.Render(strings.Repeat(gaugeFilledChar, filled))
    emptyPart := gaugeEmptyStyle.Render(strings.Repeat(gaugeEmptyChar, empty))

    return filledPart + emptyPart
}

// 색상 변화 게이지 (사용량에 따라)
func RenderColorGauge(percent float64, width int) string {
    var color lipgloss.Color
    switch {
    case percent >= 90:
        color = lipgloss.Color("196") // 빨강
    case percent >= 70:
        color = lipgloss.Color("214") // 주황
    case percent >= 50:
        color = lipgloss.Color("226") // 노랑
    default:
        color = lipgloss.Color("82") // 초록
    }

    filled := int(float64(width) * percent / 100)
    empty := width - filled

    filledPart := lipgloss.NewStyle().Foreground(color).Render(strings.Repeat(gaugeFilledChar, filled))
    emptyPart := gaugeEmptyStyle.Render(strings.Repeat(gaugeEmptyChar, empty))

    return filledPart + emptyPart
}
```

### 6.9 컴포넌트 - 스파크라인

```go
// internal/interface/tui/components/sparkline.go
package components

import (
    "strings"

    "github.com/charmbracelet/lipgloss"
)

var sparkChars = []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

func RenderSparkline(data []float64, width int) string {
    if len(data) == 0 {
        return ""
    }

    // 데이터를 width에 맞게 샘플링
    sampled := sampleData(data, width)

    // 최대값 찾기
    max := 0.0
    for _, v := range sampled {
        if v > max {
            max = v
        }
    }

    if max == 0 {
        return strings.Repeat(sparkChars[0], width)
    }

    // 스파크라인 생성
    var result strings.Builder
    style := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))

    for _, v := range sampled {
        idx := int((v / max) * float64(len(sparkChars)-1))
        result.WriteString(style.Render(sparkChars[idx]))
    }

    return result.String()
}

func sampleData(data []float64, width int) []float64 {
    if len(data) <= width {
        return data
    }

    result := make([]float64, width)
    step := float64(len(data)) / float64(width)

    for i := 0; i < width; i++ {
        idx := int(float64(i) * step)
        result[i] = data[idx]
    }

    return result
}
```

---

## 7. 구현 로드맵

### Phase 1: Foundation (2주)

| # | 태스크 | 파일 |
|---|--------|------|
| 1.1 | Go 모듈 초기화 | `go.mod` |
| 1.2 | Cobra CLI 뼈대 | `internal/interface/cli/*.go` |
| 1.3 | libp2p 노드 초기화 | `internal/infrastructure/network/libp2p/` |
| 1.4 | 다중 전송 계층 + NAT | `transports.go`, `nat.go` |
| 1.5 | 초대 토큰 시스템 | `internal/infrastructure/crypto/` |
| 1.6 | init/join/status 명령어 | `internal/interface/cli/` |

### Phase 2: TUI Dashboard (2주)

| # | 태스크 | 파일 |
|---|--------|------|
| 2.1 | Bubbletea 앱 뼈대 | `internal/interface/tui/app.go` |
| 2.2 | 메인 모델 + Update/View | `model.go`, `update.go`, `view.go` |
| 2.3 | 헤더/푸터/탭 컴포넌트 | `components/*.go` |
| 2.4 | 게이지/스파크라인 컴포넌트 | `components/gauge.go`, `sparkline.go` |
| 2.5 | Cluster 탭 뷰 | `views/cluster.go` |
| 2.6 | Context 탭 뷰 | `views/context.go` |
| 2.7 | Locks 탭 뷰 | `views/locks.go` |
| 2.8 | Tokens 탭 뷰 | `views/tokens.go` |
| 2.9 | Peers 탭 뷰 | `views/peers.go` |
| 2.10 | 실시간 업데이트 | `messages.go` |

### Phase 3: Core Features (3주)

| # | 태스크 | 파일 |
|---|--------|------|
| 3.1 | SemanticTarget + tree-sitter | `internal/domain/lock/` |
| 3.2 | SemanticLock + 3-phase commit | `negotiator.go` |
| 3.3 | Vector DB (Milvus Lite) | `internal/infrastructure/storage/vector/` |
| 3.4 | Context Delta 동기화 | `internal/domain/context/` |
| 3.5 | 토큰 사용량 추적 | `internal/domain/token/` |

### Phase 4: Production (2주)

| # | 태스크 | 파일 |
|---|--------|------|
| 4.1 | Human-in-the-loop | `internal/interface/notification/` |
| 4.2 | 네트워크 파티션 복구 | `internal/domain/lock/recovery.go` |
| 4.3 | goreleaser 설정 | `.goreleaser.yaml` |
| 4.4 | E2E 테스트 | `tests/e2e/` |

---

## 8. 트레이드오프 요약

| 결정 | 선택 | 얻는 것 | 포기하는 것 |
|------|------|---------|-------------|
| 언어 | Go | 성능, 단일 바이너리 | 빠른 프로토타이핑 |
| CLI | Cobra | 표준, 안정적 | 경량화 |
| TUI | Bubbletea | 현대적, Elm 아키텍처 | 단순함 |
| 네트워크 | libp2p | 자동 NAT, 동적 peer | Wireguard 성능 |

---

*작성일: 2026-02-08*
*버전: 4.0*
*변경: Cobra CLI + Bubbletea TUI 추가*
