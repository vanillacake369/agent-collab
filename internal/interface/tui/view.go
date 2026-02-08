package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View는 UI를 렌더링합니다.
func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	var sections []string

	// 헤더
	sections = append(sections, m.renderHeader())

	// 탭
	sections = append(sections, m.renderTabs())

	// 컨텐츠
	sections = append(sections, m.renderContent())

	// 푸터
	sections = append(sections, m.renderFooter())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderHeader는 헤더를 렌더링합니다.
func (m Model) renderHeader() string {
	// 첫 번째 줄: 타이틀
	title := HeaderTitleStyle.Render("🔗 agent-collab")

	// 상태
	status := StatusIcon("connected")
	statusText := fmt.Sprintf("%s Connected", status)

	// 두 번째 줄: 프로젝트 정보
	projectInfo := fmt.Sprintf("Project: %s | Node: %s", m.projectName, m.nodeID)
	peerInfo := fmt.Sprintf("Peers: %d | Sync: %.1f%%", m.peerCount, m.syncHealth)

	// 업타임
	uptimeStr := formatDuration(m.uptime)

	line1 := lipgloss.JoinHorizontal(lipgloss.Left,
		title,
		strings.Repeat(" ", 3),
		HeaderInfoStyle.Render(projectInfo),
	)

	line2 := lipgloss.JoinHorizontal(lipgloss.Left,
		HeaderInfoStyle.Render("Status: "),
		statusText,
		strings.Repeat(" ", 3),
		HeaderInfoStyle.Render(peerInfo),
		strings.Repeat(" ", 3),
		HeaderInfoStyle.Render("Uptime: "+uptimeStr),
	)

	header := lipgloss.JoinVertical(lipgloss.Left, line1, line2)

	return lipgloss.NewStyle().
		Width(m.width).
		Padding(0, 1).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(ColorMuted).
		Render(header)
}

// renderTabs는 탭 바를 렌더링합니다.
func (m Model) renderTabs() string {
	var tabs []string

	for i, name := range TabNames {
		tabName := fmt.Sprintf("[%d] %s", i+1, name)

		var style lipgloss.Style
		if Tab(i) == m.activeTab {
			style = ActiveTabStyle
		} else {
			style = InactiveTabStyle
		}

		tabs = append(tabs, style.Render(tabName))
	}

	return lipgloss.JoinHorizontal(lipgloss.Left, tabs...)
}

// renderContent는 탭 컨텐츠를 렌더링합니다.
func (m Model) renderContent() string {
	contentHeight := m.height - 8 // 헤더, 탭, 푸터 제외
	if contentHeight < 0 {
		contentHeight = 10
	}

	style := lipgloss.NewStyle().
		Width(m.width - 2).
		Height(contentHeight).
		Padding(1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorSecondary)

	var content string
	switch m.activeTab {
	case TabCluster:
		content = m.renderClusterView()
	case TabContext:
		content = m.renderContextView()
	case TabLocks:
		content = m.renderLocksView()
	case TabTokens:
		content = m.renderTokensView()
	case TabPeers:
		content = m.renderPeersView()
	}

	return style.Render(content)
}

// renderFooter는 푸터를 렌더링합니다.
func (m Model) renderFooter() string {
	// 키 바인딩
	keys := []struct {
		key  string
		desc string
	}{
		{"q", "Quit"},
		{"r", "Refresh"},
		{"1-5", "Tab"},
		{"↑↓", "Navigate"},
		{"?", "Help"},
	}

	var keyHelps []string
	for _, k := range keys {
		keyHelps = append(keyHelps,
			fmt.Sprintf("%s %s",
				FooterKeyStyle.Render("["+k.key+"]"),
				FooterDescStyle.Render(k.desc)))
	}
	keyLine := strings.Join(keyHelps, "  ")

	// 메트릭
	metricsLine := fmt.Sprintf("CPU: %.1f%% | MEM: %s | NET: ↑%s/s ↓%s/s | Tokens: %s/hr",
		m.cpuUsage,
		formatBytes(m.memUsage),
		formatBytes(m.netUpload),
		formatBytes(m.netDownload),
		formatNumber(m.tokensRate))

	footer := lipgloss.JoinVertical(lipgloss.Left,
		keyLine,
		MutedStyle.Render(metricsLine))

	return lipgloss.NewStyle().
		Width(m.width).
		Padding(0, 1).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(ColorMuted).
		Render(footer)
}

// 탭별 뷰 렌더링 (임시 구현)
func (m Model) renderClusterView() string {
	var lines []string

	lines = append(lines, BoldStyle.Render("Cluster Overview"))
	lines = append(lines, "")

	// 헬스 게이지
	lines = append(lines, fmt.Sprintf("Health Score: %s %.1f%%  [Healthy]",
		renderGauge(m.syncHealth, 20), m.syncHealth))
	lines = append(lines, "")

	// Quick Stats
	lines = append(lines, BoxTitleStyle.Render("Quick Stats"))
	lines = append(lines, fmt.Sprintf("  Total Peers      : %d", m.peerCount))
	lines = append(lines, fmt.Sprintf("  Active Locks     : %d", 2))
	lines = append(lines, fmt.Sprintf("  Pending Syncs    : %d", 0))
	lines = append(lines, fmt.Sprintf("  Avg Latency      : %dms", 42))
	lines = append(lines, fmt.Sprintf("  Messages/sec     : %.1f", 12.4))

	return strings.Join(lines, "\n")
}

