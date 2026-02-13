package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"agent-collab/src/interface/daemon"
	"agent-collab/src/interface/tui/mode"
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
		// 크기 변경 시 화면 전체를 지우고 다시 그리기
		// zellij/tmux fullscreen 같은 급격한 크기 변화에서 잔상 방지
		return m, tea.ClearScreen

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
		case mode.Help:
			return m.updateHelpMode(msg)
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
		// 주기적으로 데이터 갱신 (metrics, peers, status)
		cmds = append(cmds, m.tick(), m.fetchMetrics(), m.fetchPeers(), m.fetchStatus())

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

	// 도움말
	case key.Matches(msg, m.keys.Help):
		m.EnterHelpMode()
		return m, nil

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
	switch msg.String() {
	case "esc":
		m.ExitToNormalMode()
		return m, nil

	case "enter":
		// 입력이 비어있고 선택된 힌트가 있으면 힌트 적용
		if m.commandInput.Value() == "" && len(m.filteredHints) > 0 {
			m.ApplySelectedHint()
			return m, nil
		}
		cmd := m.executeCommand(m.commandInput.Value())
		m.ExitToNormalMode()
		return m, cmd

	case "tab":
		// Tab: 선택된 힌트로 자동완성
		m.ApplySelectedHint()
		return m, nil

	case "up", "ctrl+p":
		// 위쪽 화살표: 이전 힌트 선택
		m.SelectPrevHint()
		return m, nil

	case "down", "ctrl+n":
		// 아래쪽 화살표: 다음 힌트 선택
		m.SelectNextHint()
		return m, nil

	default:
		var cmd tea.Cmd
		m.commandInput, cmd = m.commandInput.Update(msg)
		// 입력이 변경되면 퍼지 매칭 업데이트
		m.UpdateFilteredHints()
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

// updateHelpMode는 Help 모드에서 키를 처리합니다.
func (m Model) updateHelpMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// 아무 키나 누르면 Help 모드 종료
	m.ExitToNormalMode()
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
			return tea.QuitMsg{}

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
			result = "도움말: q(종료), i(init), j(join), l(leave), 1-5(탭 전환), :(명령)"

		default:
			result = "알 수 없는 명령: " + cmd
		}

		return CommandResultMsg{Result: result, Err: err}
	}
}

// 액션 실행 함수들

func (m *Model) executeInit(projectName string) error {
	return m.executeInitWithClient(projectName)
}

func (m *Model) executeInitWithClient(projectName string) error {
	client := m.getClient()
	result, err := client.Init(projectName)
	if err != nil {
		return err
	}
	m.projectName = result.ProjectName
	m.nodeID = result.NodeID
	m.SetResult("프로젝트 '"+projectName+"' 초기화 완료", nil)
	return nil
}

func (m *Model) executeJoin(token string) error {
	return m.executeJoinWithClient(token)
}

func (m *Model) executeJoinWithClient(token string) error {
	client := m.getClient()
	result, err := client.Join(token)
	if err != nil {
		return err
	}
	m.projectName = result.ProjectName
	m.peerCount = result.ConnectedPeers
	m.SetResult("클러스터 참여 완료 (토큰: "+token[:min(10, len(token))]+"...)", nil)
	return nil
}

func (m *Model) executeLeave() error {
	return m.executeLeaveWithClient()
}

func (m *Model) executeLeaveWithClient() error {
	client := m.getClient()
	_, err := client.Leave()
	if err != nil {
		return err
	}
	m.SetResult("클러스터 탈퇴 완료", nil)
	return nil
}

func (m *Model) executeReleaseLock(lockID string) error {
	return m.executeReleaseLockWithClient(lockID)
}

func (m *Model) executeReleaseLockWithClient(lockID string) error {
	client := m.getClient()
	err := client.ReleaseLock(lockID)
	if err != nil {
		return err
	}
	m.SetResult("락 '"+lockID+"' 해제 완료", nil)
	return nil
}

// fetchTokenUsageWithClient fetches token usage from daemon.
func (m *Model) fetchTokenUsageWithClient() (*TokensMsg, error) {
	client := m.getClient()
	usage, err := client.TokenUsage()
	if err != nil {
		return nil, err
	}
	return &TokensMsg{
		TodayUsed:   int64(usage.TokensToday),
		DailyLimit:  int64(usage.DailyLimit),
		CostToday:   usage.CostToday,
		CostWeek:    usage.CostWeek,
		CostMonth:   usage.CostMonth,
		TokensWeek:  int64(usage.TokensWeek),
		TokensMonth: int64(usage.TokensMonth),
	}, nil
}

