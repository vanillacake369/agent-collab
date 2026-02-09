package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"

	"agent-collab/internal/interface/tui/mode"
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

	// 모드 시스템
	mode     mode.Mode
	prevMode mode.Mode

	// 명령 팔레트
	commandInput textinput.Model
	commandHints []CommandHint

	// 입력 프롬프트
	inputPrompt   string
	inputCallback func(string) error
	inputError    string

	// 확인 대화상자
	confirmPrompt  string
	confirmYesText string
	confirmNoText  string
	confirmAction  func() error

	// 선택
	selectedIndex int

	// 실행 결과
	lastResult  string
	lastError   error
	showResult  bool
	resultTimer int

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

// CommandHint는 명령 자동완성 힌트입니다.
type CommandHint struct {
	Command     string
	Description string
	Args        string
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
	Locks         []LockInfo
	SelectedIndex int
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

// Mode 헬퍼 메서드

// IsNormalMode는 Normal 모드인지 확인합니다.
func (m Model) IsNormalMode() bool {
	return m.mode == mode.Normal
}

// IsCommandMode는 Command 모드인지 확인합니다.
func (m Model) IsCommandMode() bool {
	return m.mode == mode.Command
}

// IsInputMode는 Input 모드인지 확인합니다.
func (m Model) IsInputMode() bool {
	return m.mode == mode.Input
}

// IsConfirmMode는 Confirm 모드인지 확인합니다.
func (m Model) IsConfirmMode() bool {
	return m.mode == mode.Confirm
}

// EnterCommandMode는 Command 모드로 진입합니다.
func (m *Model) EnterCommandMode() {
	m.prevMode = m.mode
	m.mode = mode.Command
	m.commandInput.SetValue("")
	m.commandInput.Focus()
}

// EnterInputMode는 Input 모드로 진입합니다.
func (m *Model) EnterInputMode(prompt string, callback func(string) error) {
	m.prevMode = m.mode
	m.mode = mode.Input
	m.inputPrompt = prompt
	m.inputCallback = callback
	m.inputError = ""
	m.commandInput.SetValue("")
	m.commandInput.Focus()
}

// EnterConfirmMode는 Confirm 모드로 진입합니다.
func (m *Model) EnterConfirmMode(prompt string, action func() error) {
	m.prevMode = m.mode
	m.mode = mode.Confirm
	m.confirmPrompt = prompt
	m.confirmYesText = "Yes"
	m.confirmNoText = "No"
	m.confirmAction = action
}

// ExitToNormalMode는 Normal 모드로 복귀합니다.
func (m *Model) ExitToNormalMode() {
	m.mode = mode.Normal
	m.commandInput.Blur()
	m.inputError = ""
}

// SetResult는 실행 결과를 설정합니다.
func (m *Model) SetResult(result string, err error) {
	m.lastResult = result
	m.lastError = err
	m.showResult = true
	m.resultTimer = 5 // 5초 후 자동 숨김
}

// ClearResult는 실행 결과를 지웁니다.
func (m *Model) ClearResult() {
	m.lastResult = ""
	m.lastError = nil
	m.showResult = false
}
