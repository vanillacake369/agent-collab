package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"agent-collab/src/interface/tui/mode"
)

// View는 UI를 렌더링합니다.
func (m Model) View() string {
	if !m.ready || m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	var sections []string

	// 헤더
	sections = append(sections, m.renderHeader())

	// 탭
	sections = append(sections, m.renderTabs())

	// 컨텐츠 (모드에 따라 오버레이)
	sections = append(sections, m.renderContent())

	// 모드별 오버레이 (Help 제외 - Help는 전체 화면 오버레이)
	if m.mode != mode.Normal && m.mode != mode.Help {
		sections = append(sections, m.renderModeOverlay())
	}

	// 결과 메시지
	if m.showResult {
		sections = append(sections, m.renderResultBar())
	}

	// 푸터
	sections = append(sections, m.renderFooter())

	base := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Help 모드일 때 전체 화면 오버레이로 표시
	if m.mode == mode.Help {
		return m.renderHelpOverlayFullscreen(base)
	}

	// 전체 화면 크기로 출력하여 리사이즈 시 잔상 방지
	// lipgloss.Place를 사용하여 콘텐츠를 화면 크기에 맞게 배치
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Left,
		lipgloss.Top,
		base,
	)
}

// renderModeOverlay는 모드별 오버레이를 렌더링합니다.
func (m Model) renderModeOverlay() string {
	switch m.mode {
	case mode.Command:
		return m.renderCommandPalette()
	case mode.Input:
		return m.renderInputPrompt()
	case mode.Confirm:
		return m.renderConfirmDialog()
	case mode.Help:
		return m.renderHelpOverlay()
	default:
		return ""
	}
}

// renderCommandPalette는 명령 팔레트를 렌더링합니다.
func (m Model) renderCommandPalette() string {
	width := m.width - 10
	if width < 50 {
		width = 50
	}

	// 입력 필드
	inputStyle := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)

	input := inputStyle.Render(":") + m.commandInput.View()

	// 힌트 목록 (퍼지 매칭 결과)
	var hints []string
	maxHints := 8

	for i, filtered := range m.filteredHints {
		if i >= maxHints {
			break
		}

		// 명령어 하이라이팅
		cmdStyled := m.renderCommandWithHighlight(filtered.Hint.Command, filtered.MatchedIndexes)

		// 선택 표시
		prefix := "  "
		lineStyle := lipgloss.NewStyle()
		if i == m.commandSelectedIdx {
			prefix = "▸ "
			lineStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("237")).
				Foreground(lipgloss.Color("255"))
		}

		// 설명과 인자
		desc := MutedStyle.Render(filtered.Hint.Description)
		args := ""
		if filtered.Hint.Args != "" {
			args = MutedStyle.Render(" " + filtered.Hint.Args)
		}

		line := fmt.Sprintf("%s%-15s %s%s", prefix, cmdStyled, desc, args)
		hints = append(hints, lineStyle.Render(line))
	}

	// 힌트가 없으면 안내 메시지
	if len(hints) == 0 && m.commandInput.Value() != "" {
		hints = append(hints, MutedStyle.Render("  일치하는 명령이 없습니다"))
	}

	// 하단 도움말
	helpText := MutedStyle.Render("Tab: 자동완성  ↑↓: 선택  Enter: 실행  Esc: 취소")

	content := lipgloss.JoinVertical(lipgloss.Left,
		input,
		strings.Repeat("─", width-4),
		strings.Join(hints, "\n"),
		"",
		helpText,
	)

	style := lipgloss.NewStyle().
		Width(width).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary)

	return style.Render(content)
}

// renderCommandWithHighlight는 매칭된 문자를 하이라이팅하여 렌더링합니다.
func (m Model) renderCommandWithHighlight(cmd string, matchedIndexes []int) string {
	if len(matchedIndexes) == 0 {
		return cmd
	}

	// 매칭된 인덱스를 set으로 변환
	matchSet := make(map[int]bool)
	for _, idx := range matchedIndexes {
		matchSet[idx] = true
	}

	highlightStyle := lipgloss.NewStyle().
		Foreground(ColorSuccess).
		Bold(true)

	var result strings.Builder
	for i, ch := range cmd {
		if matchSet[i] {
			result.WriteString(highlightStyle.Render(string(ch)))
		} else {
			result.WriteString(string(ch))
		}
	}

	return result.String()
}

