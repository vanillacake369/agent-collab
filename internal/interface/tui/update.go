package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"agent-collab/internal/interface/tui/mode"
)

// Update는 메시지를 처리하고 모델을 업데이트합니다.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.updateViewSizes()
		m.commandInput.Width = m.width - 10

	case tea.KeyMsg:
		// 모드별 키 처리
		switch m.mode {
		case mode.Normal:
			return m.updateNormalMode(msg)
		case mode.Command:
			return m.updateCommandMode(msg)
		case mode.Input:
			return m.updateInputMode(msg)
		case mode.Confirm:
			return m.updateConfirmMode(msg)
		}

	case TickMsg:
		m.uptime = time.Since(m.startTime)
		// 결과 타이머 감소
		if m.showResult && m.resultTimer > 0 {
			m.resultTimer--
			if m.resultTimer == 0 {
				m.ClearResult()
			}
		}
		cmds = append(cmds, m.tick(), m.fetchMetrics())

	case CommandResultMsg:
		m.SetResult(msg.Result, msg.Err)
		m.ExitToNormalMode()

	case InitialDataMsg:
		m.projectName = msg.ProjectName
		m.nodeID = msg.NodeID
		m.peerCount = msg.PeerCount
		m.syncHealth = msg.SyncHealth
		m.startTime = time.Now()

	case MetricsMsg:
		m.cpuUsage = msg.CPUUsage
		m.memUsage = msg.MemUsage
		m.netUpload = msg.NetUpload
		m.netDownload = msg.NetDownload
		m.tokensRate = msg.TokensRate

	case PeersMsg:
		m.peerCount = len(msg.Peers)
		m.peersData.Peers = msg.Peers

	case LocksMsg:
		m.locksData.Locks = msg.Locks

	case ContextMsg:
		m.contextData.TotalEmbeddings = msg.TotalEmbeddings
		m.contextData.DatabaseSize = msg.DatabaseSize
		m.contextData.SyncProgress = msg.SyncProgress

	case TokensMsg:
		m.tokensData.TodayUsed = msg.TodayUsed
		m.tokensData.DailyLimit = msg.DailyLimit
		m.tokensData.Breakdown = msg.Breakdown
		m.tokensData.HourlyData = msg.HourlyData
		m.tokensData.CostToday = msg.CostToday
		m.tokensData.CostWeek = msg.CostWeek
		m.tokensData.CostMonth = msg.CostMonth
		m.tokensData.TokensWeek = msg.TokensWeek
		m.tokensData.TokensMonth = msg.TokensMonth
	}

	return m, tea.Batch(cmds...)
}

// updateNormalMode는 Normal 모드에서 키를 처리합니다.
func (m Model) updateNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch {
	// 종료
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	// 명령 모드 진입
	case key.Matches(msg, m.keys.CommandMode):
		m.EnterCommandMode()
		return m, nil

	// 탭 전환
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
		m.activeTab = Tab((int(m.activeTab) + 1) % 5)
	case key.Matches(msg, m.keys.PrevTab):
		m.activeTab = Tab((int(m.activeTab) + 4) % 5)

	// 새로고침
	case key.Matches(msg, m.keys.Refresh):
		cmds = append(cmds, m.fetchAllData())

	// 액션 단축키
	case key.Matches(msg, m.keys.ActionInit):
		m.EnterInputMode("프로젝트 이름", func(name string) error {
			return m.executeInit(name)
		})
		return m, nil

	case key.Matches(msg, m.keys.ActionJoin):
		m.EnterInputMode("초대 토큰", func(token string) error {
			return m.executeJoin(token)
		})
		return m, nil

	case key.Matches(msg, m.keys.ActionLeave):
		m.EnterConfirmMode("클러스터에서 탈퇴하시겠습니까?", ConfirmLeave, "")
		return m, nil

	case key.Matches(msg, m.keys.ActionStatus):
		m.activeTab = TabCluster
		cmds = append(cmds, m.fetchAllData())

	case key.Matches(msg, m.keys.ActionAgents):
		// TODO: agents 탭 추가 또는 모달
		m.SetResult("Agents 기능은 아직 구현 중입니다", nil)

	case key.Matches(msg, m.keys.ActionLocks):
		m.activeTab = TabLocks

	case key.Matches(msg, m.keys.ActionPeers):
		m.activeTab = TabPeers

	case key.Matches(msg, m.keys.ActionTokens):
		m.activeTab = TabTokens

	// 네비게이션 (탭 내 선택)
	case key.Matches(msg, m.keys.Up):
		m.navigateUp()
	case key.Matches(msg, m.keys.Down):
		m.navigateDown()

	// 선택된 항목 액션
	case key.Matches(msg, m.keys.Enter):
		cmds = append(cmds, m.executeSelectedAction())

	case key.Matches(msg, m.keys.Delete):
		if m.activeTab == TabLocks && len(m.locksData.Locks) > 0 {
			lockID := m.locksData.Locks[m.locksData.SelectedIndex].ID
			m.EnterConfirmMode("락 '"+lockID+"'을 해제하시겠습니까?", ConfirmReleaseLock, lockID)
		}
	}

	return m, tea.Batch(cmds...)
}

