package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	footerKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	footerDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	footerMetricStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))
)

// Footer는 푸터 컴포넌트입니다.
type Footer struct {
	width       int
	cpuUsage    float64
	memUsage    int64
	netUpload   int64
	netDownload int64
	tokensRate  int64
}

// NewFooter는 새 푸터를 생성합니다.
func NewFooter() *Footer {
	return &Footer{}
}

// SetWidth는 너비를 설정합니다.
func (f *Footer) SetWidth(width int) {
	f.width = width
}

// UpdateMetrics는 메트릭을 업데이트합니다.
func (f *Footer) UpdateMetrics(cpu float64, mem, netUp, netDown, tokens int64) {
	f.cpuUsage = cpu
	f.memUsage = mem
	f.netUpload = netUp
	f.netDownload = netDown
	f.tokensRate = tokens
}

// Height는 푸터 높이를 반환합니다.
func (f *Footer) Height() int {
	return 2
}

// View는 푸터를 렌더링합니다.
func (f *Footer) View() string {
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
				footerKeyStyle.Render("["+k.key+"]"),
				footerDescStyle.Render(k.desc)))
	}
	keyLine := strings.Join(keyHelps, "  ")

	// 메트릭
	metricsLine := footerMetricStyle.Render(
		fmt.Sprintf("CPU: %.1f%% | MEM: %s | NET: ↑%s/s ↓%s/s | Tokens: %s/hr",
			f.cpuUsage,
			formatBytes(f.memUsage),
			formatBytes(f.netUpload),
			formatBytes(f.netDownload),
			formatNumber(f.tokensRate)))

	return lipgloss.JoinVertical(lipgloss.Left, keyLine, metricsLine)
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
