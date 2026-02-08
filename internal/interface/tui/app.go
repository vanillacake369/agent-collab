package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
	m := &Model{
		activeTab:       TabCluster,
		refreshInterval: time.Second,
		keys:            DefaultKeyMap(),
	}

	// 옵션 적용
	for _, opt := range opts {
		opt(m)
	}

	return m
}

// Init은 초기 명령을 반환합니다.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.tick(),
		m.fetchInitialData(),
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