// renderInputPrompt는 입력 프롬프트를 렌더링합니다.
func (m Model) renderInputPrompt() string {
	width := m.width - 20
	if width < 40 {
		width = 40
	}

	promptStyle := lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true)

	var lines []string
	lines = append(lines, promptStyle.Render(m.inputPrompt+":"))
	lines = append(lines, "")
	lines = append(lines, m.commandInput.View())

	if m.inputError != "" {
		lines = append(lines, "")
		lines = append(lines, ErrorStyle.Render("⚠ "+m.inputError))
	}

	lines = append(lines, "")
	lines = append(lines, MutedStyle.Render("[Enter] 확인  [Esc] 취소"))

	content := strings.Join(lines, "\n")

	style := lipgloss.NewStyle().
		Width(width).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorSecondary)

	return style.Render(content)
}

// renderConfirmDialog는 확인 대화상자를 렌더링합니다.
func (m Model) renderConfirmDialog() string {
	width := m.width - 30
	if width < 40 {
		width = 40
	}

	promptStyle := lipgloss.NewStyle().
		Foreground(ColorWarning).
		Bold(true)

	var lines []string
	lines = append(lines, promptStyle.Render("⚠ 확인"))
	lines = append(lines, "")
	lines = append(lines, m.confirmPrompt)
	lines = append(lines, "")

	yesBtn := lipgloss.NewStyle().
		Foreground(ColorSuccess).
		Bold(true).
		Render("[Y] Yes")

	noBtn := lipgloss.NewStyle().
		Foreground(ColorError).
		Bold(true).
		Render("[N] No")

	lines = append(lines, yesBtn+"  "+noBtn+"  "+MutedStyle.Render("[Esc] 취소"))

	content := strings.Join(lines, "\n")

	style := lipgloss.NewStyle().
		Width(width).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorWarning)

	return style.Render(content)
}