// updateCommandMode는 Command 모드에서 키를 처리합니다.
func (m Model) updateCommandMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Escape):
		m.ExitToNormalMode()
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		cmd := m.executeCommand(m.commandInput.Value())
		m.ExitToNormalMode()
		return m, cmd

	default:
		var cmd tea.Cmd
		m.commandInput, cmd = m.commandInput.Update(msg)
		return m, cmd
	}
}

// updateInputMode는 Input 모드에서 키를 처리합니다.
func (m Model) updateInputMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Escape):
		m.ExitToNormalMode()
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		value := m.commandInput.Value()
		if value == "" {
			m.inputError = "값을 입력해주세요"
			return m, nil
		}
		if m.inputCallback != nil {
			if err := m.inputCallback(value); err != nil {
				m.inputError = err.Error()
				return m, nil
			}
		}
		m.ExitToNormalMode()
		return m, nil

	default:
		var cmd tea.Cmd
		m.commandInput, cmd = m.commandInput.Update(msg)
		return m, cmd
	}
}

// updateConfirmMode는 Confirm 모드에서 키를 처리합니다.
func (m Model) updateConfirmMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Yes):
		// 액션 타입에 따라 처리
		actionType := m.confirmActionType
		targetID := m.confirmTargetID

		// 상태 초기화
		m.confirmActionType = ConfirmNone
		m.confirmTargetID = ""
		m.ExitToNormalMode()

		switch actionType {
		case ConfirmLeave:
			// 클러스터 탈퇴 시 TUI 종료
			return m, tea.Quit
		case ConfirmReleaseLock:
			if err := m.executeReleaseLock(targetID); err != nil {
				m.SetResult("", err)
			}
		}
		return m, nil

	case key.Matches(msg, m.keys.No), key.Matches(msg, m.keys.Escape):
		m.confirmActionType = ConfirmNone
		m.confirmTargetID = ""
		m.ExitToNormalMode()
		return m, nil
	}

	return m, nil
}

// 네비게이션 헬퍼

func (m *Model) navigateUp() {
	switch m.activeTab {
	case TabLocks:
		if m.locksData.SelectedIndex > 0 {
			m.locksData.SelectedIndex--
		}
	case TabPeers:
		if m.peersData.SelectedIndex > 0 {
			m.peersData.SelectedIndex--
		}
	}
}

func (m *Model) navigateDown() {
	switch m.activeTab {
	case TabLocks:
		if m.locksData.SelectedIndex < len(m.locksData.Locks)-1 {
			m.locksData.SelectedIndex++
		}
	case TabPeers:
		if m.peersData.SelectedIndex < len(m.peersData.Peers)-1 {
			m.peersData.SelectedIndex++
		}
	}
}

func (m *Model) executeSelectedAction() tea.Cmd {
	switch m.activeTab {
	case TabLocks:
		if len(m.locksData.Locks) > 0 {
			lock := m.locksData.Locks[m.locksData.SelectedIndex]
			m.SetResult("Lock: "+lock.ID+" ("+lock.Holder+")", nil)
		}
	case TabPeers:
		if len(m.peersData.Peers) > 0 {
			peer := m.peersData.Peers[m.peersData.SelectedIndex]
			m.SetResult("Peer: "+peer.Name+" ("+peer.ID+")", nil)
		}
	}
	return nil
}

// 명령 실행

