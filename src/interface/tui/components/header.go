package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	headerTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("205"))

	headerInfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	headerStatusOnline = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")).
				Render("●")

	headerStatusOffline = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")).
				Render("○")
)

// Header는 헤더 컴포넌트입니다.
type Header struct {
	width       int
	projectName string
	nodeID      string
	status      string
	peerCount   int
	syncHealth  float64
	uptime      string
}

// NewHeader는 새 헤더를 생성합니다.
func NewHeader() *Header {
	return &Header{
		status: "connected",
	}
}

// SetWidth는 너비를 설정합니다.
func (h *Header) SetWidth(width int) {
	h.width = width
}

// SetProject는 프로젝트 정보를 설정합니다.
func (h *Header) SetProject(name, nodeID string) {
	h.projectName = name
	h.nodeID = nodeID
}

// UpdateStatus는 상태를 업데이트합니다.
func (h *Header) UpdateStatus(status string, peerCount int, syncHealth float64) {
	h.status = status
	h.peerCount = peerCount
	h.syncHealth = syncHealth
}

// UpdateUptime는 업타임을 업데이트합니다.
func (h *Header) UpdateUptime(uptime string) {
	h.uptime = uptime
}

// Height는 헤더 높이를 반환합니다.
func (h *Header) Height() int {
	return 3
}

// View는 헤더를 렌더링합니다.
func (h *Header) View() string {
	// 타이틀
	title := headerTitleStyle.Render("🔗 agent-collab")

	// 상태 아이콘
	statusIcon := headerStatusOnline
	statusText := "Connected"
	if h.status != "connected" {
		statusIcon = headerStatusOffline
		statusText = "Disconnected"
	}

	// 프로젝트 정보
	projectInfo := headerInfoStyle.Render(
		fmt.Sprintf("Project: %s | Node: %s", h.projectName, h.nodeID))

	// 상태 정보
	statusInfo := fmt.Sprintf("%s %s | Peers: %d | Sync: %.1f%% | Uptime: %s",
		statusIcon, statusText, h.peerCount, h.syncHealth, h.uptime)

	// 레이아웃
	line1 := lipgloss.JoinHorizontal(lipgloss.Left,
		title,
		strings.Repeat(" ", 3),
		projectInfo)

	line2 := headerInfoStyle.Render("Status: ") + statusInfo

	return lipgloss.JoinVertical(lipgloss.Left, line1, line2)
}