// renderHelpOverlayFullscreen은 전체 화면에 도움말 오버레이를 렌더링합니다.
func (m Model) renderHelpOverlayFullscreen(base string) string {
	// 터미널 크기가 너무 작으면 간단한 메시지 표시
	if m.width < 40 || m.height < 15 {
		return lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			"[?] Help (터미널이 너무 작습니다)\n아무 키나 누르세요",
		)
	}

	// 오버레이 박스 크기 계산 - 터미널 크기에 맞게 동적 조정
	overlayWidth := m.width * 70 / 100 // 화면의 70%
	// 터미널 크기를 초과하지 않도록 제한
	maxWidth := m.width - 4 // 좌우 여백
	if overlayWidth > maxWidth {
		overlayWidth = maxWidth
	}
	if overlayWidth < 30 {
		overlayWidth = 30
	}

	overlayHeight := m.height * 70 / 100 // 화면의 70%
	// 터미널 크기를 초과하지 않도록 제한
	maxHeight := m.height - 2 // 상하 여백
	if overlayHeight > maxHeight {
		overlayHeight = maxHeight
	}
	if overlayHeight < 10 {
		overlayHeight = 10
	}

	// 스타일 정의
	titleStyle := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)

	sectionStyle := lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true)

	keyStyle := lipgloss.NewStyle().
		Foreground(ColorSuccess).
		Bold(true).
		Width(8)

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	// 컴팩트 모드 여부 (높이가 작으면 축약)
	compact := overlayHeight < 20

	// 도움말 내용 생성
	var lines []string
	lines = append(lines, titleStyle.Render("📖 도움말"))
	lines = append(lines, "")

	if compact {
		// 컴팩트 모드: 핵심만 표시
		lines = append(lines, fmt.Sprintf("%s %s  %s %s  %s %s  %s %s",
			keyStyle.Render("q"), descStyle.Render("종료"),
			keyStyle.Render(":"), descStyle.Render("명령"),
			keyStyle.Render("r"), descStyle.Render("새로고침"),
			keyStyle.Render("?"), descStyle.Render("도움말")))
		lines = append(lines, fmt.Sprintf("%s %s  %s %s",
			keyStyle.Render("1-5"), descStyle.Render("탭"),
			keyStyle.Render("Tab"), descStyle.Render("탭이동")))
		lines = append(lines, fmt.Sprintf("%s %s  %s %s  %s %s",
			keyStyle.Render("i"), descStyle.Render("Init"),
			keyStyle.Render("j"), descStyle.Render("Join"),
			keyStyle.Render("l"), descStyle.Render("Leave")))
		lines = append(lines, fmt.Sprintf("%s %s  %s %s  %s %s",
			keyStyle.Render("↑↓"), descStyle.Render("이동"),
			keyStyle.Render("Enter"), descStyle.Render("선택"),
			keyStyle.Render("d"), descStyle.Render("삭제")))
	} else {
		// 전체 모드
		// 일반
		lines = append(lines, sectionStyle.Render("일반"))
		lines = append(lines, fmt.Sprintf("  %s %s", keyStyle.Render("q"), descStyle.Render("종료")))
		lines = append(lines, fmt.Sprintf("  %s %s", keyStyle.Render(":"), descStyle.Render("명령 팔레트 열기")))
		lines = append(lines, fmt.Sprintf("  %s %s", keyStyle.Render("r"), descStyle.Render("데이터 새로고침")))
		lines = append(lines, fmt.Sprintf("  %s %s", keyStyle.Render("?"), descStyle.Render("도움말 표시")))
		lines = append(lines, "")

		// 탭 전환
		lines = append(lines, sectionStyle.Render("탭 전환"))
		lines = append(lines, fmt.Sprintf("  %s %s", keyStyle.Render("1-5"), descStyle.Render("탭 선택 (Cluster/Context/Locks/Tokens/Peers)")))
		lines = append(lines, fmt.Sprintf("  %s %s", keyStyle.Render("Tab"), descStyle.Render("다음 탭")))
		lines = append(lines, fmt.Sprintf("  %s %s", keyStyle.Render("S-Tab"), descStyle.Render("이전 탭")))
		lines = append(lines, "")

		// 명령어 단축키
		lines = append(lines, sectionStyle.Render("명령어 단축키"))
		lines = append(lines, fmt.Sprintf("  %s %s", keyStyle.Render("i"), descStyle.Render("Init - 새 클러스터 초기화")))
		lines = append(lines, fmt.Sprintf("  %s %s", keyStyle.Render("j"), descStyle.Render("Join - 클러스터 참여")))
		lines = append(lines, fmt.Sprintf("  %s %s", keyStyle.Render("l"), descStyle.Render("Leave - 클러스터 탈퇴")))
		lines = append(lines, "")

		// 네비게이션
		lines = append(lines, sectionStyle.Render("네비게이션"))
		lines = append(lines, fmt.Sprintf("  %s %s", keyStyle.Render("↑/k"), descStyle.Render("위로 이동")))
		lines = append(lines, fmt.Sprintf("  %s %s", keyStyle.Render("↓/j"), descStyle.Render("아래로 이동")))
		lines = append(lines, fmt.Sprintf("  %s %s", keyStyle.Render("Enter"), descStyle.Render("선택/실행")))
		lines = append(lines, fmt.Sprintf("  %s %s", keyStyle.Render("d"), descStyle.Render("삭제 (Locks 탭에서 락 해제)")))
	}

	lines = append(lines, "")
	lines = append(lines, MutedStyle.Render("아무 키나 누르면 닫힙니다"))

	content := strings.Join(lines, "\n")

	// 오버레이 박스 스타일
	boxStyle := lipgloss.NewStyle().
		Width(overlayWidth-4). // 테두리와 패딩 고려
		Height(overlayHeight-4).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Background(lipgloss.Color("235")) // 어두운 배경

	overlay := boxStyle.Render(content)

	// 화면 중앙에 오버레이 배치
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		overlay,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("237")),
	)
}

