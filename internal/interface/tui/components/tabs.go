package components

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	tabStyle = lipgloss.NewStyle().
			Padding(0, 2)

	activeTabStyle = tabStyle.Copy().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(lipgloss.Color("205"))

	inactiveTabStyle = tabStyle.Copy().
				Foreground(lipgloss.Color("240"))
)

// Tabs는 탭 바 컴포넌트입니다.
type Tabs struct {
	names     []string
	activeIdx int
	width     int
}

// NewTabs는 새 탭 바를 생성합니다.
func NewTabs(names []string) *Tabs {
	return &Tabs{
		names: names,
	}
}

// SetActive는 활성 탭을 설정합니다.
func (t *Tabs) SetActive(idx int) {
	if idx >= 0 && idx < len(t.names) {
		t.activeIdx = idx
	}
}

// SetWidth는 너비를 설정합니다.
func (t *Tabs) SetWidth(width int) {
	t.width = width
}

// Height는 탭 바 높이를 반환합니다.
func (t *Tabs) Height() int {
	return 1
}

// View는 탭 바를 렌더링합니다.
func (t *Tabs) View() string {
	var tabs []string

	for i, name := range t.names {
		tabName := fmt.Sprintf("[%d] %s", i+1, name)

		var style lipgloss.Style
		if i == t.activeIdx {
			style = activeTabStyle
		} else {
			style = inactiveTabStyle
		}

		tabs = append(tabs, style.Render(tabName))
	}

	return lipgloss.JoinHorizontal(lipgloss.Left, tabs...)
}
