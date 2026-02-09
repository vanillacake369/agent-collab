package tui

import "time"

// TickMsg는 주기적 갱신 메시지입니다.
type TickMsg time.Time

// InitialDataMsg는 초기 데이터 메시지입니다.
type InitialDataMsg struct {
	ProjectName string
	NodeID      string
	PeerCount   int
	SyncHealth  float64
}

// MetricsMsg는 메트릭 업데이트 메시지입니다.
type MetricsMsg struct {
	CPUUsage    float64
	MemUsage    int64
	NetUpload   int64
	NetDownload int64
	TokensRate  int64
}

// PeersMsg는 peer 목록 업데이트 메시지입니다.
type PeersMsg struct {
	Peers []PeerInfo
}

// PeerInfo는 peer 정보입니다.
type PeerInfo struct {
	ID        string
	Name      string
	Status    string
	Latency   int
	Transport string
	SyncPct   float64
}

// LocksMsg는 락 목록 업데이트 메시지입니다.
type LocksMsg struct {
	Locks []LockInfo
}

// LockInfo는 락 정보입니다.
type LockInfo struct {
	ID        string
	Holder    string
	Target    string
	Intention string
	TTL       int
}

// ContextMsg는 컨텍스트 상태 업데이트 메시지입니다.
type ContextMsg struct {
	TotalEmbeddings int
	DatabaseSize    int64
	SyncProgress    map[string]float64
	RecentDeltas    []DeltaInfo
}

// DeltaInfo는 Delta 정보입니다.
type DeltaInfo struct {
	Time   time.Time
	From   string
	Files  int
	Size   int64
	Status string
}

// TokensMsg는 토큰 사용량 업데이트 메시지입니다.
type TokensMsg struct {
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

// TokenBreakdown은 토큰 사용량 상세입니다.
type TokenBreakdown struct {
	Category string
	Tokens   int64
	Percent  float64
	Cost     float64
}

// CommandResultMsg는 명령 실행 결과 메시지입니다.
type CommandResultMsg struct {
	Result string
	Err    error
}

// ActionMsg는 액션 요청 메시지입니다.
type ActionMsg struct {
	Action string
	Args   []string
}
