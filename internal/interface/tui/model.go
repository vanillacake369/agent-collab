package tui

import (
	"time"
)

// Tab은 탭 종류입니다.
type Tab int

const (
	TabCluster Tab = iota
	TabContext
	TabLocks
	TabTokens
	TabPeers
)

// Model은 TUI 메인 모델입니다.
type Model struct {
	// 크기
	width  int
	height int

	// 상태
	activeTab Tab
	ready     bool

	// 데이터
	projectName string
	nodeID      string
	peerCount   int
	syncHealth  float64
	uptime      time.Duration
	startTime   time.Time

	// 뷰 데이터 (직접 저장)
	clusterData ClusterData
	contextData ContextData
	locksData   LocksData
	tokensData  TokensData
	peersData   PeersData

	// 뷰 크기
	clusterView ViewSize
	contextView ViewSize
	locksView   ViewSize
	tokensView  ViewSize
	peersView   ViewSize

	// 메트릭
	cpuUsage    float64
	memUsage    int64
	netUpload   int64
	netDownload int64
	tokensRate  int64

	// 키 바인딩
	keys KeyMap

	// 설정
	refreshInterval time.Duration
}

// ViewSize는 뷰 크기입니다.
type ViewSize struct {
	Width  int
	Height int
}

// ClusterData는 클러스터 데이터입니다.
type ClusterData struct {
	HealthScore    float64
	TotalPeers     int
	ActiveLocks    int
	PendingSyncs   int
	AvgLatency     int
	MessagesPerSec float64
}

// ContextData는 컨텍스트 데이터입니다.
type ContextData struct {
	TotalEmbeddings int
	DatabaseSize    int64
	SyncProgress    map[string]float64
}

// LocksData는 락 데이터입니다.
type LocksData struct {
	Locks []LockInfo
}

// TokensData는 토큰 데이터입니다.
type TokensData struct {
	TodayUsed   int64
	DailyLimit  int64
	Breakdown   []TokenBreakdown
	HourlyData  []float64
	CostToday   float64
	CostWeek    float64
	CostMonth   float64
	TokensWeek  int64
	TokensMonth int64
}

// PeersData는 피어 데이터입니다.
type PeersData struct {
	Peers         []PeerInfo
	SelectedIndex int
}

// TabNames는 탭 이름 목록입니다.
var TabNames = []string{"Cluster", "Context", "Locks", "Tokens", "Peers"}

// GetTabName은 탭 이름을 반환합니다.
func (t Tab) String() string {
	if int(t) < len(TabNames) {
		return TabNames[t]
	}
	return ""
}
