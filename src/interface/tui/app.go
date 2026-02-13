package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"agent-collab/src/interface/daemon"
	"agent-collab/src/interface/tui/mode"
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

// WithClient sets a custom daemon client (for testing).
func WithClient(client *daemon.Client) Option {
	return func(m *Model) {
		m.daemonClient = client
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

// NewModelWithClient creates a new TUI model with a custom daemon client (for testing).
func NewModelWithClient(client *daemon.Client) *Model {
	return NewApp(WithClient(client))
}

// getClient returns the daemon client (uses injected client if available).
func (m *Model) getClient() *daemon.Client {
	if m.daemonClient != nil {
		return m.daemonClient
	}
	return daemon.NewClient()
}

// defaultCommandHints는 기본 명령 힌트를 반환합니다.
func defaultCommandHints() []CommandHint {
	return []CommandHint{
		{
			Command:     "init",
			Description: "새 클러스터 초기화",
			Args:        "",
			SubHints: []CommandHint{
				{Command: "-p", Description: "프로젝트 이름", Args: "<name>"},
				{Command: "--project", Description: "프로젝트 이름 (긴 형식)", Args: "<name>"},
				{Command: "--force", Description: "강제 초기화", Args: ""},
			},
		},
		{
			Command:     "join",
			Description: "클러스터 참여",
			Args:        "<token>",
		},
		{
			Command:     "leave",
			Description: "클러스터 탈퇴",
			Args:        "",
			SubHints: []CommandHint{
				{Command: "--force", Description: "강제 탈퇴", Args: ""},
			},
		},
		{
			Command:     "status",
			Description: "상태 확인",
			Args:        "",
			SubHints: []CommandHint{
				{Command: "--json", Description: "JSON 형식 출력", Args: ""},
				{Command: "--verbose", Description: "상세 출력", Args: ""},
			},
		},
		{
			Command:     "lock",
			Description: "락 관리",
			Args:        "",
			SubHints: []CommandHint{
				{Command: "list", Description: "락 목록 조회", Args: ""},
				{Command: "release", Description: "락 해제", Args: "<lock-id>"},
				{Command: "acquire", Description: "락 획득", Args: "<resource>"},
			},
		},
		{
			Command:     "agents",
			Description: "에이전트 목록",
			Args:        "",
			SubHints: []CommandHint{
				{Command: "--all", Description: "모든 에이전트 표시", Args: ""},
				{Command: "--active", Description: "활성 에이전트만", Args: ""},
			},
		},
		{
			Command:     "peers",
			Description: "피어 목록",
			Args:        "",
			SubHints: []CommandHint{
				{Command: "--json", Description: "JSON 형식 출력", Args: ""},
			},
		},
		{
			Command:     "tokens",
			Description: "토큰 사용량",
			Args:        "",
			SubHints: []CommandHint{
				{Command: "--today", Description: "오늘 사용량", Args: ""},
				{Command: "--week", Description: "주간 사용량", Args: ""},
				{Command: "--month", Description: "월간 사용량", Args: ""},
			},
		},
		{
			Command:     "config",
			Description: "설정 관리",
			Args:        "",
			SubHints: []CommandHint{
				{Command: "show", Description: "설정 보기", Args: ""},
				{Command: "set", Description: "설정 변경", Args: "<key> <value>"},
				{Command: "reset", Description: "설정 초기화", Args: ""},
			},
		},
		{
			Command:     "help",
			Description: "도움말",
			Args:        "",
		},
		{
			Command:     "quit",
			Description: "종료",
			Args:        "",
		},
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
	return tea.Batch(
		func() tea.Msg {
			client := daemon.NewClient()
			if !client.IsRunning() {
				return InitialDataMsg{
					ProjectName: "(데몬 미실행)",
					NodeID:      "-",
					PeerCount:   0,
					SyncHealth:  0,
				}
			}

			status, err := client.Status()
			if err != nil {
				return InitialDataMsg{
					ProjectName: "(연결 실패)",
					NodeID:      "-",
					PeerCount:   0,
					SyncHealth:  0,
				}
			}

			return InitialDataMsg{
				ProjectName: status.ProjectName,
				NodeID:      status.NodeID,
				PeerCount:   status.PeerCount,
				SyncHealth:  100, // TODO: 실제 sync health 계산
			}
		},
		m.fetchPeers(),
		m.fetchLocks(),
	)
}
