package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"agent-collab/internal/interface/tui/mode"
)

// Option은 App 설정 옵션입니다.
type Option func(*Model)

// WithStartTab은 시작 탭을 설정합니다.
func WithStartTab(tab string) Option {
	return func(m *Model) {
		switch tab {
		case "cluster":
			m.activeTab = TabCluster
		case "context":
			m.activeTab = TabContext
		case "locks":
			m.activeTab = TabLocks
		case "tokens":
			m.activeTab = TabTokens
		case "peers":
			m.activeTab = TabPeers
		}
	}
}

// WithRefreshInterval은 갱신 간격을 설정합니다.
func WithRefreshInterval(d time.Duration) Option {
	return func(m *Model) {
		m.refreshInterval = d
	}
}

// NewApp은 새 TUI 앱을 생성합니다.
func NewApp(opts ...Option) *Model {
	// 명령 입력 초기화
	ti := textinput.New()
	ti.Placeholder = "명령어 입력..."
	ti.CharLimit = 256
	ti.Width = 50

	m := &Model{
		activeTab:       TabCluster,
		refreshInterval: time.Second,
		keys:            DefaultKeyMap(),
		mode:            mode.Normal,
		commandInput:    ti,
		commandHints:    defaultCommandHints(),
	}

	// 옵션 적용
	for _, opt := range opts {
		opt(m)
	}

	return m
}

// defaultCommandHints는 기본 명령 힌트를 반환합니다.
func defaultCommandHints() []CommandHint {
	return []CommandHint{
		{Command: "init", Description: "새 클러스터 초기화", Args: "-p <project>"},
		{Command: "join", Description: "클러스터 참여", Args: "<token>"},
		{Command: "leave", Description: "클러스터 탈퇴", Args: ""},
		{Command: "status", Description: "상태 확인", Args: ""},
		{Command: "lock list", Description: "락 목록", Args: ""},
		{Command: "lock release", Description: "락 해제", Args: "<lock-id>"},
		{Command: "agents", Description: "에이전트 목록", Args: ""},
		{Command: "peers", Description: "피어 목록", Args: ""},
		{Command: "tokens", Description: "토큰 사용량", Args: ""},
		{Command: "config", Description: "설정 보기", Args: ""},
		{Command: "help", Description: "도움말", Args: ""},
		{Command: "quit", Description: "종료", Args: ""},
	}
}

// Init은 초기 명령을 반환합니다.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.tick(),
		m.fetchInitialData(),
		textinput.Blink, // 커서 깜빡임
	)
}

// tick은 주기적 갱신 명령을 반환합니다.
func (m Model) tick() tea.Cmd {
	return tea.Tick(m.refreshInterval, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// fetchInitialData는 초기 데이터를 가져옵니다.
func (m Model) fetchInitialData() tea.Cmd {
	return func() tea.Msg {
		// TODO: 실제 데이터 가져오기
		return InitialDataMsg{
			ProjectName: "my-project",
			NodeID:      "QmXx...Yy",
			PeerCount:   4,
			SyncHealth:  98.5,
		}
	}
}