// renderHelpOverlay는 도움말 오버레이를 렌더링합니다 (미사용, 호환성 유지).
func (m Model) renderHelpOverlay() string {
	return ""
}

// renderResultBar는 결과 메시지 바를 렌더링합니다.
func (m Model) renderResultBar() string {
	var style lipgloss.Style
	var icon string

	if m.lastError != nil {
		style = lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true)
		icon = "✗ "
	} else {
		style = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)
		icon = "✓ "
	}

	msg := m.lastResult
	if m.lastError != nil {
		msg = m.lastError.Error()
	}

	return style.Render(icon + msg)
}

// renderHeader는 헤더를 렌더링합니다.
func (m Model) renderHeader() string {
	// 첫 번째 줄: 타이틀 + 모드 표시
	title := HeaderTitleStyle.Render("🔗 agent-collab")

	modeStr := ""
	if m.mode != mode.Normal {
		modeStyle := lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true)
		modeStr = modeStyle.Render(" [" + m.mode.String() + "]")
	}

	// 상태
	status := StatusIcon("connected")
	statusText := fmt.Sprintf("%s Connected", status)

	// 두 번째 줄: 프로젝트 정보
	projectInfo := fmt.Sprintf("Project: %s | Node: %s", m.projectName, m.nodeID)
	peerInfo := fmt.Sprintf("Peers: %d | Sync: %.1f%%", m.peerCount, m.syncHealth)

	// 업타임
	uptimeStr := formatDurationReal(m.uptime)

	line1 := lipgloss.JoinHorizontal(lipgloss.Left,
		title,
		modeStr,
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
	contentHeight := m.height - 10 // 헤더, 탭, 푸터, 결과바 제외
	if contentHeight < 5 {
		contentHeight = 5 // 최소 높이
	}
	if contentHeight > m.height-4 {
		contentHeight = m.height - 4 // 터미널보다 크지 않도록
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
	// 모드별 키 바인딩
	var keys []struct {
		key  string
		desc string
	}

	switch m.mode {
	case mode.Command:
		keys = []struct {
			key  string
			desc string
		}{
			{"Enter", "Execute"},
			{"Tab", "Complete"},
			{"Esc", "Cancel"},
			{"↑↓", "History"},
		}
	case mode.Input:
		keys = []struct {
			key  string
			desc string
		}{
			{"Enter", "Confirm"},
			{"Esc", "Cancel"},
		}
	case mode.Confirm:
		keys = []struct {
			key  string
			desc string
		}{
			{"y", "Yes"},
			{"n", "No"},
			{"Esc", "Cancel"},
		}
	default:
		keys = []struct {
			key  string
			desc string
		}{
			{"q", "Quit"},
			{":", "Command"},
			{"i", "Init"},
			{"j", "Join"},
			{"l", "Leave"},
			{"r", "Refresh"},
			{"?", "Help"},
		}
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

// 탭별 뷰 렌더링
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
	lines = append(lines, fmt.Sprintf("  Active Locks     : %d", len(m.locksData.Locks)))
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
	lines = append(lines, fmt.Sprintf("├─ Total Embeddings : %d", m.contextData.TotalEmbeddings))
	lines = append(lines, fmt.Sprintf("├─ Database Size    : %s", formatBytes(m.contextData.DatabaseSize)))
	lines = append(lines, "└─ Last Updated     : 2 seconds ago")
	lines = append(lines, "")

	lines = append(lines, BoxTitleStyle.Render("Sync Progress"))
	for name, pct := range m.contextData.SyncProgress {
		status := "synced"
		if pct < 100 {
			status = "syncing..."
		}
		lines = append(lines, fmt.Sprintf("  %-10s %s %3.0f%% (%s)", name, renderGauge(pct, 20), pct, status))
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderLocksView() string {
	var lines []string

	lines = append(lines, BoldStyle.Render("Semantic Locks"))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Active Locks: %d  (↑↓ 선택, d 해제, Enter 상세)", len(m.locksData.Locks)))
	lines = append(lines, "")

	// 테이블 헤더
	lines = append(lines, TableHeaderStyle.Render(
		fmt.Sprintf("  %-10s %-30s %-15s %s", "HOLDER", "TARGET", "INTENTION", "TTL")))
	lines = append(lines, strings.Repeat("─", 70))

	// 락 목록
	for i, l := range m.locksData.Locks {
		prefix := "  "
		style := lipgloss.NewStyle()

		if i == m.locksData.SelectedIndex {
			prefix = "▸ "
			style = TableSelectedStyle
		}

		line := fmt.Sprintf("%s%s %-10s %-30s %-15s %ds",
			prefix, StatusIcon("active"), l.Holder, l.Target, l.Intention, l.TTL)
		lines = append(lines, style.Render(line))
	}

	if len(m.locksData.Locks) == 0 {
		lines = append(lines, MutedStyle.Render("  활성 락이 없습니다."))
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderTokensView() string {
	var lines []string

	lines = append(lines, BoldStyle.Render("Token Usage"))
	lines = append(lines, "")

	// 오늘 사용량
	usedPct := float64(m.tokensData.TodayUsed) / float64(m.tokensData.DailyLimit) * 100
	lines = append(lines, "Today's Usage")
	lines = append(lines, fmt.Sprintf("%s %.1f%% (%s / %s)",
		renderColorGauge(usedPct, 30), usedPct,
		formatNumber(m.tokensData.TodayUsed),
		formatNumber(m.tokensData.DailyLimit)))
	lines = append(lines, "")

	// Breakdown
	lines = append(lines, BoxTitleStyle.Render("Usage Breakdown"))
	lines = append(lines, "")
	for _, b := range m.tokensData.Breakdown {
		lines = append(lines, fmt.Sprintf("  %-25s %s  %s (%2.0f%%)  $%.3f",
			b.Category, renderGauge(b.Percent, 15),
			formatNumber(b.Tokens), b.Percent, b.Cost))
	}
	lines = append(lines, "")

	// 요약
	lines = append(lines, BoxTitleStyle.Render("Period Summary"))
	lines = append(lines, fmt.Sprintf("  Today      : %s tokens     Est. $%.2f",
		formatNumber(m.tokensData.TodayUsed), m.tokensData.CostToday))
	lines = append(lines, fmt.Sprintf("  This Week  : %s tokens     Est. $%.2f",
		formatNumber(m.tokensData.TokensWeek), m.tokensData.CostWeek))
	lines = append(lines, fmt.Sprintf("  This Month : %s tokens   Est. $%.2f",
		formatNumber(m.tokensData.TokensMonth), m.tokensData.CostMonth))

	return strings.Join(lines, "\n")
}

func (m Model) renderPeersView() string {
	var lines []string

	onlineCount := 0
	for _, p := range m.peersData.Peers {
		if p.Status == "online" {
			onlineCount++
		}
	}

	lines = append(lines, BoldStyle.Render("Connected Peers"))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Total: %d peers | Online: %d | Syncing: %d  (↑↓ 선택, Enter 상세)",
		len(m.peersData.Peers), onlineCount, len(m.peersData.Peers)-onlineCount))
	lines = append(lines, "")

	// 테이블 헤더
	lines = append(lines, TableHeaderStyle.Render(
		fmt.Sprintf("  %-8s %-10s %-15s %-10s %8s  %s",
			"STATUS", "NAME", "PEER ID", "TRANSPORT", "LATENCY", "SYNC")))
	lines = append(lines, strings.Repeat("─", 70))

	// Peer 목록
	for i, p := range m.peersData.Peers {
		prefix := "  "
		style := lipgloss.NewStyle()

		if i == m.peersData.SelectedIndex {
			prefix = "▸ "
			style = TableSelectedStyle
		}

		line := fmt.Sprintf("%s%s    %-10s %-15s %-10s %6dms  %.0f%%",
			prefix, StatusIcon(p.Status), p.Name, p.ID, p.Transport, p.Latency, p.SyncPct)
		lines = append(lines, style.Render(line))
	}

	if len(m.peersData.Peers) == 0 {
		lines = append(lines, MutedStyle.Render("  연결된 피어가 없습니다."))
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

func formatDurationReal(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
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
