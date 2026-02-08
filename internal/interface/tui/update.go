package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
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
			m.activeTab = Tab((int(m.activeTab) + 1) % 5)
		case key.Matches(msg, m.keys.PrevTab):
			m.activeTab = Tab((int(m.activeTab) + 4) % 5)

		case key.Matches(msg, m.keys.Refresh):
			cmds = append(cmds, m.fetchAllData())
		}

	case TickMsg:
		m.uptime = time.Since(m.startTime)
		cmds = append(cmds, m.tick(), m.fetchMetrics())

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
		// TODO: 실제 메트릭 가져오기
		return MetricsMsg{
			CPUUsage:    2.3,
			MemUsage:    45 * 1024 * 1024, // 45MB
			NetUpload:   12 * 1024,        // 12KB/s
			NetDownload: 8 * 1024,         // 8KB/s
			TokensRate:  1200,             // 1.2K/hr
		}
	}
}

// fetchPeers는 peer 목록을 가져옵니다.
func (m Model) fetchPeers() tea.Cmd {
	return func() tea.Msg {
		// TODO: 실제 peer 목록 가져오기
		return PeersMsg{
			Peers: []PeerInfo{
				{ID: "QmAbc...123", Name: "Alice", Status: "online", Latency: 12, Transport: "QUIC", SyncPct: 100},
				{ID: "QmDef...456", Name: "Bob", Status: "online", Latency: 45, Transport: "WebRTC", SyncPct: 100},
				{ID: "QmGhi...789", Name: "Charlie", Status: "syncing", Latency: 89, Transport: "TCP", SyncPct: 82},
				{ID: "QmJkl...012", Name: "Diana", Status: "online", Latency: 23, Transport: "QUIC", SyncPct: 100},
			},
		}
	}
}

// fetchLocks는 락 목록을 가져옵니다.
func (m Model) fetchLocks() tea.Cmd {
	return func() tea.Msg {
		// TODO: 실제 락 목록 가져오기
		return LocksMsg{
			Locks: []LockInfo{
				{ID: "lock-001", Holder: "Alice", Target: "src/auth/login.go:45-67", Intention: "리팩토링", TTL: 25},
				{ID: "lock-002", Holder: "Bob", Target: "pkg/api/handler.go:120-145", Intention: "버그 수정", TTL: 18},
			},
		}
	}
}

// fetchContext는 컨텍스트 상태를 가져옵니다.
func (m Model) fetchContext() tea.Cmd {
	return func() tea.Msg {
		// TODO: 실제 컨텍스트 가져오기
		return ContextMsg{
			TotalEmbeddings: 12456,
			DatabaseSize:    234 * 1024 * 1024, // 234MB
			SyncProgress: map[string]float64{
				"Alice":   100,
				"Bob":     82,
				"Charlie": 100,
			},
		}
	}
}

// fetchTokens는 토큰 사용량을 가져옵니다.
func (m Model) fetchTokens() tea.Cmd {
	return func() tea.Msg {
		// TODO: 실제 토큰 사용량 가져오기
		return TokensMsg{
			TodayUsed:  104521,
			DailyLimit: 200000,
			Breakdown: []TokenBreakdown{
				{Category: "Embedding Generation", Tokens: 78234, Percent: 75, Cost: 0.078},
				{Category: "Context Synchronization", Tokens: 21123, Percent: 20, Cost: 0.021},
				{Category: "Lock Negotiation", Tokens: 5164, Percent: 5, Cost: 0.005},
			},
			HourlyData:  []float64{5000, 8000, 12000, 15000, 10000, 8000, 6000, 4000},
			CostToday:   0.10,
			CostWeek:    0.62,
			CostMonth:   2.35,
			TokensWeek:  623456,
			TokensMonth: 2345678,
		}
	}
}
