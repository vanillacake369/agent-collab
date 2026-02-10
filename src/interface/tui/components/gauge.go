package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	gaugeFilledStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	gaugeEmptyStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// RenderGauge는 게이지를 렌더링합니다.
func RenderGauge(percent float64, width int) string {
	filled := int(float64(width) * percent / 100)
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	filledPart := gaugeFilledStyle.Render(strings.Repeat("█", filled))
	emptyPart := gaugeEmptyStyle.Render(strings.Repeat("░", width-filled))

	return filledPart + emptyPart
}

// RenderColorGauge는 색상이 변하는 게이지를 렌더링합니다.
func RenderColorGauge(percent float64, width int) string {
	var color lipgloss.Color
	switch {
	case percent >= 90:
		color = lipgloss.Color("196") // 빨강
	case percent >= 70:
		color = lipgloss.Color("214") // 주황
	case percent >= 50:
		color = lipgloss.Color("226") // 노랑
	default:
		color = lipgloss.Color("82") // 초록
	}

	filled := int(float64(width) * percent / 100)
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	filledPart := lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("█", filled))
	emptyPart := gaugeEmptyStyle.Render(strings.Repeat("░", width-filled))

	return filledPart + emptyPart
}

// RenderProgressBar는 진행률 바를 렌더링합니다.
func RenderProgressBar(percent float64, width int, label string) string {
	gauge := RenderGauge(percent, width)
	return gauge + " " + label
}