// fetchContextStatsWithClient fetches context stats from daemon.
func (m *Model) fetchContextStatsWithClient() (*daemon.ContextStatsResponse, error) {
	client := m.getClient()
	return client.ContextStats()
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
		client := m.getClient()
		if !client.IsRunning() {
			return MetricsMsg{}
		}

		metrics, err := client.Metrics()
		if err != nil {
			return MetricsMsg{}
		}

		// Extract metrics from map
		msg := MetricsMsg{}
		if v, ok := metrics["cpu_usage"].(float64); ok {
			msg.CPUUsage = v
		}
		if v, ok := metrics["mem_usage"].(float64); ok {
			msg.MemUsage = int64(v)
		}
		if v, ok := metrics["bytes_sent"].(float64); ok {
			msg.NetUpload = int64(v)
		}
		if v, ok := metrics["bytes_received"].(float64); ok {
			msg.NetDownload = int64(v)
		}
		if v, ok := metrics["tokens_per_hour"].(float64); ok {
			msg.TokensRate = int64(v)
		}

		return msg
	}
}

// fetchStatus는 데몬 상태를 가져옵니다.
func (m Model) fetchStatus() tea.Cmd {
	return func() tea.Msg {
		client := m.getClient()
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
			SyncHealth:  100,
		}
	}
}

// fetchPeers는 peer 목록을 가져옵니다.
func (m Model) fetchPeers() tea.Cmd {
	return func() tea.Msg {
		client := m.getClient()
		if !client.IsRunning() {
			return PeersMsg{Peers: []PeerInfo{}}
		}

		resp, err := client.ListPeers()
		if err != nil {
			return PeersMsg{Peers: []PeerInfo{}}
		}

		peers := make([]PeerInfo, len(resp.Peers))
		for i, p := range resp.Peers {
			addr := ""
			if len(p.Addresses) > 0 {
				addr = p.Addresses[0]
			}
			name := p.ID
			if len(p.ID) > 12 {
				name = p.ID[:12] + "..."
			}
			peers[i] = PeerInfo{
				ID:        p.ID,
				Name:      name,
				Status:    "connected",
				Latency:   int(p.Latency),
				Transport: addr,
			}
		}

		return PeersMsg{Peers: peers}
	}
}

// fetchLocks는 락 목록을 가져옵니다.
func (m Model) fetchLocks() tea.Cmd {
	return func() tea.Msg {
		client := m.getClient()
		if !client.IsRunning() {
			return LocksMsg{Locks: []LockInfo{}}
		}

		resp, err := client.ListLocks()
		if err != nil {
			return LocksMsg{Locks: []LockInfo{}}
		}

		locks := make([]LockInfo, len(resp.Locks))
		for i, l := range resp.Locks {
			target := ""
			if l.Target != nil {
				target = l.Target.FilePath
			}
			locks[i] = LockInfo{
				ID:        l.ID,
				Holder:    l.HolderName,
				Target:    target,
				Intention: l.Intention,
				TTL:       int(l.ExpiresAt.Sub(l.AcquiredAt).Seconds()),
			}
		}

		return LocksMsg{Locks: locks}
	}
}

// fetchContext는 컨텍스트 상태를 가져옵니다.
func (m Model) fetchContext() tea.Cmd {
	return func() tea.Msg {
		client := m.getClient()
		if !client.IsRunning() {
			return ContextMsg{SyncProgress: map[string]float64{}}
		}

		stats, err := client.ContextStats()
		if err != nil {
			return ContextMsg{SyncProgress: map[string]float64{}}
		}

		return ContextMsg{
			TotalEmbeddings: int(stats.TotalEmbeddings),
			DatabaseSize:    0, // Not provided by API yet
			SyncProgress:    map[string]float64{},
		}
	}
}

// fetchTokens는 토큰 사용량을 가져옵니다.
func (m Model) fetchTokens() tea.Cmd {
	return func() tea.Msg {
		client := m.getClient()
		if !client.IsRunning() {
			return TokensMsg{
				DailyLimit: 200000,
				Breakdown:  []TokenBreakdown{},
				HourlyData: []float64{},
			}
		}

		usage, err := client.TokenUsage()
		if err != nil {
			return TokensMsg{
				DailyLimit: 200000,
				Breakdown:  []TokenBreakdown{},
				HourlyData: []float64{},
			}
		}

		return TokensMsg{
			TodayUsed:   usage.TokensToday,
			DailyLimit:  usage.DailyLimit,
			Breakdown:   []TokenBreakdown{}, // Not provided by API yet
			HourlyData:  []float64{},        // Not provided by API yet
			CostToday:   usage.CostToday,
			CostWeek:    usage.CostWeek,
			CostMonth:   usage.CostMonth,
			TokensWeek:  usage.TokensWeek,
			TokensMonth: usage.TokensMonth,
		}
	}
}