func (m *Model) executeCommand(input string) tea.Cmd {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil
	}

	cmd := parts[0]
	args := parts[1:]

	return func() tea.Msg {
		var result string
		var err error

		switch cmd {
		case "quit", "q":
			return tea.Quit

		case "init":
			if len(args) >= 2 && args[0] == "-p" {
				err = m.executeInit(args[1])
				result = "클러스터 초기화 완료"
			} else {
				err = nil
				result = "사용법: init -p <project-name>"
			}

		case "join":
			if len(args) >= 1 {
				err = m.executeJoin(args[0])
				result = "클러스터 참여 완료"
			} else {
				result = "사용법: join <token>"
			}

		case "leave":
			err = m.executeLeave()
			result = "클러스터 탈퇴 완료"

		case "status":
			result = "상태: 연결됨"

		case "lock":
			if len(args) >= 1 {
				switch args[0] {
				case "list":
					result = "락 목록 표시"
				case "release":
					if len(args) >= 2 {
						err = m.executeReleaseLock(args[1])
						result = "락 해제 완료"
					} else {
						result = "사용법: lock release <lock-id>"
					}
				}
			} else {
				result = "사용법: lock [list|release]"
			}

		case "agents":
			result = "에이전트 목록 표시"

		case "peers":
			result = "피어 목록 표시"

		case "tokens":
			result = "토큰 사용량 표시"

		case "config":
			result = "설정 표시"

		case "help":
			result = "도움말: q(종료), i(init), J(join), L(leave), s(status), :(명령)"

		default:
			result = "알 수 없는 명령: " + cmd
		}

		return CommandResultMsg{Result: result, Err: err}
	}
}

// 액션 실행 함수들

func (m *Model) executeInit(projectName string) error {
	// TODO: 실제 init 로직 연동
	m.projectName = projectName
	m.SetResult("프로젝트 '"+projectName+"' 초기화 완료", nil)
	return nil
}

func (m *Model) executeJoin(token string) error {
	// TODO: 실제 join 로직 연동
	m.SetResult("클러스터 참여 완료 (토큰: "+token[:min(10, len(token))]+"...)", nil)
	return nil
}

func (m *Model) executeLeave() error {
	// TODO: 실제 leave 로직 연동
	m.SetResult("클러스터 탈퇴 완료", nil)
	return nil
}

func (m *Model) executeReleaseLock(lockID string) error {
	// TODO: 실제 lock release 로직 연동
	m.SetResult("락 '"+lockID+"' 해제 완료", nil)
	return nil
}

// min 헬퍼
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// updateViewSizes는 뷰 크기를 업데이트합니다.
func (m *Model) updateViewSizes() {
	contentWidth := m.width - 4
	contentHeight := m.height - 8 // 헤더, 탭, 푸터 제외

	m.clusterView = ViewSize{Width: contentWidth, Height: contentHeight}
	m.contextView = ViewSize{Width: contentWidth, Height: contentHeight}
	m.locksView = ViewSize{Width: contentWidth, Height: contentHeight}
	m.tokensView = ViewSize{Width: contentWidth, Height: contentHeight}
	m.peersView = ViewSize{Width: contentWidth, Height: contentHeight}
}

// fetchAllData는 모든 데이터를 가져옵니다.
func (m Model) fetchAllData() tea.Cmd {
	return tea.Batch(
		m.fetchMetrics(),
		m.fetchPeers(),
		m.fetchLocks(),
		m.fetchContext(),
		m.fetchTokens(),
	)
}

// fetchMetrics는 메트릭을 가져옵니다.
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

// fetchPeers는 peer 목록을 가져옵니다.
func (m Model) fetchPeers() tea.Cmd {
	return func() tea.Msg {
		// TODO: 실제 peer 목록 가져오기 (daemon 연동)
		return PeersMsg{
			Peers: []PeerInfo{},
		}
	}
}

// fetchLocks는 락 목록을 가져옵니다.
func (m Model) fetchLocks() tea.Cmd {
	return func() tea.Msg {
		// TODO: 실제 락 목록 가져오기 (daemon 연동)
		return LocksMsg{
			Locks: []LockInfo{},
		}
	}
}

// fetchContext는 컨텍스트 상태를 가져옵니다.
func (m Model) fetchContext() tea.Cmd {
	return func() tea.Msg {
		// TODO: 실제 컨텍스트 가져오기 (daemon 연동)
		return ContextMsg{
			TotalEmbeddings: 0,
			DatabaseSize:    0,
			SyncProgress:    map[string]float64{},
		}
	}
}

// fetchTokens는 토큰 사용량을 가져옵니다.
func (m Model) fetchTokens() tea.Cmd {
	return func() tea.Msg {
		// TODO: 실제 토큰 사용량 가져오기 (daemon 연동)
		return TokensMsg{
			TodayUsed:   0,
			DailyLimit:  200000, // 기본 한도
			Breakdown:   []TokenBreakdown{},
			HourlyData:  []float64{},
			CostToday:   0,
			CostWeek:    0,
			CostMonth:   0,
			TokensWeek:  0,
			TokensMonth: 0,
		}
	}
}
