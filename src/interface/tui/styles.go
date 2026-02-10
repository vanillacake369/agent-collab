package tui

import "github.com/charmbracelet/lipgloss"

// 색상 팔레트
var (
	ColorPrimary   = lipgloss.Color("205") // 핑크
	ColorSecondary = lipgloss.Color("62")  // 청록
	ColorSuccess   = lipgloss.Color("82")  // 초록
	ColorWarning   = lipgloss.Color("214") // 주황
	ColorError     = lipgloss.Color("196") // 빨강
	ColorMuted     = lipgloss.Color("240") // 회색
	ColorWhite     = lipgloss.Color("255") // 흰색
	ColorDark      = lipgloss.Color("236") // 어두운 회색
)

// 기본 스타일
var (
	// 헤더 스타일
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Padding(0, 1)

	HeaderTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorWhite)

	HeaderInfoStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// 탭 스타일
	TabStyle = lipgloss.NewStyle().
			Padding(0, 2)

	ActiveTabStyle = TabStyle.Copy().
			Bold(true).
			Foreground(ColorPrimary).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(ColorPrimary)

	InactiveTabStyle = TabStyle.Copy().
				Foreground(ColorMuted)

	// 박스 스타일
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorSecondary).
			Padding(1)

	BoxTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorSecondary)

	// 테이블 스타일
	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorSecondary).
				Padding(0, 1)

	TableRowStyle = lipgloss.NewStyle().
			Padding(0, 1)

	TableSelectedStyle = TableRowStyle.Copy().
				Background(ColorDark)

	// 푸터 스타일
	FooterStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Padding(0, 1)

	FooterKeyStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	FooterDescStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// 상태 스타일
	StatusOnlineStyle = lipgloss.NewStyle().
				Foreground(ColorSuccess)

	StatusOfflineStyle = lipgloss.NewStyle().
				Foreground(ColorError)

	StatusSyncingStyle = lipgloss.NewStyle().
				Foreground(ColorWarning)

	// 게이지 스타일
	GaugeFilledStyle = lipgloss.NewStyle().
				Foreground(ColorSuccess)

	GaugeEmptyStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// 일반 텍스트
	BoldStyle = lipgloss.NewStyle().Bold(true)

	MutedStyle = lipgloss.NewStyle().Foreground(ColorMuted)

	ErrorStyle = lipgloss.NewStyle().Foreground(ColorError)

	SuccessStyle = lipgloss.NewStyle().Foreground(ColorSuccess)

	WarningStyle = lipgloss.NewStyle().Foreground(ColorWarning)
)

// StatusIcon은 상태 아이콘을 반환합니다.
func StatusIcon(status string) string {
	switch status {
	case "online", "connected", "active":
		return StatusOnlineStyle.Render("●")
	case "offline", "disconnected":
		return StatusOfflineStyle.Render("○")
	case "syncing", "connecting":
		return StatusSyncingStyle.Render("◐")
	default:
		return MutedStyle.Render("○")
	}
}
