package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/sahilm/fuzzy"

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

// ConfirmAction은 확인 대화상자 액션 타입입니다.
type ConfirmAction int

const (
	ConfirmNone ConfirmAction = iota
	ConfirmLeave
	ConfirmReleaseLock
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
	commandInput       textinput.Model
	commandHints       []CommandHint
	filteredHints      []FilteredHint // 퍼지 매칭 결과
	commandSelectedIdx int            // 선택된 힌트 인덱스

	// 입력 프롬프트
	inputPrompt   string
	inputCallback func(string) error
	inputError    string

	// 확인 대화상자
	confirmPrompt     string
	confirmYesText    string
	confirmNoText     string
	confirmActionType ConfirmAction
	confirmTargetID   string // 락 ID 등 대상

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
	SubHints    []CommandHint // 하위 옵션/서브커맨드
}

// FilteredHint는 퍼지 매칭된 힌트입니다.
type FilteredHint struct {
	Hint           CommandHint
	MatchedIndexes []int  // 매칭된 문자 인덱스 (하이라이팅용)
	Score          int    // 매칭 점수
	Prefix         string // 자동완성 시 앞에 붙일 prefix (예: "init ")
}

// CommandHintSource는 fuzzy.Source 인터페이스 구현입니다.
type CommandHintSource []CommandHint

// String은 인덱스에 해당하는 문자열을 반환합니다.
func (c CommandHintSource) String(i int) string {
	return c[i].Command
}