func (m Model) renderContextView() string {
	var lines []string

	lines = append(lines, BoldStyle.Render("Context Sync Status"))
	lines = append(lines, "")

	lines = append(lines, "Vector Database")
	lines = append(lines, "├─ Total Embeddings : 12,456")
	lines = append(lines, "├─ Database Size    : 234.5 MB")
	lines = append(lines, "└─ Last Updated     : 2 seconds ago")
	lines = append(lines, "")

	lines = append(lines, BoxTitleStyle.Render("Sync Progress"))
	lines = append(lines, fmt.Sprintf("  Alice   %s 100%% (synced)", renderGauge(100, 20)))
	lines = append(lines, fmt.Sprintf("  Bob     %s  82%% (syncing...)", renderGauge(82, 20)))
	lines = append(lines, fmt.Sprintf("  Charlie %s 100%% (synced)", renderGauge(100, 20)))

	return strings.Join(lines, "\n")
}

func (m Model) renderLocksView() string {
	var lines []string

	lines = append(lines, BoldStyle.Render("Semantic Locks"))
	lines = append(lines, "")
	lines = append(lines, "Active Locks: 2")
	lines = append(lines, "")

	// 테이블 헤더
	lines = append(lines, TableHeaderStyle.Render(
		fmt.Sprintf("%-10s %-30s %-15s %s", "HOLDER", "TARGET", "INTENTION", "TTL")))
	lines = append(lines, strings.Repeat("─", 70))

	// 락 목록
	locks := []struct {
		holder    string
		target    string
		intention string
		ttl       int
	}{
		{"Alice", "src/auth/login.go:45-67", "리팩토링", 25},
		{"Bob", "pkg/api/handler.go:120-145", "버그 수정", 18},
	}

	for _, l := range locks {
		lines = append(lines, fmt.Sprintf("%s %-10s %-30s %-15s %ds",
			StatusIcon("active"), l.holder, l.target, l.intention, l.ttl))
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderTokensView() string {
	var lines []string

	lines = append(lines, BoldStyle.Render("Token Usage"))
	lines = append(lines, "")

	// 오늘 사용량
	usedPct := 52.3
	lines = append(lines, "Today's Usage")
	lines = append(lines, fmt.Sprintf("%s %.1f%% (104.5K / 200K)",
		renderColorGauge(usedPct, 30), usedPct))
	lines = append(lines, "")

	// Breakdown
	lines = append(lines, BoxTitleStyle.Render("Usage Breakdown"))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  Embedding Generation      %s  78.2K (75%%)  $0.078", renderGauge(75, 15)))
	lines = append(lines, fmt.Sprintf("  Context Synchronization   %s  21.1K (20%%)  $0.021", renderGauge(20, 15)))
	lines = append(lines, fmt.Sprintf("  Lock Negotiation          %s   5.2K (5%%)   $0.005", renderGauge(5, 15)))
	lines = append(lines, "")

	// 요약
	lines = append(lines, BoxTitleStyle.Render("Period Summary"))
	lines = append(lines, "  Today      : 104,521 tokens     Est. $0.10")
	lines = append(lines, "  This Week  : 623,456 tokens     Est. $0.62")
	lines = append(lines, "  This Month : 2,345,678 tokens   Est. $2.35")

	return strings.Join(lines, "\n")
}

func (m Model) renderPeersView() string {
	var lines []string

	lines = append(lines, BoldStyle.Render("Connected Peers"))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Total: %d peers | Online: %d | Syncing: 1", m.peerCount, m.peerCount-1))
	lines = append(lines, "")

	// 테이블 헤더
	lines = append(lines, TableHeaderStyle.Render(
		fmt.Sprintf("%-8s %-10s %-15s %-10s %8s  %s",
			"STATUS", "NAME", "PEER ID", "TRANSPORT", "LATENCY", "SYNC")))
	lines = append(lines, strings.Repeat("─", 70))

	// Peer 목록
	peers := []struct {
		name      string
		id        string
		status    string
		transport string
		latency   int
		sync      float64
	}{
		{"Alice", "QmAbc...123", "online", "QUIC", 12, 100},
		{"Bob", "QmDef...456", "online", "WebRTC", 45, 100},
		{"Charlie", "QmGhi...789", "syncing", "TCP", 89, 82},
		{"Diana", "QmJkl...012", "online", "QUIC", 23, 100},
	}

	for _, p := range peers {
		lines = append(lines, fmt.Sprintf("%s    %-10s %-15s %-10s %6dms  %.0f%%",
			StatusIcon(p.status), p.name, p.id, p.transport, p.latency, p.sync))
	}

	return strings.Join(lines, "\n")
}

// 유틸리티 함수들
func renderGauge(percent float64, width int) string {
	filled := int(float64(width) * percent / 100)
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	filledPart := GaugeFilledStyle.Render(strings.Repeat("█", filled))
	emptyPart := GaugeEmptyStyle.Render(strings.Repeat("░", width-filled))

	return filledPart + emptyPart
}

func renderColorGauge(percent float64, width int) string {
	var color lipgloss.Color
	switch {
	case percent >= 90:
		color = ColorError
	case percent >= 70:
		color = ColorWarning
	case percent >= 50:
		color = lipgloss.Color("226") // 노랑
	default:
		color = ColorSuccess
	}

	filled := int(float64(width) * percent / 100)
	if filled > width {
		filled = width
	}

	filledPart := lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("█", filled))
	emptyPart := GaugeEmptyStyle.Render(strings.Repeat("░", width-filled))

	return filledPart + emptyPart
}

func formatDuration(d interface{}) string {
	// time.Duration 처리
	return "2h 34m" // TODO: 실제 포맷팅
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func formatNumber(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}