// Len은 소스의 길이를 반환합니다.
func (c CommandHintSource) Len() int {
	return len(c)
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

// IsHelpMode는 Help 모드인지 확인합니다.
func (m Model) IsHelpMode() bool {
	return m.mode == mode.Help
}

// EnterHelpMode는 Help 모드로 진입합니다.
func (m *Model) EnterHelpMode() {
	m.prevMode = m.mode
	m.mode = mode.Help
}

// EnterCommandMode는 Command 모드로 진입합니다.
func (m *Model) EnterCommandMode() {
	m.prevMode = m.mode
	m.mode = mode.Command
	m.commandInput.SetValue("")
	m.commandInput.Focus()
	m.commandSelectedIdx = 0
	m.UpdateFilteredHints() // 초기 힌트 목록 생성
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
func (m *Model) EnterConfirmMode(prompt string, actionType ConfirmAction, targetID string) {
	m.prevMode = m.mode
	m.mode = mode.Confirm
	m.confirmPrompt = prompt
	m.confirmYesText = "Yes"
	m.confirmNoText = "No"
	m.confirmActionType = actionType
	m.confirmTargetID = targetID
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

// UpdateFilteredHints는 현재 입력값으로 퍼지 매칭을 수행합니다.
func (m *Model) UpdateFilteredHints() {
	input := m.commandInput.Value()
	m.filteredHints = nil
	m.commandSelectedIdx = 0

	// 입력을 파싱하여 컨텍스트 파악
	currentHints, searchTerm, prefix := m.getHintContext(input)

	if searchTerm == "" {
		// 검색어가 없으면 모든 힌트 표시
		for _, hint := range currentHints {
			m.filteredHints = append(m.filteredHints, FilteredHint{
				Hint:           hint,
				MatchedIndexes: nil,
				Score:          0,
				Prefix:         prefix,
			})
		}
		return
	}

	// 퍼지 매칭 수행
	matches := fuzzy.FindFrom(searchTerm, CommandHintSource(currentHints))

	for _, match := range matches {
		m.filteredHints = append(m.filteredHints, FilteredHint{
			Hint:           currentHints[match.Index],
			MatchedIndexes: match.MatchedIndexes,
			Score:          match.Score,
			Prefix:         prefix,
		})
	}
}

// getHintContext는 현재 입력에 따라 적절한 힌트 목록과 검색어를 반환합니다.
// 반환값: (힌트 목록, 검색어, prefix)
func (m *Model) getHintContext(input string) ([]CommandHint, string, string) {
	if input == "" {
		return m.commandHints, "", ""
	}

	parts := strings.Fields(input)
	endsWithSpace := strings.HasSuffix(input, " ")

	// Case 1: 단일 단어, 공백 없음 → 최상위 명령어에서 퍼지 매칭
	// 예: "ini" → 최상위에서 "ini" 검색
	if len(parts) == 1 && !endsWithSpace {
		return m.commandHints, parts[0], ""
	}

	// Case 2: 명령어 입력 완료 (공백 있음) → 해당 명령어의 SubHints 검색
	// 예: "init " → init의 SubHints 표시
	// 예: "init -" → init의 SubHints에서 "-" 검색
	// 예: "lock li" → lock의 SubHints에서 "li" 검색
	baseCmd := parts[0]

	// 기본 명령어 찾기
	for _, hint := range m.commandHints {
		if hint.Command == baseCmd && len(hint.SubHints) > 0 {
			prefix := baseCmd + " "

			// 서브커맨드가 있는 경우 (예: "lock list")
			if len(parts) >= 2 {
				subCmd := parts[1]

				// 서브커맨드도 완료된 경우 → 서브커맨드의 SubHints 검색
				for _, subHint := range hint.SubHints {
					if subHint.Command == subCmd && len(subHint.SubHints) > 0 {
						subPrefix := prefix + subCmd + " "
						if len(parts) == 2 && endsWithSpace {
							return subHint.SubHints, "", subPrefix
						}
						if len(parts) > 2 || (len(parts) == 2 && !endsWithSpace) {
							searchTerm := ""
							if len(parts) > 2 {
								searchTerm = parts[len(parts)-1]
							}
							if len(parts) == 2 && !endsWithSpace {
								// 아직 서브커맨드 입력 중
								return hint.SubHints, subCmd, prefix
							}
							return subHint.SubHints, searchTerm, subPrefix
						}
					}
				}

				// 서브커맨드 입력 중 (공백 없음) → 서브커맨드에서 퍼지 매칭
				if !endsWithSpace {
					return hint.SubHints, subCmd, prefix
				}

				// 서브커맨드 완료 → 해당 서브커맨드의 옵션 찾기
				for _, subHint := range hint.SubHints {
					if subHint.Command == subCmd {
						subPrefix := prefix + subCmd + " "
						if len(subHint.SubHints) > 0 {
							if len(parts) > 2 {
								return subHint.SubHints, parts[len(parts)-1], subPrefix
							}
							return subHint.SubHints, "", subPrefix
						}
						// SubHints가 없으면 빈 목록 반환
						return nil, "", subPrefix
					}
				}
			}

			// "init " 처럼 명령어만 완료된 경우
			if len(parts) == 1 && endsWithSpace {
				return hint.SubHints, "", prefix
			}

			return hint.SubHints, parts[len(parts)-1], prefix
		}
	}

	// SubHints가 없는 명령어인 경우 → 힌트 없음
	if len(parts) >= 1 && endsWithSpace {
		return nil, "", input
	}

	// 기본: 최상위에서 검색
	return m.commandHints, input, ""
}

// ApplySelectedHint는 선택된 힌트로 자동완성합니다.
func (m *Model) ApplySelectedHint() bool {
	if len(m.filteredHints) == 0 || m.commandSelectedIdx >= len(m.filteredHints) {
		return false
	}

	selected := m.filteredHints[m.commandSelectedIdx]

	// prefix + 명령어 조합
	cmd := selected.Prefix + selected.Hint.Command

	// 서브힌트가 있거나 인자가 필요한 경우 공백 추가
	if len(selected.Hint.SubHints) > 0 || selected.Hint.Args != "" {
		cmd += " "
	}

	m.commandInput.SetValue(cmd)
	m.commandInput.SetCursor(len(cmd))
	m.UpdateFilteredHints()
	return true
}

// SelectNextHint는 다음 힌트를 선택합니다.
func (m *Model) SelectNextHint() {
	if len(m.filteredHints) > 0 && m.commandSelectedIdx < len(m.filteredHints)-1 {
		m.commandSelectedIdx++
	}
}

// SelectPrevHint는 이전 힌트를 선택합니다.
func (m *Model) SelectPrevHint() {
	if m.commandSelectedIdx > 0 {
		m.commandSelectedIdx--
	}
}
